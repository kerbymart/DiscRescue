package device

import (
	"fmt"
	"time"
)

type RestartPolicy string

const (
	RestartNever     RestartPolicy = "never"
	RestartRetryable RestartPolicy = "retryable"
	RestartAlways    RestartPolicy = "always"
)

type EffectKind string

const (
	EffectDispatchRequest EffectKind = "dispatch_request"
	EffectSoftDeadline    EffectKind = "soft_deadline"
	EffectHardDeadline    EffectKind = "hard_deadline"
	EffectRestartWorker   EffectKind = "restart_worker"
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

type ActiveCommand struct {
	Drive            string
	Request          CommandRequest
	Deadlines        Deadlines
	StartedAt        time.Time
	SoftDeadlineAt   time.Time
	HardDeadlineAt   time.Time
	SoftDeadlineSent bool
}

type DispatchEffect struct {
	Kind      EffectKind
	RequestID uint64
	Drive     string
}

type Supervisor struct {
	OwnedDrive           string
	NextRequestID        uint64
	LastCompletedRequest uint64
	Active               *ActiveCommand
	RestartPolicy        RestartPolicy
}

func (s Supervisor) CanDispatch() bool {
	return s.Active == nil
}

func (s Supervisor) CanOwnDrive(drive string) bool {
	return s.OwnedDrive == "" || s.OwnedDrive == drive
}

func (s Supervisor) AcquireDrive(drive string) (Supervisor, bool) {
	if drive == "" || !s.CanOwnDrive(drive) {
		return s, false
	}
	next := s
	next.OwnedDrive = drive
	return next, true
}

func (s Supervisor) ReleaseDrive(drive string) Supervisor {
	if s.OwnedDrive != drive {
		return s
	}
	next := s
	next.OwnedDrive = ""
	return next
}

func DefaultDeadlines(request CommandRequest) (Deadlines, error) {
	switch request.Command {
	case CommandInquiry, CommandGetConfiguration, CommandReadCapacity, CommandReadTOC, CommandReadDiscInformation, CommandReadDVDStructure:
		return Deadlines{Soft: 10 * time.Second, Hard: 30 * time.Second, RetryBudget: 1}, nil
	case CommandReadBlocks, CommandReadCD:
		if request.Sectors <= 1 {
			return Deadlines{Soft: 30 * time.Second, Hard: 120 * time.Second, RetryBudget: 3}, nil
		}
		return Deadlines{Soft: 15 * time.Second, Hard: 45 * time.Second, RetryBudget: 3}, nil
	case CommandSetSpeed:
		return Deadlines{Soft: 5 * time.Second, Hard: 15 * time.Second, RetryBudget: 1}, nil
	case CommandEject:
		return Deadlines{Soft: 10 * time.Second, Hard: 30 * time.Second, RetryBudget: 1}, nil
	case CommandTestReady:
		return Deadlines{Soft: 10 * time.Second, Hard: 30 * time.Second, RetryBudget: 2}, nil
	default:
		return Deadlines{}, fmt.Errorf("default deadlines: unsupported command %q", request.Command)
	}
}

func (s Supervisor) Dispatch(now time.Time, drive string, request CommandRequest, deadlines Deadlines) (Supervisor, []DispatchEffect, error) {
	if !s.CanDispatch() {
		return s, nil, fmt.Errorf("dispatch: command %d is already active", s.Active.Request.ID)
	}
	if !s.CanOwnDrive(drive) {
		return s, nil, fmt.Errorf("dispatch: drive %q is already owned by another active worker", drive)
	}
	if err := deadlines.Validate(); err != nil {
		return s, nil, err
	}
	if request.Command == "" {
		return s, nil, fmt.Errorf("dispatch: command is required")
	}

	next := s
	next, _ = next.AcquireDrive(drive)
	next.NextRequestID++
	request.ID = next.NextRequestID
	next.Active = &ActiveCommand{
		Drive:          drive,
		Request:        request,
		Deadlines:      deadlines,
		StartedAt:      now,
		SoftDeadlineAt: now.Add(deadlines.Soft),
		HardDeadlineAt: now.Add(deadlines.Hard),
	}

	return next, []DispatchEffect{{
		Kind:      EffectDispatchRequest,
		RequestID: request.ID,
		Drive:     drive,
	}}, nil
}

func (s Supervisor) ObserveResult(requestID uint64) (Supervisor, bool, error) {
	if s.Active == nil {
		return s, true, nil
	}
	if requestID != s.Active.Request.ID {
		return s, true, nil
	}

	next := s
	next.LastCompletedRequest = requestID
	next.Active = nil
	next = next.ReleaseDrive(s.OwnedDrive)
	return next, false, nil
}

func (s Supervisor) CheckDeadlines(now time.Time) (Supervisor, []DispatchEffect, error) {
	if s.Active == nil {
		return s, nil, nil
	}

	next := s
	var effects []DispatchEffect

	if !next.Active.SoftDeadlineSent && !now.Before(next.Active.SoftDeadlineAt) {
		next.Active.SoftDeadlineSent = true
		effects = append(effects, DispatchEffect{
			Kind:      EffectSoftDeadline,
			RequestID: next.Active.Request.ID,
			Drive:     next.Active.Drive,
		})
	}
	if !now.Before(next.Active.HardDeadlineAt) {
		effects = append(effects, DispatchEffect{
			Kind:      EffectHardDeadline,
			RequestID: next.Active.Request.ID,
			Drive:     next.Active.Drive,
		})
	}
	return next, effects, nil
}

func (s Supervisor) RestartDecision(retryable bool) []DispatchEffect {
	switch s.RestartPolicy {
	case RestartAlways:
		return []DispatchEffect{{Kind: EffectRestartWorker, Drive: s.OwnedDrive}}
	case RestartRetryable:
		if retryable {
			return []DispatchEffect{{Kind: EffectRestartWorker, Drive: s.OwnedDrive}}
		}
	}
	return nil
}
