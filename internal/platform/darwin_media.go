//go:build darwin

package platform

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"unsafe"
)

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
