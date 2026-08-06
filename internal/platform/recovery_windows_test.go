//go:build windows

package platform

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"discrescue/internal/mapfile"
)

type failingRangeReader struct {
	sectorSize uint32
	badLBAs    map[uint64]struct{}
}

func (r failingRangeReader) ReadAt(p []byte, off int64) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	sectorSize := int64(r.sectorSize)
	if sectorSize <= 0 || off%sectorSize != 0 || int64(len(p))%sectorSize != 0 {
		return 0, fmt.Errorf("unaligned read off=%d len=%d", off, len(p))
	}
	start := uint64(off / sectorSize)
	sectors := uint64(len(p)) / uint64(r.sectorSize)
	for lba := start; lba < start+sectors; lba++ {
		if _, bad := r.badLBAs[lba]; bad {
			return 0, errors.New("simulated read failure")
		}
	}
	for index := range p {
		p[index] = byte((int(start) + index) % 251)
	}
	return len(p), nil
}

func TestOSRecoveryRejectsExistingOutputPath(t *testing.T) {
	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "existing.iso")
	if err := os.WriteFile(outputPath, []byte("occupied"), 0o644); err != nil {
		t.Fatalf("seed existing output: %v", err)
	}

	_, err := OSRecovery{}.StartImageRecovery(RecoveryInput{
		DevicePath:        "E:",
		OutputPath:        outputPath,
		LogicalSectorSize: 2048,
		CapacitySectors:   16,
	})
	if err == nil {
		t.Fatal("expected start recovery to reject an existing output path")
	}
	if !strings.Contains(err.Error(), "already exists without") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOSRecoveryInspectTargetReportsSpaceForNewTarget(t *testing.T) {
	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "fresh.iso")

	status, err := OSRecovery{}.InspectRecoveryTarget(RecoveryInput{
		DevicePath:        "E:",
		OutputPath:        outputPath,
		LogicalSectorSize: 2048,
		CapacitySectors:   16,
	})
	if err != nil {
		t.Fatalf("inspect target: %v", err)
	}
	if !status.CanStartNew {
		t.Fatalf("expected a new target to be startable: %+v", status)
	}
	if status.RequiredBytes != 32768 {
		t.Fatalf("unexpected required bytes: %d", status.RequiredBytes)
	}
	if !status.SpaceKnown {
		t.Fatalf("expected known free-space data: %+v", status)
	}
	if status.AvailableBytes == 0 {
		t.Fatalf("expected non-zero available space: %+v", status)
	}
}

func TestOSRecoveryCopiesMountedDiscData(t *testing.T) {
	drive := strings.TrimSpace(os.Getenv("DISKRESCUE_WINDOWS_SMOKE_DRIVE"))
	if drive == "" {
		t.Skip("set DISKRESCUE_WINDOWS_SMOKE_DRIVE to run the Windows optical-drive smoke test")
	}

	media, err := identifyWindowsOpticalMedia(drive)
	if err != nil {
		t.Fatalf("identify media: %v", err)
	}

	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "smoke.iso")

	job, err := OSRecovery{}.StartImageRecovery(RecoveryInput{
		DevicePath:        drive,
		OutputPath:        outputPath,
		LogicalSectorSize: media.LogicalSectorSize,
		CapacitySectors:   media.CapacitySectors,
	})
	if err != nil {
		t.Fatalf("start recovery: %v", err)
	}

	deadline := time.Now().Add(30 * time.Second)
	for {
		snapshot := job.Snapshot()
		if snapshot.Done {
			if snapshot.ErrText != "" {
				t.Fatalf("recovery failed: %s", snapshot.ErrText)
			}
			break
		}
		if snapshot.CopiedBytes >= uint64(media.LogicalSectorSize)*32 {
			job.Cancel()
		}
		if time.Now().After(deadline) {
			job.Cancel()
			t.Fatal("recovery smoke test timed out")
		}
		time.Sleep(100 * time.Millisecond)
	}

	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("stat output: %v", err)
	}
	if info.Size() <= 0 {
		t.Fatalf("expected output image to contain data, got %d bytes", info.Size())
	}
}

func TestFastPassDefersFailedClusterAndKeepsScanning(t *testing.T) {
	tempDir := t.TempDir()
	input := RecoveryInput{
		DevicePath:        "E:",
		OutputPath:        filepath.Join(tempDir, "archive.iso"),
		LogicalSectorSize: 2048,
		CapacitySectors:   192,
	}
	state, _, err := createRecoveryMapState(input, recoveryMapPath(input.OutputPath))
	if err != nil {
		t.Fatalf("create recovery map state: %v", err)
	}
	defer state.close(false, false)

	output, err := os.OpenFile(input.OutputPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		t.Fatalf("create output image: %v", err)
	}
	defer output.Close()
	if err := output.Truncate(int64(input.CapacitySectors) * int64(input.LogicalSectorSize)); err != nil {
		t.Fatalf("truncate output image: %v", err)
	}

	job := &mountedRecoveryJob{state: state}
	buffer := make([]byte, int(input.LogicalSectorSize)*int(recoveryChunkSectors))
	reader := failingRangeReader{
		sectorSize: input.LogicalSectorSize,
		badLBAs:    map[uint64]struct{}{80: {}},
	}

	copied, err := job.runFastPass(context.Background(), reader, output, input, buffer, 0, uint64(input.LogicalSectorSize))
	if err != nil {
		t.Fatalf("run fast pass: %v", err)
	}
	if copied != uint64(128)*uint64(input.LogicalSectorSize) {
		t.Fatalf("unexpected copied bytes: %d", copied)
	}
	if deferred := countExtentsByState(state.extents, mapfile.SectorStateSkipped); deferred != recoveryChunkSectors {
		t.Fatalf("expected one deferred cluster, got %d sectors", deferred)
	}
	if unreadable := countExtentsByState(state.extents, mapfile.SectorStateMissing); unreadable != 0 {
		t.Fatalf("expected no unreadable sectors during fast pass, got %d", unreadable)
	}
	if pass := chooseRecoveryPass(state.extents, input.CapacitySectors); pass != recoveryPassRetry {
		t.Fatalf("expected retry pass after full fast scan, got %s", pass)
	}
}

func TestRetryPassNarrowsDeferredClusterToActualUnreadableSector(t *testing.T) {
	tempDir := t.TempDir()
	input := RecoveryInput{
		DevicePath:        "E:",
		OutputPath:        filepath.Join(tempDir, "archive.iso"),
		LogicalSectorSize: 2048,
		CapacitySectors:   192,
	}
	state, _, err := createRecoveryMapState(input, recoveryMapPath(input.OutputPath))
	if err != nil {
		t.Fatalf("create recovery map state: %v", err)
	}
	defer state.close(false, false)

	output, err := os.OpenFile(input.OutputPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		t.Fatalf("create output image: %v", err)
	}
	defer output.Close()
	if err := output.Truncate(int64(input.CapacitySectors) * int64(input.LogicalSectorSize)); err != nil {
		t.Fatalf("truncate output image: %v", err)
	}

	reader := failingRangeReader{
		sectorSize: input.LogicalSectorSize,
		badLBAs:    map[uint64]struct{}{80: {}},
	}
	job := &mountedRecoveryJob{state: state}
	buffer := make([]byte, int(input.LogicalSectorSize)*int(recoveryChunkSectors))
	copied, err := job.runFastPass(context.Background(), reader, output, input, buffer, 0, uint64(input.LogicalSectorSize))
	if err != nil {
		t.Fatalf("run fast pass: %v", err)
	}

	copied, err = job.runRetryPass(context.Background(), reader, output, input, copied, uint64(input.LogicalSectorSize))
	if err != nil {
		t.Fatalf("run retry pass: %v", err)
	}
	if copied != uint64(191)*uint64(input.LogicalSectorSize) {
		t.Fatalf("unexpected copied bytes after retry: %d", copied)
	}
	if deferred := countExtentsByState(state.extents, mapfile.SectorStateSkipped); deferred != 0 {
		t.Fatalf("expected retry pass to clear deferred sectors, got %d", deferred)
	}
	if unreadable := countExtentsByState(state.extents, mapfile.SectorStateMissing); unreadable != 1 {
		t.Fatalf("expected one unreadable sector after retry pass, got %d", unreadable)
	}
}
