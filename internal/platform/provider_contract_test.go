package platform

import "testing"

// These assertions are the shared provider contract boundary. They verify
// compile-time substitutability without claiming that host hardware behavior
// can be proven on a different operating system.
var (
	_ OpticalDiscovery          = OSOpticalDiscovery{}
	_ OpticalCapabilityProvider = OSOpticalDiscovery{}
	_ OpticalEjector            = OSOpticalDiscovery{}
	_ RecoveryService           = OSRecovery{}
)

func TestNativeProvidersSatisfySharedContracts(t *testing.T) {
	if _, ok := any(OSOpticalDiscovery{}).(OpticalDiscovery); !ok {
		t.Fatal("native optical provider does not satisfy discovery contract")
	}
	if _, ok := any(OSRecovery{}).(RecoveryService); !ok {
		t.Fatal("native recovery provider does not satisfy recovery contract")
	}
}
