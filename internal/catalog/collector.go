package catalog

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"
)

const (
	IdentityAlgorithmVersion uint16 = 1
	FingerprintSampleCount          = 8
	FingerprintTotalBudget          = 10 * time.Second
	FingerprintReadDeadline         = 3 * time.Second
)

// SampleSlot identifies one deterministic logical-sector sample.
type SampleSlot struct {
	Slot uint16
	LBA  int64
}

// BuildSamplePlan returns the bounded version-1 sample plan. Duplicate LBAs
// on very small media are removed while retaining the original slot numbers.
func BuildSamplePlan(sectorCount uint64) ([]SampleSlot, error) {
	if sectorCount == 0 {
		return nil, fmt.Errorf("build fingerprint sample plan: sector count must be greater than zero")
	}
	last := sectorCount - 1
	numerators := [...]uint64{0, 1, 2, 3, 4, 5, 6}
	plan := make([]SampleSlot, 0, FingerprintSampleCount)
	seen := make(map[int64]struct{}, FingerprintSampleCount)
	for slot, numerator := range numerators {
		lba := int64((last * numerator) / 8)
		if _, exists := seen[lba]; exists {
			continue
		}
		seen[lba] = struct{}{}
		plan = append(plan, SampleSlot{Slot: uint16(slot), LBA: lba})
	}
	if _, exists := seen[int64(last)]; !exists {
		plan = append(plan, SampleSlot{Slot: 7, LBA: int64(last)})
	}
	return plan, nil
}

// SectorReader is the bounded device-worker boundary used by identity
// collection. Implementations must honor ctx cancellation.
type SectorReader interface {
	ReadSector(ctx context.Context, lba int64, sectorSize uint32) ([]byte, error)
}

// FingerprintCollection reports the bounded work performed by a collector.
type FingerprintCollection struct {
	AttemptedSamples   uint16
	AvailableSamples   uint16
	UnavailableSamples uint16
	BytesRead          uint64
	Duration           time.Duration
}

// FingerprintCollector collects the version-1 distributed samples. Read
// failures become explicit unavailable samples so one damaged sector does not
// turn into a false mismatch or fabricate a hash of zero bytes.
type FingerprintCollector struct {
	TotalBudget  time.Duration
	ReadDeadline time.Duration
	Now          func() time.Time
}

func (c FingerprintCollector) normalized() FingerprintCollector {
	if c.TotalBudget <= 0 {
		c.TotalBudget = FingerprintTotalBudget
	}
	if c.ReadDeadline <= 0 {
		c.ReadDeadline = FingerprintReadDeadline
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	return c
}

// Collect appends planned sector fingerprints to base and computes QuickID
// only when every planned slot is readable.
func (c FingerprintCollector) Collect(ctx context.Context, base ContentIdentity, reader SectorReader) (ContentIdentity, FingerprintCollection, error) {
	if reader == nil {
		return ContentIdentity{}, FingerprintCollection{}, errors.New("collect fingerprint: sector reader is required")
	}
	c = c.normalized()
	if base.Version == 0 {
		base.Version = IdentityAlgorithmVersion
	}
	if base.Version != IdentityAlgorithmVersion {
		return ContentIdentity{}, FingerprintCollection{}, fmt.Errorf("collect fingerprint: unsupported identity version %d", base.Version)
	}
	plan, err := BuildSamplePlan(base.SectorCount)
	if err != nil {
		return ContentIdentity{}, FingerprintCollection{}, err
	}
	if base.LayoutSHA256 == "" {
		base.LayoutSHA256, err = BuildLayoutHash(base)
		if err != nil {
			return ContentIdentity{}, FingerprintCollection{}, err
		}
	}
	start := c.Now()
	deadline := start.Add(c.TotalBudget)
	base.Samples = make([]SectorFingerprint, 0, len(plan))
	stats := FingerprintCollection{}
	for _, planned := range plan {
		stats.AttemptedSamples++
		sample := SectorFingerprint{Slot: planned.Slot, LBA: planned.LBA}
		if c.Now().After(deadline) {
			sample.Error = SampleErrorSkipped
			stats.UnavailableSamples++
			base.Samples = append(base.Samples, sample)
			continue
		}
		remaining := deadline.Sub(c.Now())
		if remaining <= 0 {
			sample.Error = SampleErrorSkipped
			stats.UnavailableSamples++
			base.Samples = append(base.Samples, sample)
			continue
		}
		readDeadline := c.ReadDeadline
		if remaining < readDeadline {
			readDeadline = remaining
		}
		readCtx, cancel := context.WithTimeout(ctx, readDeadline)
		data, readErr := reader.ReadSector(readCtx, planned.LBA, base.LogicalBlockSize)
		cancel()
		switch {
		case readErr == nil && uint64(len(data)) != uint64(base.LogicalBlockSize):
			sample.Error = SampleErrorRead
		case readErr == nil:
			sample.Available = true
			sample.SHA256 = sha256.Sum256(data)
			stats.AvailableSamples++
			stats.BytesRead += uint64(len(data))
		case errors.Is(readErr, context.DeadlineExceeded):
			sample.Error = SampleErrorTimeout
		default:
			sample.Error = SampleErrorRead
		}
		if !sample.Available {
			stats.UnavailableSamples++
		}
		base.Samples = append(base.Samples, sample)
	}
	stats.Duration = c.Now().Sub(start)
	mandatory := make([]uint16, 0, len(plan))
	for _, planned := range plan {
		mandatory = append(mandatory, planned.Slot)
	}
	base.QuickID, _, err = BuildQuickContentID(base, mandatory)
	if err != nil {
		return ContentIdentity{}, stats, err
	}
	if err := base.Validate(); err != nil {
		return ContentIdentity{}, stats, err
	}
	return base, stats, nil
}

// CollectObservation packages collection output for media-inspection and
// catalog orchestration without conflating partial evidence with a mismatch.
func (c FingerprintCollector) CollectObservation(ctx context.Context, base ContentIdentity, reader SectorReader) (IdentityObservation, error) {
	identity, stats, err := c.Collect(ctx, base, reader)
	if err != nil {
		return IdentityObservation{Status: IdentityUnavailable, Detail: err.Error()}, err
	}
	status := IdentityInsufficientEvidence
	detail := "Not enough readable fingerprint samples are available for a reliable match."
	if identity.QuickID != "" {
		status = IdentityStrongEvidence
		detail = "All bounded fingerprint samples were collected."
	} else if stats.AvailableSamples > 0 {
		status = IdentityPartialEvidence
		detail = "Some fingerprint samples were unavailable; automatic matching is restricted."
	}
	return IdentityObservation{
		Identity:           identity,
		Status:             status,
		AttemptedSamples:   stats.AttemptedSamples,
		AvailableSamples:   stats.AvailableSamples,
		UnavailableSamples: stats.UnavailableSamples,
		BytesRead:          stats.BytesRead,
		CollectionDuration: stats.Duration,
		Detail:             detail,
	}, nil
}
