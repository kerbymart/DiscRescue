package linux

import (
	"fmt"
	"path/filepath"
	"regexp"

	"discrescue/internal/device"
)

var opticalDevicePathPattern = regexp.MustCompile(`^/dev/sr[0-9]+$`)

type OpenMode string

const (
	OpenModeReadOnly OpenMode = "read_only"
)

type BlockOpenOptions struct {
	Path                string
	Mode                OpenMode
	RequireOptical      bool
	RequireAdvisoryLock bool
}

type ProbeMetadata struct {
	DevicePath        string
	LogicalSectorSize uint32
	CapacitySectors   uint64
	Profile           string
}

type BlockAdapter struct {
	Options BlockOpenOptions
	Probe   ProbeMetadata
}

func CanonicalizeOpticalDevicePath(path string) (string, error) {
	cleaned := filepath.ToSlash(filepath.Clean(path))
	if !opticalDevicePathPattern.MatchString(cleaned) {
		return "", fmt.Errorf("canonicalize optical device path: %q is not a canonical /dev/srN path", cleaned)
	}
	return cleaned, nil
}

func (o BlockOpenOptions) Validate() error {
	if o.Mode != OpenModeReadOnly {
		return fmt.Errorf("validate block open options: only %q mode is allowed", OpenModeReadOnly)
	}
	if _, err := CanonicalizeOpticalDevicePath(o.Path); err != nil {
		return err
	}
	return nil
}

func (p ProbeMetadata) Validate() error {
	if _, err := CanonicalizeOpticalDevicePath(p.DevicePath); err != nil {
		return err
	}
	if p.LogicalSectorSize == 0 {
		return fmt.Errorf("validate probe metadata: logical sector size must be greater than zero")
	}
	if p.CapacitySectors == 0 {
		return fmt.Errorf("validate probe metadata: capacity sectors must be greater than zero")
	}
	if len(p.Profile) == 0 {
		return fmt.Errorf("validate probe metadata: profile is required")
	}
	if len(p.Profile) > 32 {
		return fmt.Errorf("validate probe metadata: profile length %d exceeds 32 bytes", len(p.Profile))
	}
	return nil
}

func (p ProbeMetadata) MediaInfo() device.MediaInfo {
	return device.MediaInfo{
		LogicalSectorSize: p.LogicalSectorSize,
		CapacitySectors:   p.CapacitySectors,
		Profile:           p.Profile,
	}
}

func BuildBlockAdapter(options BlockOpenOptions, probe ProbeMetadata) (BlockAdapter, error) {
	if err := options.Validate(); err != nil {
		return BlockAdapter{}, err
	}
	if err := probe.Validate(); err != nil {
		return BlockAdapter{}, err
	}
	if options.Path != probe.DevicePath {
		return BlockAdapter{}, fmt.Errorf("build block adapter: open path %q does not match probe path %q", options.Path, probe.DevicePath)
	}
	return BlockAdapter{
		Options: options,
		Probe:   probe,
	}, nil
}
