package catalog

import "fmt"

type LookupBudget struct {
	MaxComparedSamples uint16
}

func (b LookupBudget) Validate() error {
	if b.MaxComparedSamples == 0 {
		return fmt.Errorf("validate lookup budget: max compared samples must be greater than zero")
	}
	return nil
}

type LookupResult struct {
	Match              MatchStrength
	Candidates         []Entry
	MatchingSamples    uint16
	ConflictingSamples uint16
	IdentityVersion    uint16
	BudgetExhausted    bool
}

func Lookup(entries []Entry, identity ContentIdentity, budget LookupBudget) (LookupResult, error) {
	if err := identity.Validate(); err != nil {
		return LookupResult{}, err
	}
	if err := budget.Validate(); err != nil {
		return LookupResult{}, err
	}

	best := LookupResult{
		Match:           MatchNo,
		IdentityVersion: identity.Version,
	}
	compatibleFound := false
	compared := uint16(0)

	for _, entry := range entries {
		if err := entry.Identity.Validate(); err != nil {
			return LookupResult{}, fmt.Errorf("lookup entry: %w", err)
		}
		if !compatibleGeometry(entry.Identity, identity) || entry.Identity.LayoutSHA256 != identity.LayoutSHA256 {
			continue
		}
		compatibleFound = true

		match, matching, conflicting, exhausted := compareCandidate(entry.Identity, identity, budget.MaxComparedSamples-compared)
		if matching > 0 || conflicting > 0 {
			compared += matching + conflicting
		}

		candidateResult := LookupResult{
			Match:              match,
			Candidates:         []Entry{entry},
			MatchingSamples:    matching,
			ConflictingSamples: conflicting,
			IdentityVersion:    identity.Version,
			BudgetExhausted:    exhausted,
		}
		best = selectStrongerResult(best, candidateResult)
		if exhausted {
			best.BudgetExhausted = true
		}
		if best.Match == MatchConflict {
			return best, nil
		}
		if compared >= budget.MaxComparedSamples {
			best.BudgetExhausted = true
			if best.Match == MatchNo {
				best.Match = MatchIndeterminate
			}
			return best, nil
		}
	}

	if best.Match != MatchNo {
		return best, nil
	}
	if compatibleFound {
		best.Match = MatchIndeterminate
	}
	return best, nil
}

func compatibleGeometry(left, right ContentIdentity) bool {
	return left.Version == right.Version &&
		left.Profile == right.Profile &&
		left.LogicalBlockSize == right.LogicalBlockSize &&
		left.SectorCount == right.SectorCount &&
		left.Sessions == right.Sessions
}

func compareCandidate(candidate, observed ContentIdentity, remainingBudget uint16) (MatchStrength, uint16, uint16, bool) {
	if candidate.QuickID != "" && candidate.QuickID == observed.QuickID {
		return MatchStrong, 0, 0, false
	}

	indexed := make(map[uint16]SectorFingerprint, len(candidate.Samples))
	for _, sample := range candidate.Samples {
		indexed[sample.Slot] = sample
	}

	var matching uint16
	var conflicting uint16
	var compared uint16

	for _, sample := range observed.Samples {
		candidateSample, ok := indexed[sample.Slot]
		if !ok || !sample.Available || !candidateSample.Available {
			continue
		}
		if compared >= remainingBudget {
			return MatchIndeterminate, matching, conflicting, true
		}
		compared++
		if candidateSample.LBA != sample.LBA || candidateSample.SHA256 != sample.SHA256 {
			conflicting++
			return MatchConflict, matching, conflicting, false
		}
		matching++
	}

	switch {
	case conflicting > 0:
		return MatchConflict, matching, conflicting, false
	case matching >= 4:
		return MatchStrong, matching, conflicting, false
	case matching >= 1:
		return MatchProbable, matching, conflicting, false
	default:
		return MatchIndeterminate, matching, conflicting, false
	}
}

func selectStrongerResult(current, candidate LookupResult) LookupResult {
	if matchRank(candidate.Match) > matchRank(current.Match) {
		return candidate
	}
	if matchRank(candidate.Match) < matchRank(current.Match) {
		return current
	}
	if candidate.Match == MatchConflict || candidate.Match == MatchStrong || candidate.Match == MatchProbable {
		if candidate.MatchingSamples+candidate.ConflictingSamples > current.MatchingSamples+current.ConflictingSamples {
			return candidate
		}
	}
	if current.BudgetExhausted {
		candidate.BudgetExhausted = true
		return candidate
	}
	return current
}

func matchRank(match MatchStrength) int {
	switch match {
	case MatchConflict:
		return 4
	case MatchStrong:
		return 3
	case MatchProbable:
		return 2
	case MatchIndeterminate:
		return 1
	default:
		return 0
	}
}
