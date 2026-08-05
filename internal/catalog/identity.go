package catalog

import "fmt"

type MatchStrength string

const (
	MatchStrong        MatchStrength = "strong"
	MatchProbable      MatchStrength = "probable"
	MatchIndeterminate MatchStrength = "indeterminate"
	MatchConflict      MatchStrength = "conflict"
	MatchNo            MatchStrength = "none"
)

type TrackLayout struct {
	TrackNumber  uint16
	StartLBA     int64
	EndLBA       int64
	Mode         uint16
	ControlFlags uint16
	LeadOutLBA   int64
}

func (t TrackLayout) Validate() error {
	if t.TrackNumber == 0 {
		return fmt.Errorf("validate track layout: track number is required")
	}
	if t.EndLBA < t.StartLBA {
		return fmt.Errorf("validate track layout: end lba %d is smaller than start lba %d", t.EndLBA, t.StartLBA)
	}
	if t.LeadOutLBA < t.EndLBA {
		return fmt.Errorf("validate track layout: lead-out lba %d is smaller than end lba %d", t.LeadOutLBA, t.EndLBA)
	}
	return nil
}

type SampleErrorClass uint16

const (
	SampleErrorNone      SampleErrorClass = 0
	SampleErrorRead      SampleErrorClass = 1
	SampleErrorTimeout   SampleErrorClass = 2
	SampleErrorSkipped   SampleErrorClass = 3
	SampleErrorUnbounded SampleErrorClass = 4
)

type SectorFingerprint struct {
	Slot      uint16
	LBA       int64
	Available bool
	SHA256    [32]byte
	Error     SampleErrorClass
}

func (s SectorFingerprint) Validate() error {
	if s.Available && s.Error != SampleErrorNone {
		return fmt.Errorf("validate sector fingerprint: available sample slot %d cannot have error class %d", s.Slot, s.Error)
	}
	if !s.Available && s.Error == SampleErrorNone {
		return fmt.Errorf("validate sector fingerprint: unavailable sample slot %d must declare an error class", s.Slot)
	}
	if !s.Available && s.SHA256 != ([32]byte{}) {
		return fmt.Errorf("validate sector fingerprint: unavailable sample slot %d must not contain a hash", s.Slot)
	}
	return nil
}

type VolumeHint struct {
	HintType uint16
	Value    string
}

func (h VolumeHint) Validate() error {
	if h.Value == "" {
		return fmt.Errorf("validate volume hint: value is required")
	}
	return nil
}

type ContentIdentity struct {
	Version           uint16
	Profile           uint16
	LogicalBlockSize  uint32
	SectorCount       uint64
	Sessions          uint16
	Tracks            []TrackLayout
	LayoutSHA256      string
	VolumeHints       []VolumeHint
	Samples           []SectorFingerprint
	QuickID           string
	FullContentSHA256 string
}

func (id ContentIdentity) Validate() error {
	if id.Version == 0 {
		return fmt.Errorf("validate content identity: version is required")
	}
	if id.LogicalBlockSize == 0 {
		return fmt.Errorf("validate content identity: logical block size must be greater than zero")
	}
	if id.SectorCount == 0 {
		return fmt.Errorf("validate content identity: sector count must be greater than zero")
	}
	if id.LayoutSHA256 == "" {
		return fmt.Errorf("validate content identity: layout hash is required")
	}
	seenSlots := make(map[uint16]struct{}, len(id.Samples))
	for i, track := range id.Tracks {
		if err := track.Validate(); err != nil {
			return fmt.Errorf("validate content identity track[%d]: %w", i, err)
		}
		if i > 0 && id.Tracks[i-1].EndLBA >= track.StartLBA {
			return fmt.Errorf(
				"validate content identity track[%d]: overlapping track ranges %d-%d and %d-%d",
				i,
				id.Tracks[i-1].StartLBA,
				id.Tracks[i-1].EndLBA,
				track.StartLBA,
				track.EndLBA,
			)
		}
	}
	for i, sample := range id.Samples {
		if err := sample.Validate(); err != nil {
			return fmt.Errorf("validate content identity sample[%d]: %w", i, err)
		}
		if _, exists := seenSlots[sample.Slot]; exists {
			return fmt.Errorf("validate content identity sample[%d]: duplicate slot %d", i, sample.Slot)
		}
		seenSlots[sample.Slot] = struct{}{}
	}
	for i, hint := range id.VolumeHints {
		if err := hint.Validate(); err != nil {
			return fmt.Errorf("validate content identity volume hint[%d]: %w", i, err)
		}
	}
	return nil
}

type ProcessingState string

const (
	ProcessingObserved          ProcessingState = "observed"
	ProcessingInProgress        ProcessingState = "in_progress"
	ProcessingStoppedResumable  ProcessingState = "stopped_resumable"
	ProcessingCompletedVerified ProcessingState = "completed_verified"
	ProcessingCompletedWithGaps ProcessingState = "completed_with_gaps"
	ProcessingFailed            ProcessingState = "failed"
	ProcessingMerged            ProcessingState = "merged"
)
