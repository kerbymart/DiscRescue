package catalog

import "testing"

func TestLookupSkipsHiddenEntries(t *testing.T) {
	identity := identityWithSamples("layout-hidden", []SectorFingerprint{
		availableSample(1, 0, "same-1"),
		availableSample(2, 16, "same-2"),
		availableSample(3, 64, "same-3"),
		availableSample(4, 128, "same-4"),
	})
	hiddenEntry := testCatalogEntry(1, 100, 200)
	hiddenEntry.Identity = identity
	hiddenEntry.Hidden = true

	result, err := Lookup([]Entry{hiddenEntry}, identity, LookupBudget{MaxComparedSamples: 4})
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if result.Match != MatchNo {
		t.Fatalf("expected hidden record to be excluded from lookup, got %q", result.Match)
	}
}

func TestHideRecordMarksRecordHidden(t *testing.T) {
	store := Store{
		Entries: []Entry{
			testCatalogEntry(1, 100, 200),
		},
	}

	updated, err := store.HideRecord(store.Entries[0].RecordID)
	if err != nil {
		t.Fatalf("hide record: %v", err)
	}
	if !updated.Entries[0].Hidden {
		t.Fatal("expected record to be hidden")
	}
}

func TestForgetMissingPathReferencesRemovesMissingJobsAndClearsPreferredJob(t *testing.T) {
	entry := testCatalogEntry(1, 100, 200)
	remainingJobID := RecordID{9}
	entry.PreferredJobID = entry.JobReferences[0].JobID
	entry.JobReferences = []JobReference{
		{JobID: entry.PreferredJobID, Path: "D:/archive/missing.iso", FilesPresent: false},
		{JobID: remainingJobID, Path: "D:/archive/present.iso", FilesPresent: true},
	}

	store := Store{Entries: []Entry{entry}}
	updated, err := store.ForgetMissingPathReferences(entry.RecordID)
	if err != nil {
		t.Fatalf("forget missing path references: %v", err)
	}

	if len(updated.Entries[0].JobReferences) != 1 {
		t.Fatalf("expected one present job reference to remain, got %d", len(updated.Entries[0].JobReferences))
	}
	if updated.Entries[0].JobReferences[0].JobID != remainingJobID {
		t.Fatalf("unexpected remaining job reference: %#v", updated.Entries[0].JobReferences[0].JobID)
	}
	if updated.Entries[0].PreferredJobID != (RecordID{}) {
		t.Fatalf("expected preferred job id to clear when the preferred path is removed, got %#v", updated.Entries[0].PreferredJobID)
	}
}

func TestRecordingPreferenceDisablesAutomaticRecordingOnly(t *testing.T) {
	preference := RecordingPreference{AutomaticRecordingDisabled: true}

	if preference.AutomaticRecordingEnabled() {
		t.Fatal("expected automatic recording to be disabled")
	}
	if preference.ShouldRecordAutomatically() {
		t.Fatal("expected automatic recording decisions to be blocked")
	}
	if !preference.ShouldRecordManualAction() {
		t.Fatal("expected manual actions to remain allowed")
	}
}
