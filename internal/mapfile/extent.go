package mapfile

import (
	"fmt"
	"math"
)

func (e Extent) EndLBA() uint64 {
	return e.StartLBA + uint64(e.Sectors)
}

// CheckedEndLBA returns the exclusive end of an extent without allowing
// uint64 arithmetic to wrap.
func (e Extent) CheckedEndLBA() (uint64, error) {
	sectors := uint64(e.Sectors)
	if sectors > math.MaxUint64-e.StartLBA {
		return 0, fmt.Errorf("extent lba range overflows")
	}
	return e.StartLBA + sectors, nil
}
func (e Extent) Validate() error {
	if e.Sectors == 0 {
		return fmt.Errorf("validate extent: sectors must be greater than zero")
	}
	if _, err := e.CheckedEndLBA(); err != nil {
		return fmt.Errorf("validate extent: %w", err)
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
