package device

import (
	"slices"
	"testing"
)

func TestAllowedCommandKindsMatchReadOnlyVocabulary(t *testing.T) {
	expected := []CommandKind{
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

	allowed := AllowedCommandKinds()
	if !slices.Equal(allowed, expected) {
		t.Fatalf("unexpected allowed commands: got %v want %v", allowed, expected)
	}
}

func TestAllowedCommandKindsAreReadOnly(t *testing.T) {
	for _, command := range AllowedCommandKinds() {
		spec, ok := AllowedCommandSpec(command)
		if !ok {
			t.Fatalf("expected command %q to have a spec", command)
		}
		if !spec.ReadOnly {
			t.Fatalf("expected command %q to be read-only", command)
		}
		if !command.IsReadOnly() {
			t.Fatalf("expected command %q to be classified as read-only", command)
		}
	}
}

func TestValidateCommandKindRejectsUnknownOrDestructiveCommands(t *testing.T) {
	for _, command := range []CommandKind{
		"format_unit",
		"blank_disc",
		"close_track",
		"reserve_track",
		"write_blocks",
		"scsi_reset",
		"bus_reset",
		"controller_reset",
		"unmount_media",
		"mount_media",
		"shell_exec",
	} {
		if err := ValidateCommandKind(command); err == nil {
			t.Fatalf("expected command %q to be rejected", command)
		}
	}
}

func TestCommandRequestValidateRequiresSectorsForReadCommands(t *testing.T) {
	request := CommandRequest{
		Command: CommandReadBlocks,
		Sectors: 0,
	}
	if err := request.Validate(); err == nil {
		t.Fatal("expected read command without sectors to fail")
	}
}

func TestAllowedCommandSpecExposesProjectOwnedMetadata(t *testing.T) {
	spec, ok := AllowedCommandSpec(CommandReadCD)
	if !ok {
		t.Fatal("expected read cd command spec to exist")
	}
	if spec.DisplayName != "READ CD" {
		t.Fatalf("unexpected display name: %q", spec.DisplayName)
	}
	if spec.Opcode != 0xbe {
		t.Fatalf("unexpected opcode: %#x", spec.Opcode)
	}
	if !spec.AllowsMediaIO {
		t.Fatal("expected read cd to allow media io")
	}
}
