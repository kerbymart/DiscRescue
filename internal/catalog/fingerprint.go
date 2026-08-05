package catalog

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sort"
)

const MinimumStrongSamples uint16 = 4

type CompareResult struct {
	Match              MatchStrength
	MatchingSamples    uint16
	ConflictingSamples uint16
	ComparedSamples    uint16
	BudgetExhausted    bool
}

func BuildLayoutHash(identity ContentIdentity) (string, error) {
	if identity.Version == 0 {
		return "", fmt.Errorf("build layout hash: identity version is required")
	}
	if identity.LogicalBlockSize == 0 {
		return "", fmt.Errorf("build layout hash: logical block size must be greater than zero")
	}
	if identity.SectorCount == 0 {
		return "", fmt.Errorf("build layout hash: sector count must be greater than zero")
	}
	for i, track := range identity.Tracks {
		if err := track.Validate(); err != nil {
			return "", fmt.Errorf("build layout hash track[%d]: %w", i, err)
		}
	}
	for i, hint := range identity.VolumeHints {
		if err := hint.Validate(); err != nil {
			return "", fmt.Errorf("build layout hash volume hint[%d]: %w", i, err)
		}
	}

	hasher := sha256.New()
	var scratch [8]byte

	binary.LittleEndian.PutUint16(scratch[:2], identity.Version)
	hasher.Write(scratch[:2])
	binary.LittleEndian.PutUint16(scratch[:2], identity.Profile)
	hasher.Write(scratch[:2])
	binary.LittleEndian.PutUint32(scratch[:4], identity.LogicalBlockSize)
	hasher.Write(scratch[:4])
	binary.LittleEndian.PutUint64(scratch[:8], identity.SectorCount)
	hasher.Write(scratch[:8])
	binary.LittleEndian.PutUint16(scratch[:2], identity.Sessions)
	hasher.Write(scratch[:2])

	for _, track := range identity.Tracks {
		binary.LittleEndian.PutUint16(scratch[:2], track.TrackNumber)
		hasher.Write(scratch[:2])
		binary.LittleEndian.PutUint64(scratch[:8], uint64(track.StartLBA))
		hasher.Write(scratch[:8])
		binary.LittleEndian.PutUint64(scratch[:8], uint64(track.EndLBA))
		hasher.Write(scratch[:8])
		binary.LittleEndian.PutUint16(scratch[:2], track.Mode)
		hasher.Write(scratch[:2])
		binary.LittleEndian.PutUint16(scratch[:2], track.ControlFlags)
		hasher.Write(scratch[:2])
		binary.LittleEndian.PutUint64(scratch[:8], uint64(track.LeadOutLBA))
		hasher.Write(scratch[:8])
	}

	hints := append([]VolumeHint(nil), identity.VolumeHints...)
	sort.Slice(hints, func(i, j int) bool {
		if hints[i].HintType != hints[j].HintType {
			return hints[i].HintType < hints[j].HintType
		}
		return hints[i].Value < hints[j].Value
	})
	for _, hint := range hints {
		binary.LittleEndian.PutUint16(scratch[:2], hint.HintType)
		hasher.Write(scratch[:2])
		binary.LittleEndian.PutUint16(scratch[:2], uint16(len(hint.Value)))
		hasher.Write(scratch[:2])
		hasher.Write([]byte(hint.Value))
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func BuildQuickContentID(identity ContentIdentity, mandatorySlots []uint16) (string, bool, error) {
	if err := identity.Validate(); err != nil {
		return "", false, err
	}
	if len(mandatorySlots) == 0 {
		return "", false, fmt.Errorf("build quick content id: at least one mandatory slot is required")
	}

	indexed := make(map[uint16]SectorFingerprint, len(identity.Samples))
	for _, sample := range identity.Samples {
		indexed[sample.Slot] = sample
	}

	hasher := sha256.New()
	var scratch [8]byte
	binary.LittleEndian.PutUint16(scratch[:2], identity.Version)
	hasher.Write(scratch[:2])
	hasher.Write([]byte(identity.LayoutSHA256))

	slots := append([]uint16(nil), mandatorySlots...)
	sort.Slice(slots, func(i, j int) bool { return slots[i] < slots[j] })
	for _, slot := range slots {
		sample, ok := indexed[slot]
		if !ok || !sample.Available {
			return "", false, nil
		}
		binary.LittleEndian.PutUint16(scratch[:2], sample.Slot)
		hasher.Write(scratch[:2])
		binary.LittleEndian.PutUint64(scratch[:8], uint64(sample.LBA))
		hasher.Write(scratch[:8])
		hasher.Write(sample.SHA256[:])
	}

	return hex.EncodeToString(hasher.Sum(nil)), true, nil
}

func CompareContentIdentity(candidate, observed ContentIdentity, remainingBudget uint16) (CompareResult, error) {
	if err := candidate.Validate(); err != nil {
		return CompareResult{}, fmt.Errorf("compare content identity candidate: %w", err)
	}
	if err := observed.Validate(); err != nil {
		return CompareResult{}, fmt.Errorf("compare content identity observed: %w", err)
	}
	if remainingBudget == 0 {
		return CompareResult{}, fmt.Errorf("compare content identity: remaining budget must be greater than zero")
	}

	if !CompatibleGeometry(candidate, observed) || candidate.LayoutSHA256 != observed.LayoutSHA256 {
		return CompareResult{Match: MatchNo}, nil
	}
	if candidate.QuickID != "" && candidate.QuickID == observed.QuickID {
		return CompareResult{Match: MatchStrong}, nil
	}

	indexed := make(map[uint16]SectorFingerprint, len(candidate.Samples))
	for _, sample := range candidate.Samples {
		indexed[sample.Slot] = sample
	}

	var result CompareResult
	for _, sample := range observed.Samples {
		candidateSample, ok := indexed[sample.Slot]
		if !ok || !sample.Available || !candidateSample.Available {
			continue
		}
		if result.ComparedSamples >= remainingBudget {
			result.Match = MatchIndeterminate
			result.BudgetExhausted = true
			return result, nil
		}
		result.ComparedSamples++
		if candidateSample.LBA != sample.LBA || candidateSample.SHA256 != sample.SHA256 {
			result.ConflictingSamples++
			result.Match = MatchConflict
			return result, nil
		}
		result.MatchingSamples++
	}

	switch {
	case result.ConflictingSamples > 0:
		result.Match = MatchConflict
	case result.MatchingSamples >= MinimumStrongSamples:
		result.Match = MatchStrong
	case result.MatchingSamples >= 1:
		result.Match = MatchProbable
	default:
		result.Match = MatchIndeterminate
	}
	return result, nil
}

func CompatibleGeometry(left, right ContentIdentity) bool {
	return left.Version == right.Version &&
		left.Profile == right.Profile &&
		left.LogicalBlockSize == right.LogicalBlockSize &&
		left.SectorCount == right.SectorCount &&
		left.Sessions == right.Sessions
}
