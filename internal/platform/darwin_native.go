//go:build darwin

package platform

// This file intentionally uses only the Go standard library.  The Darwin
// device constants are copied from Apple's public xnu disk ioctl contract so
// cross-compiling does not require Xcode, a macOS SDK, or cgo.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	// DKIOCGETBLOCKSIZE and DKIOCGETBLOCKCOUNT are Darwin disk ioctls.
	dkioGetBlockSize  = uintptr(0x40046418)
	dkioGetBlockCount = uintptr(0x40086419)
	// DKIOCEJECT is the public Darwin ioctl for ejecting removable media.
	dkioEject = uintptr(0x20006415)

	darwinForceEjectTimeout  = 10 * time.Second
	darwinEjectDiagnosticMax = 4 << 10
)

type darwinNativeDrive struct {
	Path              string
	DisplayName       string
	LogicalSectorSize uint32
	CapacityBytes     uint64
	RegistryID        uint64
	Media             bool
	State             MediaProbeState
}

func nativeDarwinDiscover() ([]darwinNativeDrive, error) {
	paths, err := darwinDeviceCandidates()
	if err != nil {
		return nil, err
	}
	configured := darwinOpticalDevicesConfigured()
	var drives []darwinNativeDrive
	for _, path := range paths {
		drive, err := inspectDarwinDisk(path)
		if shouldRetainDarwinCandidate(configured, err) {
			drives = append(drives, drive)
			continue
		}
	}
	return drives, nil
}

func shouldRetainDarwinCandidate(configured bool, err error) bool {
	if err == nil {
		// Geometry was read from the node, which is the positive device evidence
		// available to automatic, pure-Go discovery.
		return true
	}
	if !configured {
		// /dev/diskN includes every storage device. A permission or I/O failure
		// cannot establish that an automatically discovered node is optical.
		return false
	}
	var probeErr *MediaInspectionError
	return errors.As(err, &probeErr) && probeErr.State != MediaProbeUnavailable
}

func darwinOpticalDevicesConfigured() bool {
	return strings.TrimSpace(os.Getenv("DISKRESCUE_DARWIN_OPTICAL_DEVICES")) != ""
}

func darwinDeviceCandidates() ([]string, error) {
	// This override is useful for systems where the optical device node is
	// supplied by a vendor driver and is also deterministic in integration
	// tests. It never invokes a host command.
	if configured := strings.TrimSpace(os.Getenv("DISKRESCUE_DARWIN_OPTICAL_DEVICES")); configured != "" {
		var paths []string
		for _, value := range strings.Split(configured, ",") {
			path := strings.TrimSpace(value)
			if path != "" {
				paths = append(paths, path)
			}
		}
		return paths, nil
	}
	entries, err := os.ReadDir("/dev")
	if err != nil {
		return nil, fmt.Errorf("enumerate Darwin device nodes: %w", err)
	}
	var paths []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, "disk") && isDecimal(name[len("disk"):]) {
			paths = append(paths, filepath.Join("/dev", name))
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func inspectDarwinDisk(path string) (darwinNativeDrive, error) {
	whole, err := normalizeDarwinOpticalDevice(path)
	if err != nil {
		return darwinNativeDrive{}, &MediaInspectionError{Path: path, Operation: "normalize device", State: MediaProbeFailure, Err: err}
	}
	drive := darwinNativeDrive{Path: whole, DisplayName: filepath.Base(path)}
	f, err := os.OpenFile(whole, os.O_RDONLY, 0)
	if err != nil {
		return drive, darwinProbeError(path, "open", err)
	}
	defer f.Close()
	var block uint32
	if err := darwinIoctl(f, dkioGetBlockSize, uintptr(unsafe.Pointer(&block))); err != nil {
		return drive, darwinProbeError(path, "DKIOCGETBLOCKSIZE", err)
	}
	if block == 0 {
		return drive, &MediaInspectionError{Path: path, Operation: "DKIOCGETBLOCKSIZE", State: MediaProbeFailure, Err: fmt.Errorf("reported zero block size")}
	}
	var count uint64
	if err := darwinIoctl(f, dkioGetBlockCount, uintptr(unsafe.Pointer(&count))); err != nil {
		return drive, darwinProbeError(path, "DKIOCGETBLOCKCOUNT", err)
	}
	drive.LogicalSectorSize = block
	drive.CapacityBytes = count * uint64(block)
	drive.Media = count > 0
	if !drive.Media {
		drive.State = MediaProbeNoMedia
	}
	return drive, nil
}

func darwinProbeError(path, operation string, err error) error {
	state := MediaProbeFailure
	switch {
	case errors.Is(err, syscall.ENOENT), errors.Is(err, syscall.ENODEV):
		state = MediaProbeUnavailable
	case errors.Is(err, syscall.ENXIO), errors.Is(err, syscall.EIO):
		state = MediaProbeNotReady
	case errors.Is(err, syscall.EACCES), errors.Is(err, syscall.EPERM):
		state = MediaProbePermission
	case errors.Is(err, syscall.EBUSY):
		state = MediaProbeBusy
	}
	return &MediaInspectionError{Path: path, Operation: operation, State: state, Err: err}
}

func nativeDarwinEject(path string, force bool) error {
	whole, err := normalizeDarwinOpticalDevice(path)
	if err != nil {
		return err
	}
	if force {
		return nativeDarwinDiskutilEject(whole, "force")
	}
	return nativeDarwinDiskutilEject(whole, "normal")
}

// nativeDarwinDiskutilEject is the macOS eject request for a normalized raw
// optical device. Unlike DKIOCEJECT, Disk Utility coordinates a mounted-volume
// eject. Force mode reaches this same mechanism only after explicit UI
// confirmation; it is retained for platforms with a distinct escalation.
func nativeDarwinDiskutilEject(rawPath, mode string) error {
	ctx, cancel := context.WithTimeout(context.Background(), darwinForceEjectTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, "/usr/sbin/diskutil", "eject", rawPath)
	var diagnostics limitedDarwinEjectDiagnostics
	command.Stderr = &diagnostics
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("macOS %s eject timed out after %s: %w", mode, darwinForceEjectTimeout, ctx.Err())
		}
		if detail := strings.TrimSpace(diagnostics.String()); detail != "" {
			return fmt.Errorf("macOS %s eject: %w: %s", mode, err, detail)
		}
		return fmt.Errorf("macOS %s eject: %w", mode, err)
	}
	return nil
}

// limitedDarwinEjectDiagnostics preserves enough native utility context for
// the user without allowing an external command to retain unbounded output.
type limitedDarwinEjectDiagnostics struct{ bytes.Buffer }

func (b *limitedDarwinEjectDiagnostics) Write(value []byte) (int, error) {
	remaining := darwinEjectDiagnosticMax - b.Len()
	if remaining > 0 {
		if len(value) > remaining {
			_, _ = b.Buffer.Write(value[:remaining])
		} else {
			_, _ = b.Buffer.Write(value)
		}
	}
	return len(value), nil
}

func darwinIoctl(file *os.File, request, argument uintptr) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), request, argument)
	if errno != 0 {
		return errno
	}
	return nil
}

func isDecimal(value string) bool {
	if value == "" {
		return false
	}
	_, err := strconv.Atoi(value)
	return err == nil
}
