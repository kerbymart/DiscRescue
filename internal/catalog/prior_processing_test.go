package catalog

import "testing"

func TestBuildPriorProcessingCopyReturnsNoMatchLine(t *testing.T) {
	copy, err := BuildPriorProcessingCopy(LookupResult{Match: MatchNo})
	if err != nil {
		t.Fatalf("build prior-processing copy: %v", err)
	}

	if copy.State != PriorProcessingNone {
		t.Fatalf("expected no-match state, got %q", copy.State)
	}
	if copy.Detail != "History: no matching contents found on this computer" {
		t.Fatalf("unexpected detail: %q", copy.Detail)
	}
}

func TestBuildPriorProcessingCopyReturnsIndeterminateLine(t *testing.T) {
	copy, err := BuildPriorProcessingCopy(LookupResult{Match: MatchIndeterminate})
	if err != nil {
		t.Fatalf("build prior-processing copy: %v", err)
	}

	if copy.State != PriorProcessingIndeterminate {
		t.Fatalf("expected indeterminate state, got %q", copy.State)
	}
	if copy.Detail != "History: could not identify enough readable areas to check reliably" {
		t.Fatalf("unexpected detail: %q", copy.Detail)
	}
}

func TestBuildPriorProcessingCopyReturnsProbableMatchCopy(t *testing.T) {
	entry := testCatalogEntry(1, 100, 200)

	copy, err := BuildPriorProcessingCopy(LookupResult{
		Match:      MatchProbable,
		Candidates: []Entry{entry},
	})
	if err != nil {
		t.Fatalf("build prior-processing copy: %v", err)
	}

	if copy.State != PriorProcessingPossibleMatch {
		t.Fatalf("expected probable-match state, got %q", copy.State)
	}
	if copy.Title != "These contents may have been processed before" {
		t.Fatalf("unexpected title: %q", copy.Title)
	}
}

func TestBuildPriorProcessingCopyReturnsConflictCopy(t *testing.T) {
	entry := testCatalogEntry(1, 100, 200)

	copy, err := BuildPriorProcessingCopy(LookupResult{
		Match:      MatchConflict,
		Candidates: []Entry{entry},
	})
	if err != nil {
		t.Fatalf("build prior-processing copy: %v", err)
	}

	if copy.State != PriorProcessingConflict {
		t.Fatalf("expected conflict state, got %q", copy.State)
	}
	if copy.Title != "Matching contents conflict with a previous record" {
		t.Fatalf("unexpected title: %q", copy.Title)
	}
}

func TestBuildPriorProcessingCopyMapsObservedToSeen(t *testing.T) {
	entry := testCatalogEntry(1, 100, 200)
	entry.State = ProcessingObserved
	entry.Status = string(ProcessingObserved)

	copy, err := BuildPriorProcessingCopy(LookupResult{
		Match:      MatchStrong,
		Candidates: []Entry{entry},
	})
	if err != nil {
		t.Fatalf("build prior-processing copy: %v", err)
	}

	if copy.State != PriorProcessingSeen {
		t.Fatalf("expected seen state, got %q", copy.State)
	}
	if copy.Detail != "Seen before; no recovery was started" {
		t.Fatalf("unexpected detail: %q", copy.Detail)
	}
}

func TestBuildPriorProcessingCopyMapsIncompleteRecovery(t *testing.T) {
	entry := testCatalogEntry(1, 100, 200)
	entry.State = ProcessingStoppedResumable
	entry.Status = string(ProcessingStoppedResumable)

	copy, err := BuildPriorProcessingCopy(LookupResult{
		Match:      MatchStrong,
		Candidates: []Entry{entry},
	})
	if err != nil {
		t.Fatalf("build prior-processing copy: %v", err)
	}

	if copy.State != PriorProcessingIncomplete {
		t.Fatalf("expected incomplete state, got %q", copy.State)
	}
	if copy.Detail != "Recovery incomplete; progress can be resumed" {
		t.Fatalf("unexpected detail: %q", copy.Detail)
	}
}

func TestBuildPriorProcessingCopyMapsCompletedArchive(t *testing.T) {
	entry := testCatalogEntry(1, 100, 200)
	entry.State = ProcessingCompletedVerified
	entry.Status = string(ProcessingCompletedVerified)

	copy, err := BuildPriorProcessingCopy(LookupResult{
		Match:      MatchStrong,
		Candidates: []Entry{entry},
	})
	if err != nil {
		t.Fatalf("build prior-processing copy: %v", err)
	}

	if copy.State != PriorProcessingArchived {
		t.Fatalf("expected archived state, got %q", copy.State)
	}
	if copy.Detail != "Archived successfully" {
		t.Fatalf("unexpected detail: %q", copy.Detail)
	}
}

func TestBuildPriorProcessingCopyMapsMissingFilesToUnavailable(t *testing.T) {
	entry := testCatalogEntry(1, 100, 200)
	entry.State = ProcessingCompletedVerified
	entry.Status = string(ProcessingCompletedVerified)
	entry.JobReferences = []JobReference{
		{JobID: entry.PreferredJobID, Path: "D:/archive/family.iso", FilesPresent: false},
	}

	copy, err := BuildPriorProcessingCopy(LookupResult{
		Match:      MatchStrong,
		Candidates: []Entry{entry},
	})
	if err != nil {
		t.Fatalf("build prior-processing copy: %v", err)
	}

	if copy.State != PriorProcessingUnavailable {
		t.Fatalf("expected unavailable state, got %q", copy.State)
	}
	if copy.Detail != "Previous files not found" {
		t.Fatalf("unexpected detail: %q", copy.Detail)
	}
}
