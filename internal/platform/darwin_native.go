//go:build darwin

package platform

// This file intentionally uses only the Go standard library.  The Darwin
// device constants are copied from Apple's public xnu disk ioctl contract so
// cross-compiling does not require Xcode, a macOS SDK, or cgo.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

const (
	// DKIOCGETBLOCKSIZE and DKIOCGETBLOCKCOUNT are Darwin disk ioctls.
	dkioGetBlockSize  = uintptr(0x40046418)
	dkioGetBlockCount = uintptr(0x40086419)
	// DKIOCEJECT is the public Darwin ioctl for ejecting removable media.
	dkioEject = uintptr(0x20006415)
)

type darwinNativeDrive struct {
	Path              string
	DisplayName       string
	LogicalSectorSize uint32
	CapacityBytes     uint64
	RegistryID        uint64
	Media             bool
}

func nativeDarwinDiscover() ([]darwinNativeDrive, error) {
	paths, err := darwinDeviceCandidates()
	if err != nil {
		return nil, err
	}
	var drives []darwinNativeDrive
	for _, path := range paths {
		drive, ok := inspectDarwinDisk(path)
		if ok {
			drives = append(drives, drive)
		}
	}
	return drives, nil
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

func inspectDarwinDisk(path string) (darwinNativeDrive, bool) {
	whole, err := normalizeDarwinOpticalDevice(path)
	if err != nil {
		return darwinNativeDrive{}, false
	}
	f, err := os.OpenFile(whole, os.O_RDONLY, 0)
	if err != nil {
		return darwinNativeDrive{}, false
	}
	defer f.Close()
	var block uint32
	if err := darwinIoctl(f, dkioGetBlockSize, uintptr(unsafe.Pointer(&block))); err != nil || block == 0 {
		return darwinNativeDrive{}, false
	}
	var count uint64
	if err := darwinIoctl(f, dkioGetBlockCount, uintptr(unsafe.Pointer(&count))); err != nil {
		count = 0
	}
	return darwinNativeDrive{
		Path: whole, DisplayName: filepath.Base(path), LogicalSectorSize: block,
		CapacityBytes: count * uint64(block), Media: count > 0,
	}, true
}

func nativeDarwinEject(path string, _ bool) error {
	whole, err := normalizeDarwinOpticalDevice(path)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(whole, os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("open optical device: %w", err)
	}
	defer f.Close()
	if err := darwinIoctl(f, dkioEject, 0); err != nil {
		return fmt.Errorf("eject optical media: %w", err)
	}
	return nil
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
