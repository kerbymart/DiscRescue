package merge

import (
	"reflect"
)

func appendMergedExtent(result []MergedExtent, next MergedExtent) []MergedExtent {
	if len(result) == 0 {
		return append(result, next)
	}
	last := result[len(result)-1]
	if canMergeMergedExtent(last, next) {
		last.Extent.Sectors += next.Extent.Sectors
		result[len(result)-1] = last
		return result
	}
	return append(result, next)
}
func canMergeMergedExtent(left, right MergedExtent) bool {
	return left.Extent.EndLBA() == right.Extent.StartLBA &&
		left.Extent.State == right.Extent.State &&
		left.Extent.Confidence == right.Extent.Confidence &&
		left.SelectedCaptureID == right.SelectedCaptureID &&
		left.SelectionRule == right.SelectionRule &&
		reflect.DeepEqual(left.Provenance, right.Provenance) &&
		left.UnresolvedConflict == right.UnresolvedConflict &&
		reflect.DeepEqual(left.CandidateHashes, right.CandidateHashes)
}
