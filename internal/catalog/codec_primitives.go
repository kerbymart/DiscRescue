package catalog

import (
	"fmt"
)

func processingStateCode(state ProcessingState) uint16 {
	switch state {
	case ProcessingObserved:
		return 0
	case ProcessingInProgress:
		return 1
	case ProcessingStoppedResumable:
		return 2
	case ProcessingCompletedVerified:
		return 3
	case ProcessingCompletedWithGaps:
		return 4
	case ProcessingFailed:
		return 5
	case ProcessingMerged:
		return 6
	case ProcessingCompleted:
		return 7
	default:
		return 0xffff
	}
}
func decodeProcessingState(code uint16) (ProcessingState, error) {
	switch code {
	case 0:
		return ProcessingObserved, nil
	case 1:
		return ProcessingInProgress, nil
	case 2:
		return ProcessingStoppedResumable, nil
	case 3:
		return ProcessingCompletedVerified, nil
	case 4:
		return ProcessingCompletedWithGaps, nil
	case 5:
		return ProcessingFailed, nil
	case 6:
		return ProcessingMerged, nil
	case 7:
		return ProcessingCompleted, nil
	default:
		return "", fmt.Errorf("decode processing state: unsupported code %d", code)
	}
}
