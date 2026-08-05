package mapfile

import "fmt"

type Extent struct {
	StartLBA   uint64
	Sectors    uint32
	State      SectorState
	Confidence Confidence
}

func (e Extent) EndLBA() uint64 {
	return e.StartLBA + uint64(e.Sectors)
}

func (e Extent) Validate() error {
	if e.Sectors == 0 {
		return fmt.Errorf("validate extent: sectors must be greater than zero")
	}
	return ValidateStateConfidence(e.State, e.Confidence)
}

func (e Extent) Transition(nextState SectorState, nextConfidence Confidence) (Extent, error) {
	if err := ValidateTransition(e.State, e.Confidence, nextState, nextConfidence); err != nil {
		return Extent{}, err
	}

	next := e
	next.State = nextState
	next.Confidence = nextConfidence
	return next, nil
}

func Overlaps(left, right Extent) bool {
	return left.StartLBA < right.EndLBA() && right.StartLBA < left.EndLBA()
}
