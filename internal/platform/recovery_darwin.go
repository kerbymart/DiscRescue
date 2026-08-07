//go:build darwin

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

type darwinRecoveryMapState struct {
	mapPath      string
	journalFile  *os.File
	header       mapfile.Header
	nextSequence uint64
	extents      []mapfile.Extent
}

type darwinRecoveryJob struct {
	cancel context.CancelFunc
	state  *darwinRecoveryMapState

	mu       sync.Mutex
	snapshot RecoverySnapshot
}

func (j *darwinRecoveryJob) Snapshot() RecoverySnapshot {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.snapshot
}

func (j *darwinRecoveryJob) Cancel() { j.cancel() }

func (j *darwinRecoveryJob) setProgress(progress recoveryPassProgress, sectorSize uint32) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.snapshot.CopiedBytes = progress.RecoveredSectors * uint64(sectorSize)
	j.snapshot.ScannedSectors = progress.ScannedSectors
	j.snapshot.DeferredSectors = progress.DeferredSectors
	j.snapshot.UnreadableSectors = progress.UnreadableSectors
	j.snapshot.Pass = progress.Pass
	j.snapshot.LastIssue = append([]string(nil), progress.LastIssue...)
}

func (j *darwinRecoveryJob) finish(canceled bool, err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.snapshot.Done = true
	j.snapshot.Canceled = canceled
	if err != nil {
		j.snapshot.ErrText = err.Error()
	}
	if j.state != nil {
		_ = j.state.close(canceled || err == nil)
	}
}

func (OSRecovery) StartImageRecovery(input RecoveryInput) (RecoveryJob, error) {
	if input.DevicePath == "" || input.OutputPath == "" || input.OutputPath == "Not chosen yet" {
		return nil, fmt.Errorf("start image recovery: device and output paths are required")
	}
	if input.LogicalSectorSize == 0 || input.CapacitySectors == 0 {
		return nil, fmt.Errorf("start image recovery: media geometry is required")
	}
	state, resumed, err := openDarwinRecoveryMap(input)
	if err != nil {
		return nil, err
	}
	_, recovered, deferred, unreadable := summarizeRecoveryExtentStates(state.extents)
	ctx, cancel := context.WithCancel(context.Background())
	job := &darwinRecoveryJob{
		cancel: cancel,
		state:  state,
		snapshot: RecoverySnapshot{
			StartedAt: time.Now(), TotalBytes: input.CapacitySectors * uint64(input.LogicalSectorSize),
			CopiedBytes: recovered * uint64(input.LogicalSectorSize), ScannedSectors: recovered + deferred + unreadable,
			DeferredSectors: deferred, UnreadableSectors: unreadable, Pass: "Fast acquisition", MapPath: state.mapPath, Resumed: resumed,
		},
	}
	go job.run(ctx, input)
	return job, nil
}

func (OSRecovery) InspectRecoveryTarget(input RecoveryInput) (RecoveryTargetStatus, error) {
	if input.OutputPath == "" || input.OutputPath == "Not chosen yet" {
		return RecoveryTargetStatus{}, fmt.Errorf("inspect recovery target: output path is not configured")
	}
	mapPath := darwinRecoveryMapPath(input.OutputPath)
	outputInfo, outputErr := os.Stat(input.OutputPath)
	_, mapErr := os.Stat(mapPath)
	status := RecoveryTargetStatus{OutputPath: input.OutputPath, MapPath: mapPath, RequiredBytes: input.CapacitySectors * uint64(input.LogicalSectorSize)}
	switch {
	case errors.Is(outputErr, os.ErrNotExist) && errors.Is(mapErr, os.ErrNotExist):
		status.CanStartNew = true
		status.Detail = "A new recovery will be created at this path."
		return status, nil
	case outputErr == nil && mapErr == nil:
		data, err := os.ReadFile(mapPath)
		if err != nil {
			return RecoveryTargetStatus{}, fmt.Errorf("inspect recovery target: read recovery map %s: %w", mapPath, err)
		}
		header, checkpoint, journalOffset, err := readDarwinRecoveryMap(data)
		if err != nil {
			return RecoveryTargetStatus{}, fmt.Errorf("inspect recovery target: load recovery map %s: %w", mapPath, err)
		}
		if header.LogicalSectorSize != input.LogicalSectorSize || header.ExpectedSectorCount != input.CapacitySectors {
			return RecoveryTargetStatus{}, fmt.Errorf("inspect recovery target: recovery map %s does not match the selected media", mapPath)
		}
		replayed, err := mapfile.ReplayJournal(checkpoint, data[journalOffset:])
		if err != nil {
			return RecoveryTargetStatus{}, fmt.Errorf("inspect recovery target: replay recovery map %s: %w", mapPath, err)
		}
		if requiredImageBytesDarwin(replayed.Extents, input.LogicalSectorSize) > uint64(outputInfo.Size()) {
			return RecoveryTargetStatus{}, fmt.Errorf("inspect recovery target: image %s is smaller than the durable recovery map", input.OutputPath)
		}
		_, recovered, deferred, unreadable := summarizeRecoveryExtentStates(replayed.Extents)
		status.CanResume = true
		status.RecoveredSectors, status.DeferredSectors, status.UnreadableSectors = recovered, deferred, unreadable
		status.Detail = fmt.Sprintf("Resume recovery from %d recovered sectors, %d deferred sectors, and %d unreadable sectors.", recovered, deferred, unreadable)
		return status, nil
	case outputErr == nil && errors.Is(mapErr, os.ErrNotExist):
		status.Detail = fmt.Sprintf("Output image %s already exists without %s. Choose another output path.", input.OutputPath, mapPath)
		return status, nil
	case errors.Is(outputErr, os.ErrNotExist) && mapErr == nil:
		status.Detail = fmt.Sprintf("Recovery map %s exists without image %s. Choose another output path.", mapPath, input.OutputPath)
		return status, nil
	default:
		return RecoveryTargetStatus{}, fmt.Errorf("inspect recovery target: check output %s and map %s", input.OutputPath, mapPath)
	}
}

func (j *darwinRecoveryJob) run(ctx context.Context, input RecoveryInput) {
	rawPath, err := normalizeDarwinOpticalDevice(input.DevicePath)
	if err != nil {
		j.finish(false, fmt.Errorf("open macOS optical source: %w", err))
		return
	}
	source, err := os.Open(rawPath)
	if err != nil {
		j.finish(false, fmt.Errorf("open macOS optical source %s read-only: %w", rawPath, err))
		return
	}
	defer source.Close()
	if dir := filepath.Dir(input.OutputPath); dir != "." {
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
	if err := output.Truncate(int64(input.CapacitySectors) * int64(input.LogicalSectorSize)); err != nil {
		j.finish(false, fmt.Errorf("prepare output image %s: %w", input.OutputPath, err))
		return
	}
	err = runPassBasedRecovery(ctx, source, output, input.LogicalSectorSize, input.CapacitySectors, j.state, func(progress recoveryPassProgress) { j.setProgress(progress, input.LogicalSectorSize) })
	if errors.Is(err, context.Canceled) {
		j.finish(true, nil)
		return
	}
	if err != nil {
		j.finish(false, fmt.Errorf("recover image %s: %w", input.OutputPath, err))
		return
	}
	if err := output.Sync(); err != nil {
		j.finish(false, fmt.Errorf("sync output image %s: %w", input.OutputPath, err))
		return
	}
	j.finish(false, nil)
}

func openDarwinRecoveryMap(input RecoveryInput) (*darwinRecoveryMapState, bool, error) {
	status, err := (OSRecovery{}).InspectRecoveryTarget(input)
	if err != nil {
		return nil, false, err
	}
	if status.CanStartNew {
		return createDarwinRecoveryMap(input, status.MapPath)
	}
	if status.CanResume {
		return loadDarwinRecoveryMap(input, status.MapPath)
	}
	return nil, false, fmt.Errorf("start image recovery: %s", status.Detail)
}

func createDarwinRecoveryMap(input RecoveryInput, mapPath string) (*darwinRecoveryMapState, bool, error) {
	headerBytes, err := mapfile.MarshalHeader(mapfile.Header{LogicalSectorSize: input.LogicalSectorSize, ExpectedSectorCount: input.CapacitySectors, OutputFormat: 1, CreationUnixNano: time.Now().UnixNano()})
	if err != nil {
		return nil, false, fmt.Errorf("create recovery map header: %w", err)
	}
	checkpointBytes, err := mapfile.MarshalCheckpoint(mapfile.Checkpoint{})
	if err != nil {
		return nil, false, fmt.Errorf("create recovery map checkpoint: %w", err)
	}
	if dir := filepath.Dir(mapPath); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, false, err
		}
	}
	file, err := os.OpenFile(mapPath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o644)
	if err != nil {
		return nil, false, fmt.Errorf("create recovery map %s: %w", mapPath, err)
	}
	if _, err = file.Write(append(headerBytes, checkpointBytes...)); err != nil {
		file.Close()
		return nil, false, err
	}
	if err = file.Sync(); err != nil {
		file.Close()
		return nil, false, err
	}
	return &darwinRecoveryMapState{mapPath: mapPath, journalFile: file, header: mapfile.Header{LogicalSectorSize: input.LogicalSectorSize, ExpectedSectorCount: input.CapacitySectors, OutputFormat: 1}, nextSequence: 1}, false, nil
}

func loadDarwinRecoveryMap(input RecoveryInput, mapPath string) (*darwinRecoveryMapState, bool, error) {
	data, err := os.ReadFile(mapPath)
	if err != nil {
		return nil, false, err
	}
	header, checkpoint, offset, err := readDarwinRecoveryMap(data)
	if err != nil {
		return nil, false, err
	}
	replayed, err := mapfile.ReplayJournal(checkpoint, data[offset:])
	if err != nil {
		return nil, false, err
	}
	if header.LogicalSectorSize != input.LogicalSectorSize || header.ExpectedSectorCount != input.CapacitySectors {
		return nil, false, fmt.Errorf("recovery map %s does not match the selected media", mapPath)
	}
	file, err := os.OpenFile(mapPath, os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		return nil, false, err
	}
	return &darwinRecoveryMapState{mapPath: mapPath, journalFile: file, header: header, nextSequence: replayed.LastSequence + 1, extents: append([]mapfile.Extent(nil), replayed.Extents...)}, true, nil
}

func readDarwinRecoveryMap(data []byte) (mapfile.Header, mapfile.Checkpoint, int, error) {
	if len(data) < 8 {
		return mapfile.Header{}, mapfile.Checkpoint{}, 0, fmt.Errorf("recovery map is too short")
	}
	headerLength := int(binary.LittleEndian.Uint16(data[6:8]))
	if len(data) < headerLength+10 {
		return mapfile.Header{}, mapfile.Checkpoint{}, 0, fmt.Errorf("recovery map checkpoint is truncated")
	}
	header, err := mapfile.UnmarshalHeader(data[:headerLength])
	if err != nil {
		return mapfile.Header{}, mapfile.Checkpoint{}, 0, err
	}
	payloadLength := int(binary.LittleEndian.Uint32(data[headerLength+6 : headerLength+10]))
	checkpointLength := 14 + payloadLength
	if len(data) < headerLength+checkpointLength {
		return mapfile.Header{}, mapfile.Checkpoint{}, 0, fmt.Errorf("recovery map checkpoint is truncated")
	}
	checkpoint, err := mapfile.UnmarshalCheckpoint(data[headerLength : headerLength+checkpointLength])
	if err != nil {
		return mapfile.Header{}, mapfile.Checkpoint{}, 0, err
	}
	return header, checkpoint, headerLength + checkpointLength, nil
}

func (s *darwinRecoveryMapState) Extents() []mapfile.Extent {
	return append([]mapfile.Extent(nil), s.extents...)
}

func (s *darwinRecoveryMapState) ApplyExtent(extent mapfile.Extent) error {
	next, err := mapfile.ApplyExtent(s.extents, extent)
	if err != nil {
		return err
	}
	record, err := mapfile.MarshalJournalRecord(mapfile.JournalRecord{Type: mapfile.RecordExtentStateChanged, Sequence: s.nextSequence, Extent: &extent})
	if err != nil {
		return err
	}
	if _, err = s.journalFile.Write(record); err != nil {
		return err
	}
	if err = s.journalFile.Sync(); err != nil {
		return err
	}
	s.extents, s.nextSequence = next, s.nextSequence+1
	return nil
}

func (s *darwinRecoveryMapState) close(clean bool) error {
	if s == nil || s.journalFile == nil {
		return nil
	}
	s.header.CleanShutdown = clean
	header, err := mapfile.MarshalHeader(s.header)
	if err != nil {
		s.journalFile.Close()
		return err
	}
	if _, err = s.journalFile.WriteAt(header, 0); err != nil {
		s.journalFile.Close()
		return err
	}
	if err = s.journalFile.Sync(); err != nil {
		s.journalFile.Close()
		return err
	}
	return s.journalFile.Close()
}

func darwinRecoveryMapPath(outputPath string) string {
	if strings.HasSuffix(outputPath, ".iso") {
		return strings.TrimSuffix(outputPath, ".iso") + ".drmap"
	}
	return outputPath + ".drmap"
}

func requiredImageBytesDarwin(extents []mapfile.Extent, logicalSectorSize uint32) uint64 {
	var required uint64
	for _, extent := range extents {
		if !recoveryStateHasData(extent.State) {
			continue
		}
		end := extent.EndLBA() * uint64(logicalSectorSize)
		if end > required {
			required = end
		}
	}
	return required
}

var _ io.ReaderAt = (*os.File)(nil)
