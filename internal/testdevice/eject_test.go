package testdevice

import (
	"testing"

	"discrescue/internal/device"
)

func TestEjectSimulatorCoversNormalAndForceOutcomes(t *testing.T) {
	spec := EjectSpec{NormalSupported: true, ForceSupported: true, NormalFails: true}
	if _, err := spec.Apply(device.EjectRequest{Mode: device.EjectNormal}, device.DriveUnowned); !device.IsCode(err, device.ErrorDeviceFailure) {
		t.Fatalf("normal result: %v", err)
	}
	result, err := spec.Apply(device.EjectRequest{Mode: device.EjectForce, ExplicitConfirm: true}, device.DriveUnowned)
	if err != nil || result.Verification != device.EjectConfirmed {
		t.Fatalf("force result=%+v err=%v", result, err)
	}
}

func TestEjectSimulatorRejectsUnsupportedAndBusy(t *testing.T) {
	if _, err := (EjectSpec{}).Apply(device.EjectRequest{Mode: device.EjectNormal}, device.DriveUnowned); !device.IsCode(err, device.ErrorUnsupported) {
		t.Fatalf("unsupported result: %v", err)
	}
	spec := EjectSpec{NormalSupported: true}
	if _, err := spec.Apply(device.EjectRequest{Mode: device.EjectNormal}, device.DriveRecoveryOwned); !device.IsCode(err, device.ErrorBusy) {
		t.Fatalf("busy result: %v", err)
	}
}
