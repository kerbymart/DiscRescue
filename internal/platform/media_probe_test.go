package platform

import (
	"errors"
	"strings"
	"syscall"
	"testing"
)

func TestMediaInspectionErrorPreservesNativeCauseAndActionableState(t *testing.T) {
	cause := syscall.EACCES
	err := &MediaInspectionError{
		Path:      "/dev/rdisk4",
		Operation: "open",
		State:     MediaProbePermission,
		Err:       cause,
	}

	if got := err.Error(); got != "inspect media: open /dev/rdisk4: permission denied" {
		t.Fatalf("unexpected error: %q", got)
	}
	if !errors.Is(err, cause) {
		t.Fatal("expected native cause to remain unwrap-able")
	}
}

func TestMediaInspectionErrorDoesNotCallMissingMediaUnavailable(t *testing.T) {
	err := (&MediaInspectionError{Path: "/dev/rdisk4", State: MediaProbeNoMedia}).Error()
	if strings.Contains(err, "no longer available") {
		t.Fatalf("no-media error was reported as unavailable: %q", err)
	}
}
