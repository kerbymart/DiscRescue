package testdevice

import (
	"testing"

	"discrescue/internal/device"
)

func TestRequestAuditAcceptsReadOnlyCommandsWithinBudget(t *testing.T) {
	audit := RequestAudit{
		Entries: []RequestAuditEntry{
			{Command: device.CommandInquiry, Pass: PassFast, Attempt: 1},
			{Command: device.CommandReadBlocks, Pass: PassFast, Attempt: 2, StartLBA: 128, Sectors: 16},
			{Command: device.CommandTestReady, Pass: PassScrape, Attempt: 1},
		},
	}

	if err := audit.AssertReadOnlyCommands(); err != nil {
		t.Fatalf("expected read-only audit to pass, got error: %v", err)
	}
	if err := audit.AssertRetryBudget(RetryBudget{Pass: PassFast, MaxAttempt: 2}); err != nil {
		t.Fatalf("expected retry budget check to pass, got error: %v", err)
	}
}

func TestRequestAuditRejectsNonReadOnlyCommand(t *testing.T) {
	audit := RequestAudit{
		Entries: []RequestAuditEntry{
			{Command: device.CommandKind("format_unit"), Pass: PassFast, Attempt: 1},
		},
	}

	if err := audit.AssertReadOnlyCommands(); err == nil {
		t.Fatal("expected non-read-only command to fail audit")
	}
}

func TestRequestAuditRejectsRetryBudgetViolation(t *testing.T) {
	audit := RequestAudit{
		Entries: []RequestAuditEntry{
			{Command: device.CommandReadBlocks, Pass: PassTrim, Attempt: 3, StartLBA: 512, Sectors: 1},
		},
	}

	if err := audit.AssertRetryBudget(RetryBudget{Pass: PassTrim, MaxAttempt: 2}); err == nil {
		t.Fatal("expected retry budget violation to fail audit")
	}
}

func TestRequestAuditRejectsReadCommandWithoutSectors(t *testing.T) {
	audit := RequestAudit{
		Entries: []RequestAuditEntry{
			{Command: device.CommandReadCD, Pass: PassScrape, Attempt: 1},
		},
	}

	if err := audit.AssertReadOnlyCommands(); err == nil {
		t.Fatal("expected invalid read command shape to fail audit")
	}
}
