package recovery

import (
	"crypto/sha256"
	"testing"

	"discrescue/internal/catalog"
)

func TestPreparePass0BuildsStrongLookupForMatchingIdentity(t *testing.T) {
	geometry := dataGeometry()
	planned := PlanIdentitySamples(geometry)
	samples := availableSamples(planned)
	identity, err := BuildContentIdentity(geometry, samples, planned)
	if err != nil {
		t.Fatalf("build content identity: %v", err)
	}

	plan, err := PreparePass0(Pass0Input{
		Geometry:               geometry,
		ObservedSamples:        samples,
		CatalogEntries:         []catalog.Entry{{Identity: identity, Status: string(catalog.ProcessingObserved)}},
		LookupBudget:           catalog.LookupBudget{MaxComparedSamples: 8},
		DevicePreferredCluster: 24,
	})
	if err != nil {
		t.Fatalf("prepare pass0: %v", err)
	}
	if plan.Representation != OutputISO {
		t.Fatalf("unexpected representation: %s", plan.Representation)
	}
	if plan.Lookup.Match != catalog.MatchStrong {
		t.Fatalf("unexpected lookup match: %s", plan.Lookup.Match)
	}
	if plan.InitialCluster != 24 {
		t.Fatalf("unexpected initial cluster size: %d", plan.InitialCluster)
	}
	if plan.FirstRead.Strategy != StrategyFast || plan.FirstRead.Sectors != 24 {
		t.Fatalf("unexpected initial request: %+v", plan.FirstRead)
	}
	if plan.Identity.QuickID == "" {
		t.Fatal("expected quick id when all mandatory samples are available")
	}
}

func TestPreparePass0LeavesQuickIDEmptyWhenMandatorySampleMissing(t *testing.T) {
	geometry := dataGeometry()
	planned := PlanIdentitySamples(geometry)
	samples := availableSamples(planned[:len(planned)-1])

	identity, err := BuildContentIdentity(geometry, samples, planned)
	if err != nil {
		t.Fatalf("build content identity: %v", err)
	}
	if identity.QuickID != "" {
		t.Fatalf("expected quick id to be absent, got %q", identity.QuickID)
	}
}

func TestPreparePass0ChoosesBinCueForMultiTrackLayout(t *testing.T) {
	geometry := Geometry{
		Profile:          0x08,
		LogicalBlockSize: 2352,
		SectorCount:      12000,
		Sessions:         1,
		Tracks: []catalog.TrackLayout{
			{TrackNumber: 1, StartLBA: 0, EndLBA: 5999, Mode: TrackModeAudio, LeadOutLBA: 6000},
			{TrackNumber: 2, StartLBA: 6000, EndLBA: 11999, Mode: TrackModeAudio, LeadOutLBA: 12000},
		},
	}

	if got := SelectOutputRepresentation(geometry); got != OutputBINCUE {
		t.Fatalf("unexpected representation: %s", got)
	}
}

func TestPlanIdentitySamplesProducesUniqueDeterministicSlots(t *testing.T) {
	geometry := dataGeometry()

	left := PlanIdentitySamples(geometry)
	right := PlanIdentitySamples(geometry)
	if len(left) == 0 {
		t.Fatal("expected sample plan entries")
	}
	if len(left) != len(right) {
		t.Fatalf("expected deterministic sample count, got %d and %d", len(left), len(right))
	}
	for i := range left {
		if left[i] != right[i] {
			t.Fatalf("sample plan changed at index %d: %+v != %+v", i, left[i], right[i])
		}
		for j := i + 1; j < len(left); j++ {
			if left[i].LBA == left[j].LBA {
				t.Fatalf("duplicate sample lba %d at indexes %d and %d", left[i].LBA, i, j)
			}
		}
	}
}

func TestPreparePass0UsesBoundedLookupResult(t *testing.T) {
	geometry := dataGeometry()
	planned := PlanIdentitySamples(geometry)
	samples := availableSamples(planned)
	identity, err := BuildContentIdentity(geometry, samples, planned)
	if err != nil {
		t.Fatalf("build content identity: %v", err)
	}
	identity.QuickID = ""

	plan, err := PreparePass0(Pass0Input{
		Geometry:        geometry,
		ObservedSamples: samples,
		CatalogEntries:  []catalog.Entry{{Identity: identity, Status: string(catalog.ProcessingObserved)}},
		LookupBudget:    catalog.LookupBudget{MaxComparedSamples: 2},
	})
	if err != nil {
		t.Fatalf("prepare pass0: %v", err)
	}
	if plan.Lookup.Match != catalog.MatchIndeterminate {
		t.Fatalf("unexpected lookup match: %s", plan.Lookup.Match)
	}
	if !plan.Lookup.BudgetExhausted {
		t.Fatal("expected bounded lookup to report budget exhaustion")
	}
}

func TestPreparePass0RejectsInvalidGeometry(t *testing.T) {
	_, err := PreparePass0(Pass0Input{
		Geometry: Geometry{
			Profile:          0x10,
			LogicalBlockSize: 0,
			SectorCount:      4096,
			Sessions:         1,
			Tracks: []catalog.TrackLayout{
				{TrackNumber: 1, StartLBA: 0, EndLBA: 4095, Mode: TrackModeData2048, LeadOutLBA: 4096},
			},
		},
		LookupBudget: catalog.LookupBudget{MaxComparedSamples: 4},
	})
	if err == nil {
		t.Fatal("expected invalid geometry to be rejected")
	}
}

func dataGeometry() Geometry {
	return Geometry{
		Profile:          0x10,
		LogicalBlockSize: 2048,
		SectorCount:      4096,
		Sessions:         1,
		Tracks: []catalog.TrackLayout{
			{TrackNumber: 1, StartLBA: 0, EndLBA: 4095, Mode: TrackModeData2048, LeadOutLBA: 4096},
		},
	}
}

func availableSamples(plan []SamplePlanEntry) []catalog.SectorFingerprint {
	samples := make([]catalog.SectorFingerprint, 0, len(plan))
	for _, entry := range plan {
		samples = append(samples, catalog.SectorFingerprint{
			Slot:      entry.Slot,
			LBA:       entry.LBA,
			Available: true,
			SHA256:    sha256.Sum256([]byte(entry.Reason)),
		})
	}
	return samples
}
