package catalog

import "testing"

func TestCatalogWriteStatusSummaryLines(t *testing.T) {
	tests := []struct {
		name  string
		state CatalogWriteState
		want  string
	}{
		{name: "not attempted", state: CatalogWriteNotAttempted},
		{name: "recorded", state: CatalogWriteRecorded, want: "Recorded in local processed-media catalog"},
		{name: "unavailable", state: CatalogWriteUnavailable, want: "Catalog unavailable; recovery files are still saved"},
		{name: "failed", state: CatalogWriteFailed, want: "Catalog update failed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status := CatalogWriteStatus{State: test.state}
			if got := status.SummaryLine(); got != test.want {
				t.Fatalf("SummaryLine() = %q, want %q", got, test.want)
			}
			if err := status.Validate(); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestCatalogWriteStatusRejectsUnknownState(t *testing.T) {
	if err := (CatalogWriteStatus{State: CatalogWriteState("unknown")}).Validate(); err == nil {
		t.Fatal("Validate() unexpectedly accepted an unknown catalog write state")
	}
}
