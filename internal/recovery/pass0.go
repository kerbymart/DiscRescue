package recovery

import (
	"encoding/binary"
	"encoding/hex"
	"sort"

	"discrescue/internal/catalog"
)

const (
	TrackModeAudio    uint16 = 0
	TrackModeData2048 uint16 = 1
)

type OutputRepresentation string

const (
	OutputISO    OutputRepresentation = "iso"
	OutputBINCUE OutputRepresentation = "bincue"
	OutputIMG    OutputRepresentation = "img"
)

type Geometry struct {
	Profile          uint16
	LogicalBlockSize uint32
	SectorCount      uint64
	Sessions         uint16
	Tracks           []catalog.TrackLayout
}

func (g Geometry) Validate() error {
	identity := catalog.ContentIdentity{
		Version:          1,
		Profile:          g.Profile,
		LogicalBlockSize: g.LogicalBlockSize,
		SectorCount:      g.SectorCount,
		Sessions:         g.Sessions,
		Tracks:           g.Tracks,
		LayoutSHA256:     "geometry-validation",
	}
	return identity.Validate()
}

type SamplePlanEntry struct {
	Slot      uint16
	LBA       int64
	Mandatory bool
	Reason    string
}

type Pass0Input struct {
	Geometry               Geometry
	ObservedSamples        []catalog.SectorFingerprint
	CatalogEntries         []catalog.Entry
	LookupBudget           catalog.LookupBudget
	DevicePreferredCluster uint32
}

type Pass0Plan struct {
	Representation OutputRepresentation
	Identity       catalog.ContentIdentity
	SamplePlan     []SamplePlanEntry
	Lookup         catalog.LookupResult
	InitialCluster uint32
	FirstRead      Request
}

func PreparePass0(input Pass0Input) (Pass0Plan, error) {
	if err := input.Geometry.Validate(); err != nil {
		return Pass0Plan{}, err
	}
	if err := input.LookupBudget.Validate(); err != nil {
		return Pass0Plan{}, err
	}

	representation := SelectOutputRepresentation(input.Geometry)
	samplePlan := PlanIdentitySamples(input.Geometry)
	identity, err := BuildContentIdentity(input.Geometry, input.ObservedSamples, samplePlan)
	if err != nil {
		return Pass0Plan{}, err
	}
	lookup, err := catalog.Lookup(input.CatalogEntries, identity, input.LookupBudget)
	if err != nil {
		return Pass0Plan{}, err
	}

	cluster := selectInitialClusterSize(input.Geometry, input.DevicePreferredCluster)
	if cluster > uint32(input.Geometry.SectorCount) {
		cluster = uint32(input.Geometry.SectorCount)
	}

	return Pass0Plan{
		Representation: representation,
		Identity:       identity,
		SamplePlan:     samplePlan,
		Lookup:         lookup,
		InitialCluster: cluster,
		FirstRead:      FastPass(0, cluster),
	}, nil
}

func SelectOutputRepresentation(geometry Geometry) OutputRepresentation {
	if len(geometry.Tracks) > 1 {
		return OutputBINCUE
	}
	if len(geometry.Tracks) == 1 && geometry.Tracks[0].Mode == TrackModeAudio {
		return OutputBINCUE
	}
	if geometry.LogicalBlockSize == 2048 {
		return OutputISO
	}
	return OutputIMG
}

func PlanIdentitySamples(geometry Geometry) []SamplePlanEntry {
	if geometry.SectorCount == 0 {
		return nil
	}

	positions := []struct {
		slot   uint16
		lba    int64
		reason string
	}{
		{slot: 1, lba: 0, reason: "first-readable"},
		{slot: 2, lba: clampLBA(16, geometry.SectorCount), reason: "descriptor-region"},
		{slot: 3, lba: fractionalLBA(geometry.SectorCount, 1, 8), reason: "fraction-12_5"},
		{slot: 4, lba: fractionalLBA(geometry.SectorCount, 1, 4), reason: "fraction-25"},
		{slot: 5, lba: fractionalLBA(geometry.SectorCount, 1, 2), reason: "fraction-50"},
		{slot: 6, lba: fractionalLBA(geometry.SectorCount, 3, 4), reason: "fraction-75"},
		{slot: 7, lba: fractionalLBA(geometry.SectorCount, 7, 8), reason: "fraction-87_5"},
		{slot: 8, lba: int64(geometry.SectorCount - 1), reason: "final-readable"},
	}

	extraSlotLBA := deterministicExtraLBA(geometry)
	positions = append(positions, struct {
		slot   uint16
		lba    int64
		reason string
	}{slot: 9, lba: extraSlotLBA, reason: "layout-derived"})

	seen := make(map[int64]struct{}, len(positions))
	planned := make([]SamplePlanEntry, 0, len(positions))
	for _, position := range positions {
		if _, ok := seen[position.lba]; ok {
			continue
		}
		seen[position.lba] = struct{}{}
		planned = append(planned, SamplePlanEntry{
			Slot:      position.slot,
			LBA:       position.lba,
			Mandatory: true,
			Reason:    position.reason,
		})
	}
	sort.Slice(planned, func(i, j int) bool {
		return planned[i].Slot < planned[j].Slot
	})
	return planned
}

func BuildContentIdentity(geometry Geometry, observed []catalog.SectorFingerprint, plan []SamplePlanEntry) (catalog.ContentIdentity, error) {
	samples := append([]catalog.SectorFingerprint(nil), observed...)
	sort.Slice(samples, func(i, j int) bool {
		return samples[i].Slot < samples[j].Slot
	})

	layoutHash, err := buildLayoutHash(geometry)
	if err != nil {
		return catalog.ContentIdentity{}, err
	}

	identity := catalog.ContentIdentity{
		Version:          1,
		Profile:          geometry.Profile,
		LogicalBlockSize: geometry.LogicalBlockSize,
		SectorCount:      geometry.SectorCount,
		Sessions:         geometry.Sessions,
		Tracks:           append([]catalog.TrackLayout(nil), geometry.Tracks...),
		LayoutSHA256:     layoutHash,
		Samples:          samples,
	}

	if quickID, ok, err := buildQuickID(identity, plan); err != nil {
		return catalog.ContentIdentity{}, err
	} else if ok {
		identity.QuickID = quickID
	}

	return identity, identity.Validate()
}

func buildLayoutHash(geometry Geometry) (string, error) {
	return catalog.BuildLayoutHash(catalog.ContentIdentity{
		Version:          1,
		Profile:          geometry.Profile,
		LogicalBlockSize: geometry.LogicalBlockSize,
		SectorCount:      geometry.SectorCount,
		Sessions:         geometry.Sessions,
		Tracks:           append([]catalog.TrackLayout(nil), geometry.Tracks...),
	})
}

func buildQuickID(identity catalog.ContentIdentity, plan []SamplePlanEntry) (string, bool, error) {
	slots := make([]uint16, 0, len(plan))
	for _, expected := range plan {
		if !expected.Mandatory {
			continue
		}
		slots = append(slots, expected.Slot)
	}
	return catalog.BuildQuickContentID(identity, slots)
}

func selectInitialClusterSize(geometry Geometry, preferred uint32) uint32 {
	if preferred > 0 {
		return preferred
	}
	if geometry.LogicalBlockSize == 2048 {
		return 32
	}
	return 4
}

func deterministicExtraLBA(geometry Geometry) int64 {
	hash, err := buildLayoutHash(geometry)
	if err != nil || geometry.SectorCount == 0 {
		return 0
	}
	decoded, err := hex.DecodeString(hash[:16])
	if err != nil || len(decoded) < 8 {
		return 0
	}
	value := binary.LittleEndian.Uint64(decoded[:8])
	return int64(value % geometry.SectorCount)
}

func fractionalLBA(sectorCount uint64, numerator uint64, denominator uint64) int64 {
	if sectorCount == 0 {
		return 0
	}
	return int64(((sectorCount - 1) * numerator) / denominator)
}

func clampLBA(lba uint64, sectorCount uint64) int64 {
	if sectorCount == 0 {
		return 0
	}
	if lba >= sectorCount {
		return int64(sectorCount - 1)
	}
	return int64(lba)
}
