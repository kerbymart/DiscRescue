//go:build windows

package platform

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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
