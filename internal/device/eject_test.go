package device

import (
	"bytes"
	"testing"
)

func TestNormalEjectDoesNotRequireForceConfirmation(t *testing.T) {
	want := EjectRequest{Mode: EjectNormal}
	payload, err := MarshalEjectRequest(want)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(payload, []byte{0, 0, 0, 0}) {
		t.Fatalf("unexpected payload %x", payload)
	}
	got, err := UnmarshalEjectRequest(payload)
	if err != nil || got != want {
		t.Fatalf("got %+v, err %v", got, err)
	}
}

func TestForceEjectRequiresExplicitConfirmation(t *testing.T) {
	if _, err := MarshalEjectRequest(EjectRequest{Mode: EjectForce}); err == nil {
		t.Fatal("expected force confirmation validation")
	}
	payload, err := MarshalEjectRequest(EjectRequest{Mode: EjectForce, ExplicitConfirm: true})
	if err != nil {
		t.Fatal(err)
	}
	got, err := UnmarshalEjectRequest(payload)
	if err != nil || got.Mode != EjectForce || !got.ExplicitConfirm {
		t.Fatalf("got %+v, err %v", got, err)
	}
}

func TestEjectOwnershipProtectsActiveRecovery(t *testing.T) {
	var ownership Ownership
	if err := ownership.Acquire(DriveRecoveryOwned); err != nil {
		t.Fatal(err)
	}
	if err := ownership.ValidateEject(EjectRequest{Mode: EjectNormal}); !IsCode(err, ErrorBusy) {
		t.Fatalf("got %v", err)
	}
	if err := ownership.ValidateEject(EjectRequest{Mode: EjectForce, ExplicitConfirm: true}); !IsCode(err, ErrorBusy) {
		t.Fatalf("got %v", err)
	}
	ownership.Release(DriveRecoveryOwned)
	if err := ownership.ValidateEject(EjectRequest{Mode: EjectNormal}); err != nil {
		t.Fatal(err)
	}
}
