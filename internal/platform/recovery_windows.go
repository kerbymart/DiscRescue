//go:build windows

package platform

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"discrescue/internal/mapfile"
)

type OSRecovery struct{}

type mountedRecoveryJob struct {
	cancel context.CancelFunc
	state  *recoveryMapState

	mu       sync.Mutex
	snapshot RecoverySnapshot
}

type recoveryMapState struct {
	mapPath       string
	journalFile   *os.File
	header        mapfile.Header
	checkpointLen int
	nextSequence  uint64
	extents       []mapfile.Extent
}

func (j *mountedRecoveryJob) Snapshot() RecoverySnapshot {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.snapshot
}

func (j *mountedRecoveryJob) Cancel() {
	j.cancel()
}

func (j *mountedRecoveryJob) setProgress(copied uint64) {
	j.mu.Lock()
	j.snapshot.CopiedBytes = copied
	j.mu.Unlock()
}

func (j *mountedRecoveryJob) addUnreadable(count uint64, lines ...string) {
	j.mu.Lock()
	j.snapshot.UnreadableSectors += count
	if len(lines) > 0 {
		j.snapshot.LastIssue = append([]string(nil), lines...)
	}
	j.mu.Unlock()
}

func (j *mountedRecoveryJob) finish(canceled bool, err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.snapshot.Done = true
	j.snapshot.Canceled = canceled
	if err != nil {
		j.snapshot.ErrText = err.Error()
	}
	if j.state != nil {
		_ = j.state.close(canceled, err == nil)
	}
}

func (OSRecovery) StartImageRecovery(input RecoveryInput) (RecoveryJob, error) {
	if input.DevicePath == "" {
		return nil, fmt.Errorf("start image recovery: device path is required")
	}
	if input.OutputPath == "" || input.OutputPath == "Not chosen yet" {
		return nil, fmt.Errorf("start image recovery: output path is not configured")
	}
	if input.LogicalSectorSize == 0 {
		return nil, fmt.Errorf("start image recovery: logical sector size is required")
	}
	if input.CapacitySectors == 0 {
		return nil, fmt.Errorf("start image recovery: capacity sectors must be greater than zero")
	}

	state, resumed, err := openRecoveryMapState(input)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	copiedBytes, unreadableSectors := summarizeExtents(state.extents, input.LogicalSectorSize)
	lastIssue := []string{}
	if resumed {
		lastIssue = []string{"Resuming from durable recovery state."}
	}
	job := &mountedRecoveryJob{
		cancel: cancel,
		state:  state,
		snapshot: RecoverySnapshot{
			StartedAt:         time.Now(),
			TotalBytes:        input.CapacitySectors * uint64(input.LogicalSectorSize),
			CopiedBytes:       copiedBytes,
			UnreadableSectors: unreadableSectors,
			MapPath:           state.mapPath,
			Resumed:           resumed,
			LastIssue:         lastIssue,
		},
	}

	go job.run(ctx, input)
	return job, nil
}

func (j *mountedRecoveryJob) run(ctx context.Context, input RecoveryInput) {
	rawPath := rawVolumePath(input.DevicePath)
	source, err := os.Open(rawPath)
	if err != nil {
		j.finish(false, fmt.Errorf("open source volume %s: %w", rawPath, err))
		return
	}
	defer source.Close()

	if dir := filepath.Dir(input.OutputPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			j.finish(false, fmt.Errorf("create output directory %s: %w", dir, err))
			return
		}
	}
	output, err := os.OpenFile(input.OutputPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		j.finish(false, fmt.Errorf("create output image %s: %w", input.OutputPath, err))
		return
	}
	defer output.Close()

	totalBytes := input.CapacitySectors * uint64(input.LogicalSectorSize)
	if !j.snapshot.Resumed {
		if err := output.Truncate(int64(totalBytes)); err != nil {
			j.finish(false, fmt.Errorf("prepare output image %s: %w", input.OutputPath, err))
			return
		}
	}
	if err := output.Truncate(int64(totalBytes)); err != nil {
		j.finish(false, fmt.Errorf("prepare output image %s: %w", input.OutputPath, err))
		return
	}

	chunkSize := int(input.LogicalSectorSize) * 64
	if chunkSize < int(input.LogicalSectorSize) {
		chunkSize = int(input.LogicalSectorSize)
	}
	buffer := make([]byte, chunkSize)
	copied := j.snapshot.CopiedBytes
	logicalSectorSize := uint64(input.LogicalSectorSize)

	for lba := uint64(0); lba < input.CapacitySectors; {
		select {
		case <-ctx.Done():
			j.setProgress(copied)
			j.finish(true, nil)
			return
		default:
		}

		if extent, _, ok := mapfile.LookupExtent(j.state.extents, lba); ok && claimsImageData(extent.State) {
			lba = extent.EndLBA()
			j.setProgress(copied)
			continue
		}

		sectorsToRead := uint64(64)
		if remaining := input.CapacitySectors - lba; remaining < sectorsToRead {
			sectorsToRead = remaining
		}
		readSize := int(sectorsToRead * logicalSectorSize)
		offset := int64(lba * logicalSectorSize)

		n, err := source.ReadAt(buffer[:readSize], offset)
		if err == nil || (err == io.EOF && n == readSize) {
			if err := writeFullAt(output, buffer[:readSize], offset); err != nil {
				j.setProgress(copied)
				j.finish(false, fmt.Errorf("write output image %s at byte %d: %w", input.OutputPath, offset, err))
				return
			}
			if err := j.state.appendExtent(mapfile.Extent{
				StartLBA:   lba,
				Sectors:    uint32(sectorsToRead),
				State:      mapfile.SectorStateReadUnverified,
				Confidence: mapfile.ConfidenceSingleRead,
			}); err != nil {
				j.setProgress(copied)
				j.finish(false, fmt.Errorf("persist recovery map %s: %w", j.state.mapPath, err))
				return
			}
			copied += uint64(readSize)
			j.setProgress(copied)
			lba += sectorsToRead
			continue
		}

		singleSector := int(input.LogicalSectorSize)
		sectorBuffer := make([]byte, singleSector)
		clusterStart := lba
		clusterEnd := lba + sectorsToRead
		for lba < clusterEnd {
			select {
			case <-ctx.Done():
				j.setProgress(copied)
				j.finish(true, nil)
				return
			default:
			}

			sectorOffset := int64(lba * logicalSectorSize)
			n, err := source.ReadAt(sectorBuffer, sectorOffset)
			if err == nil || (err == io.EOF && n == singleSector) {
				if err := writeFullAt(output, sectorBuffer[:singleSector], sectorOffset); err != nil {
					j.setProgress(copied)
					j.finish(false, fmt.Errorf("write output image %s at byte %d: %w", input.OutputPath, sectorOffset, err))
					return
				}
				if err := j.state.appendExtent(mapfile.Extent{
					StartLBA:   lba,
					Sectors:    1,
					State:      mapfile.SectorStateReadUnverified,
					Confidence: mapfile.ConfidenceSingleRead,
				}); err != nil {
					j.setProgress(copied)
					j.finish(false, fmt.Errorf("persist recovery map %s: %w", j.state.mapPath, err))
					return
				}
				copied += logicalSectorSize
				j.setProgress(copied)
				lba++
				continue
			}

			if err := j.state.appendExtent(mapfile.Extent{
				StartLBA:   lba,
				Sectors:    1,
				State:      mapfile.SectorStateMissing,
				Confidence: mapfile.ConfidenceNone,
			}); err != nil {
				j.setProgress(copied)
				j.finish(false, fmt.Errorf("persist recovery map %s: %w", j.state.mapPath, err))
				return
			}
			j.addUnreadable(1,
				fmt.Sprintf("Unreadable sector at LBA %d.", lba),
				fmt.Sprintf("Cluster fallback was required for LBA %d to %d.", clusterStart, clusterEnd-1),
			)
			lba++
		}
	}

	if err := output.Sync(); err != nil {
		j.finish(false, fmt.Errorf("sync output image %s: %w", input.OutputPath, err))
		return
	}
	j.finish(false, nil)
}

func writeFullAt(file *os.File, data []byte, offset int64) error {
	written := 0
	for written < len(data) {
		n, err := file.WriteAt(data[written:], offset+int64(written))
		written += n
		if err != nil {
			return err
		}
		if n == 0 {
			return errors.New("short write")
		}
	}
	return nil
}

func openRecoveryMapState(input RecoveryInput) (*recoveryMapState, bool, error) {
	mapPath := recoveryMapPath(input.OutputPath)
	outputInfo, outputErr := os.Stat(input.OutputPath)
	_, mapErr := os.Stat(mapPath)

	switch {
	case errors.Is(outputErr, os.ErrNotExist) && errors.Is(mapErr, os.ErrNotExist):
		return createRecoveryMapState(input, mapPath)
	case outputErr == nil && mapErr == nil:
		return loadRecoveryMapState(input, mapPath, outputInfo)
	case outputErr == nil && errors.Is(mapErr, os.ErrNotExist):
		return nil, false, fmt.Errorf("start image recovery: output image %s already exists without %s; choose another output path", input.OutputPath, mapPath)
	case errors.Is(outputErr, os.ErrNotExist) && mapErr == nil:
		return nil, false, fmt.Errorf("start image recovery: recovery map %s exists without image %s; choose another output path", mapPath, input.OutputPath)
	case outputErr != nil && !errors.Is(outputErr, os.ErrNotExist):
		return nil, false, fmt.Errorf("start image recovery: check output image %s: %w", input.OutputPath, outputErr)
	default:
		return nil, false, fmt.Errorf("start image recovery: check recovery map %s: %w", mapPath, mapErr)
	}
}

func createRecoveryMapState(input RecoveryInput, mapPath string) (*recoveryMapState, bool, error) {
	header := mapfile.Header{
		LogicalSectorSize:   input.LogicalSectorSize,
		ExpectedSectorCount: input.CapacitySectors,
		OutputFormat:        1,
		CreationUnixNano:    time.Now().UnixNano(),
	}
	headerBytes, err := mapfile.MarshalHeader(header)
	if err != nil {
		return nil, false, fmt.Errorf("create recovery map header: %w", err)
	}
	checkpointBytes, err := mapfile.MarshalCheckpoint(mapfile.Checkpoint{})
	if err != nil {
		return nil, false, fmt.Errorf("create recovery map checkpoint: %w", err)
	}
	if dir := filepath.Dir(mapPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, false, fmt.Errorf("create recovery map directory %s: %w", dir, err)
		}
	}
	file, err := os.OpenFile(mapPath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o644)
	if err != nil {
		return nil, false, fmt.Errorf("create recovery map %s: %w", mapPath, err)
	}
	if _, err := file.Write(headerBytes); err != nil {
		file.Close()
		return nil, false, fmt.Errorf("write recovery map header %s: %w", mapPath, err)
	}
	if _, err := file.Write(checkpointBytes); err != nil {
		file.Close()
		return nil, false, fmt.Errorf("write recovery map checkpoint %s: %w", mapPath, err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return nil, false, fmt.Errorf("sync recovery map %s: %w", mapPath, err)
	}
	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		file.Close()
		return nil, false, fmt.Errorf("seek recovery map journal %s: %w", mapPath, err)
	}
	return &recoveryMapState{
		mapPath:       mapPath,
		journalFile:   file,
		header:        header,
		checkpointLen: len(checkpointBytes),
		nextSequence:  1,
	}, false, nil
}

func loadRecoveryMapState(input RecoveryInput, mapPath string, outputInfo os.FileInfo) (*recoveryMapState, bool, error) {
	data, err := os.ReadFile(mapPath)
	if err != nil {
		return nil, false, fmt.Errorf("read recovery map %s: %w", mapPath, err)
	}
	header, checkpoint, journalOffset, err := readRecoveryMapBytes(data)
	if err != nil {
		return nil, false, fmt.Errorf("load recovery map %s: %w", mapPath, err)
	}
	if header.LogicalSectorSize != input.LogicalSectorSize || header.ExpectedSectorCount != input.CapacitySectors {
		return nil, false, fmt.Errorf("start image recovery: recovery map %s does not match the selected media", mapPath)
	}
	replayed, err := mapfile.ReplayJournal(checkpoint, data[journalOffset:])
	if err != nil {
		return nil, false, fmt.Errorf("replay recovery map %s: %w", mapPath, err)
	}
	if requiredImageBytes(replayed.Extents, input.LogicalSectorSize) > uint64(outputInfo.Size()) {
		return nil, false, fmt.Errorf("start image recovery: image %s is smaller than the durable recovery map", input.OutputPath)
	}
	file, err := os.OpenFile(mapPath, os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		return nil, false, fmt.Errorf("open recovery map %s for append: %w", mapPath, err)
	}
	return &recoveryMapState{
		mapPath:       mapPath,
		journalFile:   file,
		header:        header,
		checkpointLen: journalOffset - len(mustMarshalHeader(header)),
		nextSequence:  replayed.LastSequence + 1,
		extents:       append([]mapfile.Extent(nil), replayed.Extents...),
	}, true, nil
}

func readRecoveryMapBytes(data []byte) (mapfile.Header, mapfile.Checkpoint, int, error) {
	if len(data) < 8 {
		return mapfile.Header{}, mapfile.Checkpoint{}, 0, fmt.Errorf("recovery map is too short")
	}
	headerLength := int(binary.LittleEndian.Uint16(data[6:8]))
	if len(data) < headerLength+10 {
		return mapfile.Header{}, mapfile.Checkpoint{}, 0, fmt.Errorf("recovery map is missing checkpoint data")
	}
	header, err := mapfile.UnmarshalHeader(data[:headerLength])
	if err != nil {
		return mapfile.Header{}, mapfile.Checkpoint{}, 0, err
	}
	checkpointPayloadLength := int(binary.LittleEndian.Uint32(data[headerLength+6 : headerLength+10]))
	checkpointLength := 4 + 2 + 4 + checkpointPayloadLength + 4
	if len(data) < headerLength+checkpointLength {
		return mapfile.Header{}, mapfile.Checkpoint{}, 0, fmt.Errorf("recovery map checkpoint is truncated")
	}
	checkpoint, err := mapfile.UnmarshalCheckpoint(data[headerLength : headerLength+checkpointLength])
	if err != nil {
		return mapfile.Header{}, mapfile.Checkpoint{}, 0, err
	}
	return header, checkpoint, headerLength + checkpointLength, nil
}

func (s *recoveryMapState) appendExtent(extent mapfile.Extent) error {
	record := mapfile.JournalRecord{
		Type:     mapfile.RecordExtentStateChanged,
		Sequence: s.nextSequence,
		Extent:   &extent,
	}
	encoded, err := mapfile.MarshalJournalRecord(record)
	if err != nil {
		return err
	}
	if _, err := s.journalFile.Write(encoded); err != nil {
		return err
	}
	if err := s.journalFile.Sync(); err != nil {
		return err
	}
	nextExtents, err := mapfile.InsertExtent(s.extents, extent)
	if err != nil {
		return err
	}
	s.extents = nextExtents
	s.nextSequence++
	return nil
}

func (s *recoveryMapState) close(canceled bool, success bool) error {
	if s == nil || s.journalFile == nil {
		return nil
	}
	if _, err := s.journalFile.Seek(0, io.SeekStart); err != nil {
		s.journalFile.Close()
		return err
	}
	s.header.CleanShutdown = canceled || success
	headerBytes, err := mapfile.MarshalHeader(s.header)
	if err != nil {
		s.journalFile.Close()
		return err
	}
	if _, err := s.journalFile.WriteAt(headerBytes, 0); err != nil {
		s.journalFile.Close()
		return err
	}
	if err := s.journalFile.Sync(); err != nil {
		s.journalFile.Close()
		return err
	}
	return s.journalFile.Close()
}

func summarizeExtents(extents []mapfile.Extent, logicalSectorSize uint32) (uint64, uint64) {
	var copiedBytes uint64
	var unreadable uint64
	for _, extent := range extents {
		if claimsImageData(extent.State) {
			copiedBytes += uint64(extent.Sectors) * uint64(logicalSectorSize)
			continue
		}
		if extent.State == mapfile.SectorStateMissing {
			unreadable += uint64(extent.Sectors)
		}
	}
	return copiedBytes, unreadable
}

func claimsImageData(state mapfile.SectorState) bool {
	switch state {
	case mapfile.SectorStateReadUnverified,
		mapfile.SectorStateVerified,
		mapfile.SectorStateChecksumError,
		mapfile.SectorStateConflicting,
		mapfile.SectorStateReconstructed:
		return true
	default:
		return false
	}
}

func requiredImageBytes(extents []mapfile.Extent, logicalSectorSize uint32) uint64 {
	var required uint64
	for _, extent := range extents {
		if !claimsImageData(extent.State) {
			continue
		}
		end := extent.EndLBA() * uint64(logicalSectorSize)
		if end > required {
			required = end
		}
	}
	return required
}

func mustMarshalHeader(header mapfile.Header) []byte {
	data, err := mapfile.MarshalHeader(header)
	if err != nil {
		panic(err)
	}
	return data
}

func recoveryMapPath(outputPath string) string {
	if strings.HasSuffix(outputPath, ".iso") {
		return strings.TrimSuffix(outputPath, ".iso") + ".drmap"
	}
	return outputPath + ".drmap"
}
