package device

import "testing"

func TestCommandKindReadOnlyClassification(t *testing.T) {
	readOnlyCommands := []CommandKind{
		CommandInquiry,
		CommandTestReady,
		CommandGetConfiguration,
		CommandReadCapacity,
		CommandReadTOC,
		CommandReadDiscInformation,
		CommandReadDVDStructure,
		CommandReadBlocks,
		CommandReadCD,
		CommandSetSpeed,
		CommandEject,
	}

	for _, command := range readOnlyCommands {
		if !command.IsReadOnly() {
			t.Fatalf("expected %q to be classified as read-only", command)
		}
	}

	if CommandKind("format_unit").IsReadOnly() {
		t.Fatal("expected unknown destructive command to be non-read-only")
	}
}
