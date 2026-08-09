package app

import "testing"

func TestReconcileDevicesPreservesStableSelectionAcrossPathChange(t *testing.T) {
	selected := DeviceSummary{ID: "drive-a", Path: "/dev/sr0", MediaToken: "disc-1"}
	devices, current, invalidated := reconcileDevices([]DeviceSummary{{ID: "drive-a", Path: "/dev/sr1", DisplayName: "Drive"}}, selected)
	if len(devices) != 1 || current.Path != "/dev/sr1" || invalidated {
		t.Fatalf("devices=%+v current=%+v invalidated=%v", devices, current, invalidated)
	}
}

func TestReconcileDevicesInvalidatesRemovedSelection(t *testing.T) {
	selected := DeviceSummary{ID: "drive-a", Path: "/dev/sr0", MediaToken: "disc-1"}
	_, current, invalidated := reconcileDevices([]DeviceSummary{{ID: "drive-b", Path: "/dev/sr2"}}, selected)
	if current.ID != "" || !invalidated {
		t.Fatalf("current=%+v invalidated=%v", current, invalidated)
	}
}

func TestReconcileDevicesInvalidatesMediaTokenChange(t *testing.T) {
	selected := DeviceSummary{ID: "drive-a", Path: "/dev/sr0", MediaToken: "disc-1"}
	_, current, invalidated := reconcileDevices([]DeviceSummary{{ID: "drive-a", Path: "/dev/sr0", MediaToken: "disc-2"}}, selected)
	if current.ID != "drive-a" || !invalidated {
		t.Fatalf("current=%+v invalidated=%v", current, invalidated)
	}
}
