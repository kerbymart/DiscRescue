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

const darwinDrutilTimeout = 10 * time.Second

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

func runDarwinDrutil(ctx context.Context, args ...string) ([]byte, error) {
	commandCtx, cancel := context.WithTimeout(ctx, darwinDrutilTimeout)
	defer cancel()
	command := processcmd.CommandContext(commandCtx, "/usr/bin/drutil", args...)
	output, err := command.Output()
	if err != nil {
		if commandCtx.Err() != nil {
			return nil, fmt.Errorf("drutil %s: %w", strings.Join(args, " "), commandCtx.Err())
		}
		return nil, fmt.Errorf("drutil %s: %w", strings.Join(args, " "), err)
	}
	return output, nil
}

func discoverHostOpticalDrives() ([]OpticalDrive, error) {
	output, err := runDarwinDiskutil(context.Background(), "list")
	if err != nil {
		return nil, fmt.Errorf("discover macOS optical drives: %w", err)
	}
	candidates := parseDarwinDiskutilList(string(output))
	drives := make([]OpticalDrive, 0, len(candidates))
	for _, candidate := range candidates {
		infoOutput, infoErr := runDarwinDiskutil(context.Background(), "info", strings.TrimPrefix(candidate.Path, "/dev/"))
		if infoErr != nil {
			continue
		}
		info, infoErr := parseDarwinDiskutilInfo(string(infoOutput))
		if infoErr != nil || !info.OpticalMedia {
			continue
		}
		candidate.DisplayName = darwinDriveDisplayName(info, candidate.Path)
		if info.LogicalSectorSize == 0 || info.CapacityBytes == 0 {
			candidate.Status = "optical drive present; media geometry unavailable"
		}
		drives = append(drives, candidate)
	}
	var drutilErr error
	if len(drives) == 0 {
		var drutilOutput []byte
		drutilOutput, drutilErr = runDarwinDrutil(context.Background(), "list")
		if drutilErr == nil {
			for _, drive := range parseDarwinDrutilList(string(drutilOutput)) {
				drives = append(drives, OpticalDrive{
					Path:        drutilDrivePath(drive.Index),
					DisplayName: strings.TrimSpace(strings.Join([]string{drive.Vendor, drive.Product}, " ")),
					Status:      fmt.Sprintf("drive present; media state unavailable (%s)", drive.SupportLevel),
				})
			}
		}
	}
	if len(drives) == 0 && drutilErr != nil {
		return nil, fmt.Errorf("discover macOS optical drives: diskutil found no usable media and drutil fallback failed: %w", drutilErr)
	}
	return drives, nil
}

func identifyHostOpticalMedia(path string) (OpticalMedia, error) {
	if index, ok := parseDrutilDrivePath(path); ok {
		return OpticalMedia{}, fmt.Errorf("inspect macOS optical media: drutil drive %d is present, but no mounted media geometry is available; insert a disc and retry", index)
	}
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
