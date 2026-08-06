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
	"golang.org/x/sys/windows"
)

type OSRecovery struct{}

type mountedRecoveryJob struct {
	cancel context.CancelFunc
	state  *recoveryMapState

	mu       sync.Mutex
	snapshot RecoverySnapshot
	pass     recoveryPassKind
}

type recoveryMapState struct {
	mapPath       string
	journalFile   *os.File
	header        mapfile.Header
	checkpointLen int
	nextSequence  uint64
	extents       []mapfile.Extent
}

type recoveryPassKind string

const (
	recoveryPassFast      recoveryPassKind = "fast"
	recoveryPassRetry     recoveryPassKind = "retry"
	recoveryChunkSectors                   = uint64(64)
	fastDamageSkipInitial                  = uint64(1024)
	fastDamageSkipMax                      = uint64(8192)
)

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
	j.snapshot.DeferredSectors = countExtentsByState(j.state.extents, mapfile.SectorStateSkipped)
	j.snapshot.UnreadableSectors = countExtentsByState(j.state.extents, mapfile.SectorStateMissing)
	j.snapshot.PassCoveredSectors = j.passCoveredSectorsLocked()
	j.mu.Unlock()
}

func (j *mountedRecoveryJob) setLastIssue(lines ...string) {
	j.mu.Lock()
	if len(lines) > 0 {
		j.snapshot.LastIssue = append([]string(nil), lines...)
	}
	j.mu.Unlock()
}

func (j *mountedRecoveryJob) setPass(pass recoveryPassKind, target uint64, resumed bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.pass = pass
	j.snapshot.PassStartedAt = time.Now()
	j.snapshot.PassTargetSectors = target
	j.snapshot.PassCoveredSectors = j.passCoveredSectorsLocked()
	j.snapshot.Phase = pass.phase()
	j.snapshot.Status = pass.status(resumed)
}

func (j *mountedRecoveryJob) passCoveredSectorsLocked() uint64 {
	switch j.pass {
	case recoveryPassRetry:
		if j.snapshot.PassTargetSectors == 0 {
			return 0
		}
		currentDeferred := countExtentsByState(j.state.extents, mapfile.SectorStateSkipped)
		if currentDeferred >= j.snapshot.PassTargetSectors {
			return 0
		}
		return j.snapshot.PassTargetSectors - currentDeferred
	default:
		return countCoveredSectors(j.state.extents)
	}
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
	deferredSectors := countExtentsByState(state.extents, mapfile.SectorStateSkipped)
	pass := chooseRecoveryPass(state.extents, input.CapacitySectors)
	lastIssue := []string{}
	if resumed {
		lastIssue = []string{"Resuming from durable recovery state."}
	}
	job := &mountedRecoveryJob{
		cancel: cancel,
		state:  state,
		snapshot: RecoverySnapshot{
			StartedAt:          time.Now(),
			PassStartedAt:      time.Now(),
			TotalBytes:         input.CapacitySectors * uint64(input.LogicalSectorSize),
			CopiedBytes:        copiedBytes,
			DeferredSectors:    deferredSectors,
			UnreadableSectors:  unreadableSectors,
			PassCoveredSectors: passCoveredSectors(pass, state.extents, input.CapacitySectors),
			PassTargetSectors:  passTargetSectors(pass, state.extents, input.CapacitySectors),
			MapPath:            state.mapPath,
			Resumed:            resumed,
			Phase:              pass.phase(),
			Status:             pass.status(resumed),
			LastIssue:          lastIssue,
		},
	}

	go job.run(ctx, input)
	return job, nil
}

func (OSRecovery) InspectRecoveryTarget(input RecoveryInput) (RecoveryTargetStatus, error) {
	if input.OutputPath == "" || input.OutputPath == "Not chosen yet" {
		return RecoveryTargetStatus{}, fmt.Errorf("inspect recovery target: output path is not configured")
	}
	status, err := inspectRecoveryTarget(input)
	if err != nil {
		return RecoveryTargetStatus{}, err
	}
	return status, nil
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

	chunkSize := int(input.LogicalSectorSize) * int(recoveryChunkSectors)
	if chunkSize < int(input.LogicalSectorSize) {
		chunkSize = int(input.LogicalSectorSize)
	}
	buffer := make([]byte, chunkSize)
	copied := j.snapshot.CopiedBytes
	logicalSectorSize := uint64(input.LogicalSectorSize)
	resumed := j.snapshot.Resumed
	for {
		pass := chooseRecoveryPass(j.state.extents, input.CapacitySectors)
		j.setPass(pass, passTargetSectors(pass, j.state.extents, input.CapacitySectors), resumed)
		resumed = false

		switch pass {
		case recoveryPassRetry:
			copied, err = j.runRetryPass(ctx, source, output, input, copied, logicalSectorSize)
		default:
			copied, err = j.runFastPass(ctx, source, output, input, buffer, copied, logicalSectorSize)
		}
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			j.setProgress(copied)
			j.finish(false, err)
			return
		}

		nextPass := chooseRecoveryPass(j.state.extents, input.CapacitySectors)
		if !shouldContinueRecovery(normalizedRecoveryPolicy(input.Policy), pass, nextPass) {
			break
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
	status, err := inspectRecoveryTarget(input)
	if err != nil {
		return nil, false, err
	}
	switch {
	case status.CanStartNew:
		mapPath := recoveryMapPath(input.OutputPath)
		return createRecoveryMapState(input, mapPath)
	case status.CanResume:
		return loadRecoveryMapState(input, status.MapPath)
	default:
		return nil, false, fmt.Errorf("start image recovery: %s", status.Detail)
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

func loadRecoveryMapState(input RecoveryInput, mapPath string) (*recoveryMapState, bool, error) {
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

func inspectRecoveryTarget(input RecoveryInput) (RecoveryTargetStatus, error) {
	mapPath := recoveryMapPath(input.OutputPath)
	requiredBytes := input.CapacitySectors * uint64(input.LogicalSectorSize)
	availableBytes, spaceKnown, spaceErr := availableBytesForOutputPath(input.OutputPath)
	outputInfo, outputErr := os.Stat(input.OutputPath)
	_, mapErr := os.Stat(mapPath)

	switch {
	case errors.Is(outputErr, os.ErrNotExist) && errors.Is(mapErr, os.ErrNotExist):
		if spaceErr == nil && spaceKnown && availableBytes < requiredBytes {
			return RecoveryTargetStatus{
				OutputPath:     input.OutputPath,
				MapPath:        mapPath,
				RequiredBytes:  requiredBytes,
				AvailableBytes: availableBytes,
				SpaceKnown:     true,
				Detail: fmt.Sprintf(
					"The selected output drive does not have enough free space for this image. Need %s and only %s are free. Choose another output path.",
					formatBytes(requiredBytes),
					formatBytes(availableBytes),
				),
			}, nil
		}
		return RecoveryTargetStatus{
			OutputPath:     input.OutputPath,
			MapPath:        mapPath,
			CanStartNew:    true,
			RequiredBytes:  requiredBytes,
			AvailableBytes: availableBytes,
			SpaceKnown:     spaceKnown && spaceErr == nil,
			Detail:         "A new recovery will be created at this path.",
		}, nil
	case outputErr == nil && mapErr == nil:
		data, err := os.ReadFile(mapPath)
		if err != nil {
			return RecoveryTargetStatus{}, fmt.Errorf("inspect recovery target: read recovery map %s: %w", mapPath, err)
		}
		header, checkpoint, journalOffset, err := readRecoveryMapBytes(data)
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
		if requiredImageBytes(replayed.Extents, input.LogicalSectorSize) > uint64(outputInfo.Size()) {
			return RecoveryTargetStatus{}, fmt.Errorf("inspect recovery target: image %s is smaller than the durable recovery map", input.OutputPath)
		}
		recoveredSectors, unreadableSectors := summarizeExtentsToSectors(replayed.Extents)
		deferredSectors := countExtentsByState(replayed.Extents, mapfile.SectorStateSkipped)
		return RecoveryTargetStatus{
			OutputPath:        input.OutputPath,
			MapPath:           mapPath,
			CanResume:         true,
			RecoveredSectors:  recoveredSectors,
			DeferredSectors:   deferredSectors,
			UnreadableSectors: unreadableSectors,
			RequiredBytes:     requiredBytes,
			AvailableBytes:    availableBytes,
			SpaceKnown:        spaceKnown && spaceErr == nil,
			Detail:            fmt.Sprintf("Resume recovery from %s recovered sectors, %s deferred sectors, and %s unreadable sectors.", formatUint(recoveredSectors), formatUint(deferredSectors), formatUint(unreadableSectors)),
		}, nil
	case outputErr == nil && errors.Is(mapErr, os.ErrNotExist):
		return RecoveryTargetStatus{
			OutputPath:     input.OutputPath,
			MapPath:        mapPath,
			RequiredBytes:  requiredBytes,
			AvailableBytes: availableBytes,
			SpaceKnown:     spaceKnown && spaceErr == nil,
			Detail:         fmt.Sprintf("Output image %s already exists without %s. Choose another output path.", input.OutputPath, mapPath),
		}, nil
	case errors.Is(outputErr, os.ErrNotExist) && mapErr == nil:
		return RecoveryTargetStatus{
			OutputPath:     input.OutputPath,
			MapPath:        mapPath,
			RequiredBytes:  requiredBytes,
			AvailableBytes: availableBytes,
			SpaceKnown:     spaceKnown && spaceErr == nil,
			Detail:         fmt.Sprintf("Recovery map %s exists without image %s. Choose another output path.", mapPath, input.OutputPath),
		}, nil
	case outputErr != nil && !errors.Is(outputErr, os.ErrNotExist):
		return RecoveryTargetStatus{}, fmt.Errorf("inspect recovery target: check output image %s: %w", input.OutputPath, outputErr)
	default:
		return RecoveryTargetStatus{}, fmt.Errorf("inspect recovery target: check recovery map %s: %w", mapPath, mapErr)
	}
}

func availableBytesForOutputPath(outputPath string) (uint64, bool, error) {
	absolutePath, err := filepath.Abs(outputPath)
	if err != nil {
		return 0, false, fmt.Errorf("resolve output path %s: %w", outputPath, err)
	}
	volume := filepath.VolumeName(absolutePath)
	if volume == "" {
		return 0, false, nil
	}
	root := volume + `\`
	rootPtr, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return 0, false, fmt.Errorf("encode output root %s: %w", root, err)
	}
	var freeCaller uint64
	var totalBytes uint64
	var freeTotal uint64
	if err := windows.GetDiskFreeSpaceEx(rootPtr, &freeCaller, &totalBytes, &freeTotal); err != nil {
		return 0, false, fmt.Errorf("check free space for %s: %w", root, err)
	}
	return freeTotal, true, nil
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
	nextExtents, err := mapfile.ApplyExtent(s.extents, extent)
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
	recoveredSectors, unreadable := summarizeExtentsToSectors(extents)
	return recoveredSectors * uint64(logicalSectorSize), unreadable
}

func summarizeExtentsToSectors(extents []mapfile.Extent) (uint64, uint64) {
	var recoveredSectors uint64
	var unreadable uint64
	for _, extent := range extents {
		if claimsImageData(extent.State) {
			recoveredSectors += uint64(extent.Sectors)
			continue
		}
		if extent.State == mapfile.SectorStateMissing {
			unreadable += uint64(extent.Sectors)
		}
	}
	return recoveredSectors, unreadable
}

func chooseRecoveryPass(extents []mapfile.Extent, capacity uint64) recoveryPassKind {
	if hasCoverageGap(extents, capacity) {
		return recoveryPassFast
	}
	if countExtentsByState(extents, mapfile.SectorStateSkipped) > 0 {
		return recoveryPassRetry
	}
	return recoveryPassFast
}

func passTargetSectors(pass recoveryPassKind, extents []mapfile.Extent, capacity uint64) uint64 {
	switch pass {
	case recoveryPassRetry:
		return countExtentsByState(extents, mapfile.SectorStateSkipped)
	default:
		return capacity
	}
}

func passCoveredSectors(pass recoveryPassKind, extents []mapfile.Extent, capacity uint64) uint64 {
	switch pass {
	case recoveryPassRetry:
		target := countExtentsByState(extents, mapfile.SectorStateSkipped)
		if target == 0 {
			return 0
		}
		return 0
	default:
		return countCoveredSectors(extents)
	}
}

func countCoveredSectors(extents []mapfile.Extent) uint64 {
	var total uint64
	for _, extent := range extents {
		total += uint64(extent.Sectors)
	}
	return total
}

func hasCoverageGap(extents []mapfile.Extent, capacity uint64) bool {
	var next uint64
	for _, extent := range extents {
		if extent.StartLBA > next {
			return true
		}
		if extent.EndLBA() > next {
			next = extent.EndLBA()
		}
	}
	return next < capacity
}

func countExtentsByState(extents []mapfile.Extent, state mapfile.SectorState) uint64 {
	var total uint64
	for _, extent := range extents {
		if extent.State == state {
			total += uint64(extent.Sectors)
		}
	}
	return total
}

func (p recoveryPassKind) phase() string {
	switch p {
	case recoveryPassRetry:
		return "Retrying deferred sectors"
	default:
		return "Fast acquisition pass"
	}
}

func (p recoveryPassKind) status(resumed bool) string {
	switch p {
	case recoveryPassRetry:
		if resumed {
			return "Retrying deferred sectors from the saved recovery map."
		}
		return "Retrying deferred sectors."
	default:
		if resumed {
			return "Continuing the fast acquisition pass from the saved recovery map."
		}
		return "Reading readable sectors and deferring damaged ranges."
	}
}

func normalizedRecoveryPolicy(policy RecoveryPolicy) RecoveryPolicy {
	switch policy {
	case RecoveryPolicyContinueRetry:
		return policy
	default:
		return RecoveryPolicyFast
	}
}

func shouldContinueRecovery(policy RecoveryPolicy, completed recoveryPassKind, next recoveryPassKind) bool {
	if completed == recoveryPassFast && next == recoveryPassRetry {
		return policy == RecoveryPolicyContinueRetry
	}
	return false
}

func (j *mountedRecoveryJob) runFastPass(ctx context.Context, source io.ReaderAt, output *os.File, input RecoveryInput, buffer []byte, copied uint64, logicalSectorSize uint64) (uint64, error) {
	consecutiveFailures := uint8(0)
	for lba := uint64(0); lba < input.CapacitySectors; {
		if err := checkRecoveryCanceled(ctx, copied, j); err != nil {
			return copied, err
		}
		if extent, _, ok := mapfile.LookupExtent(j.state.extents, lba); ok {
			lba = extent.EndLBA()
			j.setProgress(copied)
			continue
		}

		sectorsToRead := recoveryChunkSectors
		if remaining := input.CapacitySectors - lba; remaining < sectorsToRead {
			sectorsToRead = remaining
		}
		readSize := int(sectorsToRead * logicalSectorSize)
		offset := int64(lba * logicalSectorSize)

		n, err := source.ReadAt(buffer[:readSize], offset)
		if err == nil || (err == io.EOF && n == readSize) {
			if err := writeFullAt(output, buffer[:readSize], offset); err != nil {
				return copied, fmt.Errorf("write output image %s at byte %d: %w", input.OutputPath, offset, err)
			}
			if err := j.state.appendExtent(mapfile.Extent{
				StartLBA:   lba,
				Sectors:    uint32(sectorsToRead),
				State:      mapfile.SectorStateReadUnverified,
				Confidence: mapfile.ConfidenceSingleRead,
			}); err != nil {
				return copied, fmt.Errorf("persist recovery map %s: %w", j.state.mapPath, err)
			}
			copied += uint64(readSize)
			j.setProgress(copied)
			lba += sectorsToRead
			consecutiveFailures = 0
			continue
		}

		deferredSectors := fastDamageDeferralSectors(sectorsToRead, consecutiveFailures, input.CapacitySectors-lba)
		if err := j.state.appendExtent(mapfile.Extent{
			StartLBA:   lba,
			Sectors:    uint32(deferredSectors),
			State:      mapfile.SectorStateSkipped,
			Confidence: mapfile.ConfidenceNone,
		}); err != nil {
			return copied, fmt.Errorf("persist recovery map %s: %w", j.state.mapPath, err)
		}
		j.setProgress(copied)
		j.setLastIssue(
			fmt.Sprintf("Deferred damaged area at LBA %d to %d.", lba, lba+deferredSectors-1),
			"The fast pass jumped ahead to find the next readable area.",
		)
		lba += deferredSectors
		if consecutiveFailures < 8 {
			consecutiveFailures++
		}
	}
	return copied, nil
}

func fastDamageDeferralSectors(requested uint64, consecutiveFailures uint8, remaining uint64) uint64 {
	if requested == 0 || remaining == 0 {
		return 0
	}
	deferral := fastDamageSkipInitial
	for i := uint8(0); i < consecutiveFailures && deferral < fastDamageSkipMax; i++ {
		deferral *= 2
		if deferral > fastDamageSkipMax {
			deferral = fastDamageSkipMax
		}
	}
	if deferral < requested {
		deferral = requested
	}
	if deferral > remaining {
		return remaining
	}
	return deferral
}

func (j *mountedRecoveryJob) runRetryPass(ctx context.Context, source io.ReaderAt, output *os.File, input RecoveryInput, copied uint64, logicalSectorSize uint64) (uint64, error) {
	singleSector := int(input.LogicalSectorSize)
	sectorBuffer := make([]byte, singleSector)

	for lba := uint64(0); lba < input.CapacitySectors; {
		if err := checkRecoveryCanceled(ctx, copied, j); err != nil {
			return copied, err
		}
		extent, _, ok := mapfile.LookupExtent(j.state.extents, lba)
		if !ok {
			lba++
			continue
		}
		if extent.State != mapfile.SectorStateSkipped {
			lba = extent.EndLBA()
			j.setProgress(copied)
			continue
		}

		clusterStart := extent.StartLBA
		clusterEnd := extent.EndLBA()
		for sectorLBA := clusterStart; sectorLBA < clusterEnd; sectorLBA++ {
			if err := checkRecoveryCanceled(ctx, copied, j); err != nil {
				return copied, err
			}
			sectorOffset := int64(sectorLBA * logicalSectorSize)
			n, err := source.ReadAt(sectorBuffer, sectorOffset)
			if err == nil || (err == io.EOF && n == singleSector) {
				if err := writeFullAt(output, sectorBuffer[:singleSector], sectorOffset); err != nil {
					return copied, fmt.Errorf("write output image %s at byte %d: %w", input.OutputPath, sectorOffset, err)
				}
				if err := j.state.appendExtent(mapfile.Extent{
					StartLBA:   sectorLBA,
					Sectors:    1,
					State:      mapfile.SectorStateReadUnverified,
					Confidence: mapfile.ConfidenceSingleRead,
				}); err != nil {
					return copied, fmt.Errorf("persist recovery map %s: %w", j.state.mapPath, err)
				}
				copied += logicalSectorSize
				j.setProgress(copied)
				continue
			}

			if err := j.state.appendExtent(mapfile.Extent{
				StartLBA:   sectorLBA,
				Sectors:    1,
				State:      mapfile.SectorStateMissing,
				Confidence: mapfile.ConfidenceNone,
			}); err != nil {
				return copied, fmt.Errorf("persist recovery map %s: %w", j.state.mapPath, err)
			}
			j.setProgress(copied)
			j.setLastIssue(
				fmt.Sprintf("Unreadable sector at LBA %d.", sectorLBA),
				fmt.Sprintf("Retry pass narrowed the deferred range %d to %d.", clusterStart, clusterEnd-1),
			)
		}
		lba = clusterEnd
	}
	return copied, nil
}

func checkRecoveryCanceled(ctx context.Context, copied uint64, job *mountedRecoveryJob) error {
	select {
	case <-ctx.Done():
		job.setProgress(copied)
		job.finish(true, nil)
		return context.Canceled
	default:
		return nil
	}
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

func formatUint(value uint64) string {
	return fmt.Sprintf("%d", value)
}

func formatBytes(value uint64) string {
	const unit = 1024
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}
	div, exp := uint64(unit), 0
	for n := value / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(value)/float64(div), "KMGTPE"[exp])
}
