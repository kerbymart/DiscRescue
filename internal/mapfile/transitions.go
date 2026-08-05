package mapfile

import "fmt"

func ValidateSectorState(state SectorState) error {
	if state < SectorStateUnknown || state > SectorStateSkipped {
		return fmt.Errorf("validate sector state: unsupported state %d", state)
	}
	return nil
}

func ValidateConfidence(confidence Confidence) error {
	if confidence < ConfidenceNone || confidence > ConfidenceReconstructedChecksum {
		return fmt.Errorf("validate confidence: unsupported confidence %d", confidence)
	}
	return nil
}

func ValidateStateConfidence(state SectorState, confidence Confidence) error {
	if err := ValidateSectorState(state); err != nil {
		return err
	}
	if err := ValidateConfidence(confidence); err != nil {
		return err
	}

	switch state {
	case SectorStateUnknown, SectorStateQueued, SectorStateMissing, SectorStateIOError, SectorStateChecksumError, SectorStateConflicting, SectorStateSkipped:
		if confidence != ConfidenceNone {
			return fmt.Errorf("validate state confidence: state %s requires confidence %s", state, ConfidenceNone)
		}
	case SectorStateReadUnverified:
		if confidence != ConfidenceSingleRead {
			return fmt.Errorf("validate state confidence: state %s requires confidence %s", state, ConfidenceSingleRead)
		}
	case SectorStateVerified:
		if confidence < ConfidenceRepeatedSingleCapture || confidence > ConfidenceTrustedChecksum {
			return fmt.Errorf("validate state confidence: state %s requires confidence in [%s,%s]", state, ConfidenceRepeatedSingleCapture, ConfidenceTrustedChecksum)
		}
	case SectorStateReconstructed:
		if confidence != ConfidenceReconstructedChecksum {
			return fmt.Errorf("validate state confidence: state %s requires confidence %s", state, ConfidenceReconstructedChecksum)
		}
	}

	return nil
}

func ValidateTransition(currentState SectorState, currentConfidence Confidence, nextState SectorState, nextConfidence Confidence) error {
	if err := ValidateStateConfidence(currentState, currentConfidence); err != nil {
		return fmt.Errorf("validate transition current: %w", err)
	}
	if err := ValidateStateConfidence(nextState, nextConfidence); err != nil {
		return fmt.Errorf("validate transition next: %w", err)
	}

	if nextState == currentState && nextConfidence < currentConfidence {
		return fmt.Errorf("validate transition: cannot reduce confidence in-place from %s to %s", currentConfidence, nextConfidence)
	}

	switch currentState {
	case SectorStateVerified:
		if nextState == SectorStateReadUnverified {
			return fmt.Errorf("validate transition: cannot demote %s to %s", currentState, nextState)
		}
	case SectorStateReconstructed:
		if nextConfidence < currentConfidence {
			return fmt.Errorf("validate transition: cannot demote %s confidence from %s to %s", currentState, currentConfidence, nextConfidence)
		}
	case SectorStateConflicting:
		if nextState == SectorStateReadUnverified {
			return fmt.Errorf("validate transition: conflicting sectors cannot become unverified reads directly")
		}
	}

	return nil
}
