package app

func renderFooter(m Model, width int, tier layoutTier) string {
	if m.Page == PageRecovering {
		theme := newTheme(m.Monochrome, m.DarkBackground)
		footer := theme.Key.Render("space") + " " + theme.Muted.Render("pause") + "  \u2022  " +
			theme.Key.Render("d") + " " + theme.Muted.Render("details") + "  \u2022  " +
			theme.Key.Render("q") + " " + theme.Muted.Render("stop")
		return fitToWidth(footer, width)
	}
	helpView := FooterHelp(tier == layoutFull)
	helpView.SetWidth(width)
	return fitToWidth(helpView.View(pageHelpForModel(m)), width)
}

func showStatusLine(page Page, tier layoutTier) bool {
	if tier == layoutCompact {
		return false
	}
	return page != PageDiscover && page != PageDetails
}

func usesAltScreen(_ Page) bool {
	// DiscRescue owns one interactive terminal session. Keep every page in the
	// alternate screen so discovery and setup do not leak into shell history.
	return true
}
func pageTitle(m Model) string {
	switch m.Page {
	case PageDiscover:
		return "Finding usable optical drives"
	case PageNoDrives:
		return "Drive discovery"
	case PageDiscoveryError:
		return "Drive discovery needs attention"
	case PageChooseDrive:
		return "Choose a drive"
	case PageInspectingMedia:
		return "Inspecting selected media"
	case PagePriorProcessing:
		return "Matching contents and local history"
	case PageChooseAction:
		return "What do you want to do?"
	case PageChooseOutput:
		return "Choose output"
	case PageReview:
		return "Review and start"
	case PageRecovering:
		if name := recoveryOutputName(m); name != "" {
			return "Recovering " + name
		}
		return "Recovery in progress"
	case PagePausing:
		if name := recoveryOutputName(m); name != "" {
			return "Pausing " + name
		}
		return "Pausing recovery"
	case PagePaused:
		if name := recoveryOutputName(m); name != "" {
			return "Recovery paused — " + name
		}
		return "Recovery paused"
	case PageStopConfirm:
		return "Stop recovery?"
	case PageEjectConfirm:
		return "Confirm force eject"
	case PageSummary:
		return "Recovery summary"
	case PageResumeJobs:
		return "Resume unfinished recovery"
	case PageHistory:
		return "Browse processed media"
	case PageDetails:
		return "Recovery details"
	case PageAdvanced:
		return "Advanced settings"
	case PageAbout:
		return "About DiscRescue"
	default:
		return "DiscRescue"
	}
}
