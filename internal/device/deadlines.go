package device

import (
	"fmt"
	"time"
)

type Deadlines struct {
	Soft        time.Duration
	Hard        time.Duration
	RetryBudget uint16
}

func (d Deadlines) Validate() error {
	if d.Soft <= 0 {
		return fmt.Errorf("validate deadlines: soft deadline must be greater than zero")
	}
	if d.Hard <= 0 {
		return fmt.Errorf("validate deadlines: hard deadline must be greater than zero")
	}
	if d.Hard < d.Soft {
		return fmt.Errorf("validate deadlines: hard deadline %s is smaller than soft deadline %s", d.Hard, d.Soft)
	}
	if d.RetryBudget == 0 {
		return fmt.Errorf("validate deadlines: retry budget must be greater than zero")
	}
	return nil
}

func DefaultDeadlines(request CommandRequest) (Deadlines, error) {
	spec, ok := AllowedCommandSpec(request.Command)
	if !ok {
		return Deadlines{}, fmt.Errorf("default deadlines: unsupported command %q", request.Command)
	}
	switch spec.Deadline {
	case DeadlineProfileMetadata:
		return Deadlines{Soft: 10 * time.Second, Hard: 30 * time.Second, RetryBudget: 1}, nil
	case DeadlineProfileRead:
		if request.Sectors <= 1 {
			return Deadlines{Soft: 30 * time.Second, Hard: 120 * time.Second, RetryBudget: 3}, nil
		}
		return Deadlines{Soft: 15 * time.Second, Hard: 45 * time.Second, RetryBudget: 3}, nil
	case DeadlineProfileSetSpeed:
		return Deadlines{Soft: 5 * time.Second, Hard: 15 * time.Second, RetryBudget: 1}, nil
	case DeadlineProfileEject:
		return Deadlines{Soft: 10 * time.Second, Hard: 30 * time.Second, RetryBudget: 1}, nil
	case DeadlineProfileTestReady:
		return Deadlines{Soft: 10 * time.Second, Hard: 30 * time.Second, RetryBudget: 2}, nil
	default:
		return Deadlines{}, fmt.Errorf("default deadlines: unsupported command %q", request.Command)
	}
}
