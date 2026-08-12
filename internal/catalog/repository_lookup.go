package catalog

import (
	"context"
	"errors"
	"os"
	"path/filepath"
)

func (r *Repository) Lookup(ctx context.Context, identity ContentIdentity, budget LookupBudget) (LookupResult, error) {
	if err := contextError(ctx); err != nil {
		return LookupResult{}, err
	}
	if r == nil || r.closed {
		return LookupResult{}, errors.New("lookup catalog: repository is closed")
	}
	return Lookup(r.store.Entries, identity, budget)
}

// LookupPriorProcessing classifies matching catalog work and permits
// automatic resume only for a strong resumable candidate whose current image
// and map files both exist.
func (r *Repository) LookupPriorProcessing(ctx context.Context, observation IdentityObservation, budget LookupBudget) (PriorProcessingResult, error) {
	lookup, err := r.Lookup(ctx, observation.Identity, budget)
	if err != nil {
		return PriorProcessingResult{}, err
	}
	for i := range lookup.Candidates {
		lookup.Candidates[i] = refreshCandidateFiles(lookup.Candidates[i])
	}
	copyResult, err := BuildPriorProcessingCopy(lookup)
	if err != nil {
		return PriorProcessingResult{}, err
	}
	result := PriorProcessingResult{Match: lookup.Match, Candidates: lookup.Candidates}
	switch lookup.Match {
	case MatchStrong:
		if copyResult.State == PriorProcessingIncomplete {
			result.Kind = PriorStrongResume
			result.AutoResumeAllowed = true
		} else if copyResult.State == PriorProcessingUnavailable {
			result.Kind = PriorUnavailable
		} else {
			result.Kind = PriorStrongDone
		}
	case MatchProbable:
		result.Kind = PriorProbable
	case MatchIndeterminate:
		result.Kind = PriorIndeterminate
	case MatchConflict:
		result.Kind = PriorConflict
	default:
		result.Kind = PriorNone
	}
	result.Detail = copyResult.Detail
	return result, nil
}

// AppendEvent durably appends and syncs one event before applying it in memory.
func refreshCandidateFiles(entry Entry) Entry {
	refreshed := entry
	refreshed.JobReferences = append([]JobReference(nil), entry.JobReferences...)
	for i := range refreshed.JobReferences {
		imageInfo, imageErr := os.Stat(refreshed.JobReferences[i].Path)
		mapPath := replaceCatalogMapExtension(refreshed.JobReferences[i].Path)
		_, mapErr := os.Stat(mapPath)
		refreshed.JobReferences[i].FilesPresent = imageErr == nil && imageInfo.Mode().IsRegular() && mapErr == nil
	}
	return refreshed
}
func replaceCatalogMapExtension(path string) string {
	ext := filepath.Ext(path)
	if ext == ".iso" {
		return path[:len(path)-len(ext)] + ".drmap"
	}
	return path + ".drmap"
}
