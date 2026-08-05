package testdevice

import (
	"fmt"

	"discrescue/internal/device"
)

type RequestAuditEntry struct {
	Command  device.CommandKind
	Pass     PassName
	Attempt  uint16
	StartLBA uint64
	Sectors  uint32
}

func (e RequestAuditEntry) Validate() error {
	if e.Command == "" {
		return fmt.Errorf("validate audit entry: command is required")
	}
	if e.Attempt == 0 {
		return fmt.Errorf("validate audit entry: attempt must be greater than zero")
	}
	if e.Command == device.CommandReadBlocks || e.Command == device.CommandReadCD {
		if e.Sectors == 0 {
			return fmt.Errorf("validate audit entry: read command requires sectors")
		}
	}
	return nil
}

type RetryBudget struct {
	Pass       PassName
	MaxAttempt uint16
}

func (b RetryBudget) Validate() error {
	if b.MaxAttempt == 0 {
		return fmt.Errorf("validate retry budget: max attempt must be greater than zero")
	}
	return nil
}

type RequestAudit struct {
	Entries []RequestAuditEntry
}

func (a RequestAudit) Validate() error {
	for i, entry := range a.Entries {
		if err := entry.Validate(); err != nil {
			return fmt.Errorf("validate request audit entry[%d]: %w", i, err)
		}
	}
	return nil
}

func (a RequestAudit) AssertReadOnlyCommands() error {
	if err := a.Validate(); err != nil {
		return err
	}
	for i, entry := range a.Entries {
		if !entry.Command.IsReadOnly() {
			return fmt.Errorf("assert read-only commands entry[%d]: command %q is not read-only", i, entry.Command)
		}
	}
	return nil
}

func (a RequestAudit) AssertRetryBudget(budget RetryBudget) error {
	if err := a.Validate(); err != nil {
		return err
	}
	if err := budget.Validate(); err != nil {
		return err
	}
	for i, entry := range a.Entries {
		if budget.Pass != PassAny && entry.Pass != budget.Pass {
			continue
		}
		if entry.Attempt > budget.MaxAttempt {
			return fmt.Errorf(
				"assert retry budget entry[%d]: attempt %d exceeds budget %d for pass %q",
				i,
				entry.Attempt,
				budget.MaxAttempt,
				budget.Pass,
			)
		}
	}
	return nil
}
