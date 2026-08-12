package testdevice

import "fmt"

type DelaySpec struct {
	Range    Range
	Millis   uint32
	Selector AttemptSelector
}

func (d DelaySpec) Validate() error {
	if err := d.Range.Validate(); err != nil {
		return fmt.Errorf("validate delay spec: %w", err)
	}
	return d.Selector.Validate()
}

type FailureMode string

const (
	FailureReadError        FailureMode = "read_error"
	FailurePartialTransfer  FailureMode = "partial_transfer"
	FailureCorruptBytes     FailureMode = "corrupt_bytes"
	FailureConflictCapacity FailureMode = "conflict_capacity"
	FailureHangWorker       FailureMode = "hang_worker"
	FailureCrashWorker      FailureMode = "crash_worker"
	FailureMediaReplacement FailureMode = "media_replacement"
)

type FailureSpec struct {
	Range              Range
	Mode               FailureMode
	Selector           AttemptSelector
	TransferredSectors uint32
	Sense              SenseTuple
	CorruptOffsets     []uint32
	ConflictingSectors uint64
	ReplacementMediaID string
}

func (f FailureSpec) Validate() error {
	if err := f.Range.Validate(); err != nil {
		return fmt.Errorf("validate failure spec: %w", err)
	}
	if err := f.Selector.Validate(); err != nil {
		return fmt.Errorf("validate failure spec: %w", err)
	}
	if f.Mode == "" {
		return fmt.Errorf("validate failure spec: mode is required")
	}
	if f.TransferredSectors > uint32(f.Range.Sectors) {
		return fmt.Errorf("validate failure spec: transferred sectors %d exceed range sectors %d", f.TransferredSectors, f.Range.Sectors)
	}
	if f.Mode == FailureConflictCapacity && f.ConflictingSectors == 0 {
		return fmt.Errorf("validate failure spec: conflicting capacity must be greater than zero")
	}
	if f.Mode == FailureMediaReplacement && f.ReplacementMediaID == "" {
		return fmt.Errorf("validate failure spec: replacement media id is required")
	}
	return nil
}

type SectorPattern string

const (
	PatternZero  SectorPattern = "zero"
	PatternLBA   SectorPattern = "lba"
	PatternBytes SectorPattern = "bytes"
)

type DataSpec struct {
	Range   Range
	Pattern SectorPattern
	Bytes   []byte
}

func (d DataSpec) Validate(sectorSize uint32) error {
	if err := d.Range.Validate(); err != nil {
		return fmt.Errorf("validate data spec: %w", err)
	}
	if d.Pattern == "" {
		return fmt.Errorf("validate data spec: pattern is required")
	}
	if d.Pattern == PatternBytes && uint32(len(d.Bytes)) != sectorSize {
		return fmt.Errorf("validate data spec: byte pattern length %d does not match sector size %d", len(d.Bytes), sectorSize)
	}
	return nil
}

type Media struct {
	MediaID           string
	Profile           uint16
	LogicalSectorSize uint32
	SectorCount       uint64
	Sessions          uint16
	Tracks            uint16
	Data              []DataSpec
}

func (m Media) Validate() error {
	if m.MediaID == "" {
		return fmt.Errorf("validate media: media id is required")
	}
	if m.LogicalSectorSize == 0 {
		return fmt.Errorf("validate media: logical sector size must be greater than zero")
	}
	if m.SectorCount == 0 {
		return fmt.Errorf("validate media: sector count must be greater than zero")
	}
	for i, spec := range m.Data {
		if err := spec.Validate(m.LogicalSectorSize); err != nil {
			return fmt.Errorf("validate media data[%d]: %w", i, err)
		}
	}
	return nil
}

type Scenario struct {
	Name     string
	Media    Media
	Delays   []DelaySpec
	Failures []FailureSpec
}

func (s Scenario) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("validate scenario: name is required")
	}
	if err := s.Media.Validate(); err != nil {
		return fmt.Errorf("validate scenario: %w", err)
	}
	for i, delay := range s.Delays {
		if err := delay.Validate(); err != nil {
			return fmt.Errorf("validate scenario delay[%d]: %w", i, err)
		}
	}
	for i, failure := range s.Failures {
		if err := failure.Validate(); err != nil {
			return fmt.Errorf("validate scenario failure[%d]: %w", i, err)
		}
	}
	if err := validateNonOverlappingData(s.Media.Data); err != nil {
		return fmt.Errorf("validate scenario: %w", err)
	}
	return nil
}

func validateNonOverlappingData(specs []DataSpec) error {
	for i := 0; i < len(specs); i++ {
		for j := i + 1; j < len(specs); j++ {
			if Overlap(specs[i].Range, specs[j].Range) {
				return fmt.Errorf("validate data specs: overlapping ranges %d and %d", i, j)
			}
		}
	}
	return nil
}
