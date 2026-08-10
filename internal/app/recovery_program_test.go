package app

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestRenderPassRecoveryBodyDistinguishesRecoveryStates(t *testing.T) {
	model := NewModel()
	model.Height = 36
	model.Recovery = RecoveryViewModel{
		Phase:             "Fast acquisition",
		Status:            recoveryPassStatus("Fast acquisition"),
		ScannedSectors:    80,
		RecoveredSectors:  75,
		DeferredSectors:   4,
		UnreadableSectors: 1,
		TotalSectors:      100,
	}

	body := ansi.Strip(strings.Join(renderPassRecoveryBody(model, 76, layoutFull), "\n"))
	for _, want := range []string{
		"80%",
		"80 / 100 sectors",
		"Recovered",
		"Deferred",
		"Unreadable",
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
