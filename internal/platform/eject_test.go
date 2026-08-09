package platform

import (
	"testing"

	"discrescue/internal/device"
)

func TestOSOpticalEjectValidatesForceConfirmationBeforeNativeCall(t *testing.T) {
	discovery := OSOpticalDiscovery{}
	_, err := discovery.EjectOpticalDrive("/dev/sr0", device.EjectRequest{Mode: device.EjectForce})
	if !device.IsCode(err, device.ErrorInvalidRequest) {
		t.Fatalf("got %v", err)
	}
}

func TestOSOpticalEjectCapabilityIsExplicit(t *testing.T) {
	capability := (OSOpticalDiscovery{}).EjectCapability("/dev/sr0")
	if capability.Status == device.SupportUnknown {
		t.Fatalf("capability must be explicit: %+v", capability)
	}
}

func TestOSOpticalCapabilitiesReportRecoveryExplicitly(t *testing.T) {
	capabilities := (OSOpticalDiscovery{}).OpticalCapabilities("/dev/sr0")
	if capabilities.RecoveryRead.Status == device.SupportUnknown {
		t.Fatalf("recovery capability must be explicit: %+v", capabilities)
	}
}
