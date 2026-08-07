//go:build darwin

package platform

import (
	"context"
	"fmt"
	processcmd "os/exec"
	"path/filepath"
	"strings"
	"time"
)

const darwinDiskutilTimeout = 10 * time.Second

func runDarwinDiskutil(ctx context.Context, args ...string) ([]byte, error) {
	commandCtx, cancel := context.WithTimeout(ctx, darwinDiskutilTimeout)
	defer cancel()
	command := processcmd.CommandContext(commandCtx, "/usr/sbin/diskutil", args...)
	output, err := command.Output()
	if err != nil {
		if commandCtx.Err() != nil {
			return nil, fmt.Errorf("diskutil %s: %w", strings.Join(args, " "), commandCtx.Err())
		}
		return nil, fmt.Errorf("diskutil %s: %w", strings.Join(args, " "), err)
	}
	return output, nil
}

func discoverHostOpticalDrives() ([]OpticalDrive, error) {
	output, err := runDarwinDiskutil(context.Background(), "list")
	if err != nil {
		return nil, fmt.Errorf("discover macOS optical drives: %w", err)
	}
	return parseDarwinDiskutilList(string(output)), nil
}

func identifyHostOpticalMedia(path string) (OpticalMedia, error) {
	rawPath, err := normalizeDarwinOpticalDevice(path)
	if err != nil {
		return OpticalMedia{}, err
	}
	output, err := runDarwinDiskutil(context.Background(), "info", strings.TrimPrefix(path, "/dev/"))
	if err != nil {
		return OpticalMedia{}, fmt.Errorf("inspect macOS optical media %s: no disc detected or access is restricted: %w", path, err)
	}
	info, err := parseDarwinDiskutilInfo(string(output))
	if err != nil {
		return OpticalMedia{}, fmt.Errorf("inspect macOS optical media %s: %w", path, err)
	}
	if _, err := normalizeDarwinOpticalDevice(info.DevicePath); err != nil {
		return OpticalMedia{}, fmt.Errorf("inspect macOS optical media: diskutil reported an invalid device node %q", info.DevicePath)
	}
	name := info.VolumeName
	if name == "" {
		name = info.MediaName
	}
	if name == "" {
		name = filepath.Base(rawPath)
	}
	capacitySectors := info.CapacityBytes / uint64(info.LogicalSectorSize)
	if capacitySectors == 0 {
		return OpticalMedia{}, fmt.Errorf("inspect macOS optical media: media capacity is zero")
	}
	detail := fmt.Sprintf("%s - %d-byte sectors - %d sectors", info.FileSystem, info.LogicalSectorSize, capacitySectors)
	if info.FileSystem == "" {
		detail = fmt.Sprintf("%d-byte sectors - %d sectors", info.LogicalSectorSize, capacitySectors)
	}
	return OpticalMedia{
		Summary:             "Optical media detected.",
		Detail:              detail,
		FileSystem:          info.FileSystem,
		VolumeLabel:         info.VolumeName,
		LogicalSectorSize:   info.LogicalSectorSize,
		CapacitySectors:     capacitySectors,
		SuggestedOutputPath: filepath.Join(".", "discrescue-"+sanitizeDarwinName(name)+".iso"),
		Recoverable:         true,
	}, nil
}

func sanitizeDarwinName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.NewReplacer("/", "-", "\\", "-", ":", "-", " ", "-").Replace(name)
	if name == "" {
		return "optical-media"
	}
	return name
}
