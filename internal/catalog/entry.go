package catalog

import "fmt"

type RecordID [16]byte

type JobReference struct {
	JobID        RecordID
	Path         string
	FilesPresent bool
}

func (r JobReference) Validate() error {
	if r.JobID == (RecordID{}) {
		return fmt.Errorf("validate job reference: job id is required")
	}
	if r.Path == "" {
		return fmt.Errorf("validate job reference: path is required")
	}
	return nil
}

type Entry struct {
	RecordID              RecordID
	Identity              ContentIdentity
	State                 ProcessingState
	Status                string
	FirstSeenUnixNano     int64
	LastSeenUnixNano      int64
	LastProcessedPresent  bool
	LastProcessedUnixNano int64
	Captures              []CaptureIdentity
	JobReferences         []JobReference
	PreferredJobID        RecordID
	Hidden                bool
}

func (e Entry) Validate() error {
	if e.RecordID == (RecordID{}) {
		return fmt.Errorf("validate entry: record id is required")
	}
	if err := e.Identity.Validate(); err != nil {
		return fmt.Errorf("validate entry identity: %w", err)
	}
	if !validProcessingState(e.State) {
		return fmt.Errorf("validate entry: unsupported processing state %q", e.State)
	}
	if e.LastSeenUnixNano < e.FirstSeenUnixNano {
		return fmt.Errorf("validate entry: last seen time %d is earlier than first seen time %d", e.LastSeenUnixNano, e.FirstSeenUnixNano)
	}
	if !e.LastProcessedPresent && e.LastProcessedUnixNano != 0 {
		return fmt.Errorf("validate entry: last processed timestamp must be zero when absent")
	}
	for i, capture := range e.Captures {
		if err := capture.Validate(); err != nil {
			return fmt.Errorf("validate entry capture[%d]: %w", i, err)
		}
	}
	for i, reference := range e.JobReferences {
		if err := reference.Validate(); err != nil {
			return fmt.Errorf("validate entry job reference[%d]: %w", i, err)
		}
	}
	return nil
}

type Bounds struct {
	MaxEntries          int
	MaxCapturesPerEntry int
	MaxJobRefsPerEntry  int
}

func (b Bounds) Validate() error {
	if b.MaxEntries < 0 || b.MaxCapturesPerEntry < 0 || b.MaxJobRefsPerEntry < 0 {
		return fmt.Errorf("validate bounds: limits must be non-negative")
	}
	return nil
}
