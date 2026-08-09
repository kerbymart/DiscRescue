package device

import (
	"errors"
	"testing"
)

func TestCapabilityContractDistinguishesUnsupported(t *testing.T) {
	drive := DriveDescriptor{
		Ref:          DeviceRef{ID: "drive-1", Path: "/dev/sr0"},
		Presence:     DevicePresent,
		Capabilities: DriveCapabilities{SetReadSpeed: Capability{Status: SupportUnsupported, Detail: "not exposed by adapter"}},
	}
	if drive.Capabilities.SetReadSpeed.Status != SupportUnsupported {
		t.Fatal("unsupported capability must be explicit")
	}
	if drive.Ref.ID == "" || drive.Ref.Path == "" {
		t.Fatal("device reference must retain identity and path")
	}
}

func TestOperationErrorCodeSurvivesWrapping(t *testing.T) {
	base := &OperationError{Code: ErrorBusy, Op: "set read speed", Detail: "recovery owns drive"}
	if !IsCode(errors.Join(errors.New("outer"), base), ErrorBusy) {
		t.Fatal("expected busy code")
	}
}

func TestOwnershipSerializesRecoveryAndCommands(t *testing.T) {
	var ownership Ownership
	if err := ownership.Acquire(DriveRecoveryOwned); err != nil {
		t.Fatal(err)
	}
	if err := ownership.Acquire(DriveCommandOwned); !IsCode(err, ErrorBusy) {
		t.Fatalf("expected busy, got %v", err)
	}
	ownership.Release(DriveRecoveryOwned)
	if err := ownership.Acquire(DriveCommandOwned); err != nil {
		t.Fatal(err)
	}
}

func TestMediaObservationValidation(t *testing.T) {
	if err := (MediaObservation{Device: DeviceRef{ID: "d", Path: "/dev/sr0"}, Presence: MediaPresent}).Validate(); err == nil {
		t.Fatal("expected sector-size validation")
	}
	if err := (MediaObservation{Device: DeviceRef{ID: "d", Path: "/dev/sr0"}, Presence: MediaEmpty}).Validate(); err != nil {
		t.Fatal(err)
	}
}
