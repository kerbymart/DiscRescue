package catalog

type MatchStrength string

const (
	MatchStrong        MatchStrength = "strong"
	MatchProbable      MatchStrength = "probable"
	MatchIndeterminate MatchStrength = "indeterminate"
	MatchConflict      MatchStrength = "conflict"
)

type ContentIdentity struct {
	QuickID      string
	LayoutSHA256 string
}
