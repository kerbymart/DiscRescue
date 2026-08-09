package catalog

import (
	"crypto/rand"
	"fmt"
	"reflect"
	"time"
)

// NewRecordID returns a cryptographically random local catalog/job identifier.
func NewRecordID() (RecordID, error) {
	var id RecordID
	if _, err := rand.Read(id[:]); err != nil {
		return RecordID{}, fmt.Errorf("generate catalog record id: %w", err)
	}
	return id, nil
}

type DeviceIdentity struct {
	Vendor    string
	Product   string
	Revision  string
	Serial    string
	Transport string
}

func (d DeviceIdentity) Validate() error {
	if d.Vendor == "" && d.Product == "" && d.Serial == "" && d.Transport == "" {
		return fmt.Errorf("validate device identity: at least one identifying field is required")
	}
	return nil
}

type PhysicalCopyIdentity struct {
	AssetID     string
	HubCodeNote string
}

func (p PhysicalCopyIdentity) Validate() error {
	if p.AssetID == "" && p.HubCodeNote == "" {
		return fmt.Errorf("validate physical copy identity: asset id or hub code note is required")
	}
	return nil
}

type CaptureIdentity struct {
	CaptureID    string
	Device       DeviceIdentity
	StartedAt    time.Time
	UserLabel    string
	PhysicalCopy *PhysicalCopyIdentity
}

func (c CaptureIdentity) Validate() error {
	if c.CaptureID == "" {
		return fmt.Errorf("validate capture identity: capture id is required")
	}
	if c.StartedAt.IsZero() {
		return fmt.Errorf("validate capture identity: started-at time is required")
	}
	if err := c.Device.Validate(); err != nil {
		return fmt.Errorf("validate capture identity device: %w", err)
	}
	if c.PhysicalCopy != nil {
		if err := c.PhysicalCopy.Validate(); err != nil {
			return fmt.Errorf("validate capture identity physical copy: %w", err)
		}
	}
	return nil
}

type ContentMatchKey struct {
	Version           uint16
	Profile           uint16
	LogicalBlockSize  uint32
	SectorCount       uint64
	Sessions          uint16
	Tracks            []TrackLayout
	VolumeHints       []VolumeHint
	LayoutSHA256      string
	Samples           []SectorFingerprint
	QuickID           string
	FullContentSHA256 string
}

func (id ContentIdentity) MatchKey() ContentMatchKey {
	return ContentMatchKey{
		Version:           id.Version,
		Profile:           id.Profile,
		LogicalBlockSize:  id.LogicalBlockSize,
		SectorCount:       id.SectorCount,
		Sessions:          id.Sessions,
		Tracks:            append([]TrackLayout(nil), id.Tracks...),
		VolumeHints:       append([]VolumeHint(nil), id.VolumeHints...),
		LayoutSHA256:      id.LayoutSHA256,
		Samples:           append([]SectorFingerprint(nil), id.Samples...),
		QuickID:           id.QuickID,
		FullContentSHA256: id.FullContentSHA256,
	}
}

type IdentityRecord struct {
	Content ContentIdentity
	Capture CaptureIdentity
}

func (r IdentityRecord) Validate() error {
	if err := r.Content.Validate(); err != nil {
		return fmt.Errorf("validate identity record content: %w", err)
	}
	if err := r.Capture.Validate(); err != nil {
		return fmt.Errorf("validate identity record capture: %w", err)
	}
	return nil
}

func (r IdentityRecord) SameLogicalContent(other IdentityRecord) bool {
	return reflect.DeepEqual(r.Content.MatchKey(), other.Content.MatchKey())
}
