package app

import (
	"strings"
	"testing"
)

func TestRenderPassRecoveryBodyDistinguishesRecoveryStates(t *testing.T) {
	model := NewModel()
	model.Recovery = RecoveryViewModel{
		Phase:             "Fast acquisition",
		Status:            recoveryPassStatus("Fast acquisition"),
		ScannedSectors:    80,
		RecoveredSectors:  75,
		DeferredSectors:   4,
		UnreadableSectors: 1,
		TotalSectors:      100,
	}

	body := strings.Join(renderPassRecoveryBody(model, 76, layoutFull), "\n")
	for _, want := range []string{
		"80% scanned",
		"Scanned       80 of 100 sectors",
		"Recovered     75 sectors",
		"Deferred      4 sectors",
		"Unreadable    1 sectors",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("recovery body missing %q:\n%s", want, body)
		}
	}
}

func TestRecoveryPassStatusMakesDeferredWorkExplicit(t *testing.T) {
	for _, test := range []struct {
		pass string
		want string
	}{
		{pass: "Fast acquisition", want: "deferred"},
		{pass: "Trimming deferred ranges", want: "edges"},
		{pass: "Adaptive recovery (4-sector reads)", want: "smaller"},
		{pass: "Targeted retry", want: "bounded"},
	} {
		if got := recoveryPassStatus(test.pass); !strings.Contains(strings.ToLower(got), test.want) {
			t.Fatalf("status for %q = %q, want it to mention %q", test.pass, got, test.want)
		}
	}
}
