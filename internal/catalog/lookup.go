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
		if entry.Hidden {
			continue
		}
		if err := entry.Identity.Validate(); err != nil {
			return LookupResult{}, fmt.Errorf("lookup entry: %w", err)
		}
		if !CompatibleGeometry(entry.Identity, identity) || entry.Identity.LayoutSHA256 != identity.LayoutSHA256 {
			continue
		}
		compatibleFound = true
		if compared >= budget.MaxComparedSamples {
			best.BudgetExhausted = true
			if best.Match == MatchNo {
				best.Match = MatchIndeterminate
			}
			return best, nil
		}

		comparison, err := CompareContentIdentity(entry.Identity, identity, budget.MaxComparedSamples-compared)
		if err != nil {
			return LookupResult{}, err
		}
		if comparison.MatchingSamples > 0 || comparison.ConflictingSamples > 0 {
			compared += comparison.MatchingSamples + comparison.ConflictingSamples
		}

		candidateResult := LookupResult{
			Match:              comparison.Match,
			Candidates:         []Entry{entry},
			MatchingSamples:    comparison.MatchingSamples,
			ConflictingSamples: comparison.ConflictingSamples,
			IdentityVersion:    identity.Version,
			BudgetExhausted:    comparison.BudgetExhausted,
		}
		best = selectStrongerResult(best, candidateResult)
		if comparison.BudgetExhausted {
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
