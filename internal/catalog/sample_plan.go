package catalog

import "fmt"

type SampleSlot struct {
	Slot uint16
	LBA  int64
}

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
