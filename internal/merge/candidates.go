package merge

import (
	"bytes"
	"sort"

	"discrescue/internal/mapfile"
)

func filterTrustedVerified(candidates []candidateExtent) ([]candidateExtent, bool) {
	var filtered []candidateExtent
	for _, candidate := range candidates {
		if candidate.Extent.State == mapfile.SectorStateVerified && candidate.Extent.Confidence == mapfile.ConfidenceTrustedChecksum {
			filtered = append(filtered, candidate)
		}
	}
	return filtered, len(filtered) > 0
}
func filterReconstructed(candidates []candidateExtent) ([]candidateExtent, bool) {
	var filtered []candidateExtent
	for _, candidate := range candidates {
		if candidate.Extent.State == mapfile.SectorStateReconstructed && candidate.Extent.Confidence == mapfile.ConfidenceReconstructedChecksum {
			filtered = append(filtered, candidate)
		}
	}
	return filtered, len(filtered) > 0
}
func filterVerified(candidates []candidateExtent) ([]candidateExtent, bool) {
	var filtered []candidateExtent
	for _, candidate := range candidates {
		if candidate.Extent.State == mapfile.SectorStateVerified {
			filtered = append(filtered, candidate)
		}
	}
	return filtered, len(filtered) > 0
}
func filterUnverified(candidates []candidateExtent) ([]candidateExtent, bool) {
	var filtered []candidateExtent
	for _, candidate := range candidates {
		if candidate.Extent.State == mapfile.SectorStateReadUnverified && candidate.Extent.Confidence == mapfile.ConfidenceSingleRead {
			filtered = append(filtered, candidate)
		}
	}
	return filtered, len(filtered) > 0
}
func allSameHash(candidates []candidateExtent) bool {
	if len(candidates) == 0 {
		return false
	}
	first := candidates[0].Extent.DataHash
	for _, candidate := range candidates[1:] {
		if candidate.Extent.DataHash != first {
			return false
		}
	}
	return true
}
func distinctCaptures(candidates []candidateExtent) int {
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		seen[candidate.CaptureID] = struct{}{}
	}
	return len(seen)
}
func uniqueHashes(candidates []candidateExtent) [][16]byte {
	seen := make(map[[16]byte]struct{}, len(candidates))
	hashes := make([][16]byte, 0, len(candidates))
	for _, candidate := range candidates {
		if !claimsData(candidate.Extent.State) {
			continue
		}
		if _, ok := seen[candidate.Extent.DataHash]; ok {
			continue
		}
		seen[candidate.Extent.DataHash] = struct{}{}
		hashes = append(hashes, candidate.Extent.DataHash)
	}
	sort.Slice(hashes, func(i, j int) bool {
		return bytes.Compare(hashes[i][:], hashes[j][:]) < 0
	})
	return hashes
}
func claimsData(state mapfile.SectorState) bool {
	switch state {
	case mapfile.SectorStateReadUnverified, mapfile.SectorStateVerified, mapfile.SectorStateReconstructed:
		return true
	default:
		return false
	}
}
