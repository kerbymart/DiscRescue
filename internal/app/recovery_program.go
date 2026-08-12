package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// RecoveryProgramModel decorates ProgramModel with the pass-level progress
// fields exposed by the recovery service. Keeping the adapter here lets the
// existing UI state machine remain compatible while the TUI can distinguish
// scanned coverage, recovered data, deferred work, and confirmed unreadable
// sectors.
type RecoveryProgramModel struct {
	ProgramModel
}

func NewRecoveryProgramModel(services Services) RecoveryProgramModel {
	return RecoveryProgramModel{ProgramModel: NewProgramModel(services)}
}

func (m RecoveryProgramModel) Init() tea.Cmd {
	return tea.Batch(m.ProgramModel.Init(), m.LoadingSpinner.Tick)
}

func (m RecoveryProgramModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.ProgramModel.Update(msg)
	program, ok := next.(ProgramModel)
	if !ok {
		return next, cmd
	}

	previousDeferred := m.Recovery.DeferredSectors
	previousScanned := m.Recovery.ScannedSectors
	if program.state != nil && program.state.activeRecovery != nil {
		snapshot := program.state.activeRecovery.Snapshot()
		program.Recovery.ScannedSectors = snapshot.ScannedSectors
		program.Recovery.DeferredSectors = snapshot.DeferredSectors
		program.Recovery.UnreadableSectors = snapshot.UnreadableSectors
		if strings.TrimSpace(snapshot.Pass) != "" {
			program.Recovery.Phase = snapshot.Pass
			program.Recovery.Status = recoveryPassStatus(snapshot.Pass)
		}
	}

	switch typed := msg.(type) {
	case JobStartedMsg:
		if program.Recovery.ScannedSectors == 0 {
			program.Recovery.ScannedSectors = typed.ScannedSectors
		}
		if program.Recovery.DeferredSectors == 0 {
			program.Recovery.DeferredSectors = typed.DeferredSectors
		}
	case JobPausedMsg:
		if typed.ScannedSectors > 0 {
			program.Recovery.ScannedSectors = typed.ScannedSectors
		} else {
			program.Recovery.ScannedSectors = previousScanned
		}
		if typed.DeferredSectors > 0 {
			program.Recovery.DeferredSectors = typed.DeferredSectors
		} else {
			program.Recovery.DeferredSectors = previousDeferred
		}
	case JobStoppedMsg:
		switch typed.Summary.Outcome {
		case "Recovery complete", "Recovery finished with deferred sectors", "Recovery finished with unreadable sectors":
			program.Recovery.ScannedSectors = program.Recovery.TotalSectors
			program.Recovery.DeferredSectors = typed.Summary.DeferredSectors
			program.Summary.DeferredSectors = typed.Summary.DeferredSectors
		default:
			program.Recovery.ScannedSectors = previousScanned
			program.Recovery.DeferredSectors = previousDeferred
			program.Summary.DeferredSectors = previousDeferred
			program.Summary.UnresolvedSectors += previousDeferred
		}
	}

	return RecoveryProgramModel{ProgramModel: program}, cmd
}

func (m RecoveryProgramModel) View() tea.View {
	if m.Page != PageRecovering {
		return m.ProgramModel.View()
	}
	return renderPassRecoveryView(m.Model)
}

func recoveryPassStatus(pass string) string {
	switch {
	case pass == "Fast acquisition":
		return "Scanning forward with large reads; failed ranges are deferred so recovery can keep moving."
	case pass == "Trimming deferred ranges":
		return "Recovering readable edges around deferred ranges with bounded reads."
	case strings.HasPrefix(pass, "Adaptive recovery"):
		return "Revisiting deferred ranges with progressively smaller bounded reads."
	case pass == "Targeted retry":
		return "Running bounded targeted retries on the smallest unresolved ranges."
	case pass == "Complete":
		return "Recovery passes are complete."
	default:
		return "Reading sectors from the selected optical drive."
	}
}

func renderPassRecoveryView(m Model) tea.View {
	tier := layoutTierFor(m.Width, m.Height)
	if tier == layoutTooSmall {
		return m.View()
	}

	width := contentWidth(m.Width)
	view := tea.NewView(renderShell(m, renderPassRecoveryBody(m, width, tier), tier))
	view.WindowTitle = "DiscRescue"
	view.AltScreen = usesAltScreen(m.Page)
	return view
}

func renderPassRecoveryBody(m Model, width int, tier layoutTier) []string {
	return renderRecoveryDashboard(m, width, tier)
}

func scannedProgressBar(m Model, tier layoutTier) string {
	width := 40
	if tier == layoutMedium {
		width = 28
	}
	if tier == layoutCompact {
		width = 16
	}
	if m.Monochrome || tier == layoutCompact {
		if m.Recovery.TotalSectors == 0 {
			return "[" + strings.Repeat(".", width) + "]"
		}
		filled := int((m.Recovery.ScannedSectors * uint64(width)) / m.Recovery.TotalSectors)
		if filled > width {
			filled = width
		}
		return "[" + strings.Repeat("=", filled) + strings.Repeat(".", width-filled) + "]"
	}
	return renderUnicodeProgressBar(width, m.Recovery.ScannedSectors, m.Recovery.TotalSectors)
}
