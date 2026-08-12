package app

// renderCompactPageBody dispatches only compact-layout decisions. The page
// files own the actual compact content so a screen's responsive behavior stays
// next to its full and medium layouts.
func renderCompactPageBody(m Model, width int, tier layoutTier) ([]string, bool) {
	if m.Page == PageEjectConfirm && m.Height < 28 {
		return renderCompactEjectConfirmPage(m, width)
	}
	if tier != layoutCompact && m.Height >= 20 {
		return nil, false
	}

	switch m.Page {
	case PageDiscover, PageNoDrives, PageDiscoveryError:
		return renderCompactDrivePage(m, width)
	case PageInspectingMedia, PagePriorProcessing, PageChooseAction, PageAdvanced:
		return renderCompactMediaPage(m, width)
	case PageChooseOutput, PageReview:
		return renderCompactOutputPage(m, width)
	case PagePausing, PagePaused, PageStopConfirm:
		return renderCompactRecoveryPage(m, width)
	case PageDetails:
		return renderCompactDetailsPage(m, width)
	case PageAbout:
		return renderCompactAboutPage(m, width)
	default:
		return nil, false
	}
}
