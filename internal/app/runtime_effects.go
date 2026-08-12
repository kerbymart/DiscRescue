package app

import tea "charm.land/bubbletea/v2"

func (m ProgramModel) runEffect(request EffectRequestedMsg) tea.Msg {
	switch request.Kind {
	case EffectDiscoverDevices, EffectIdentifyMedia, EffectEject:
		return m.runOpticalEffect(request)
	case EffectLookupHistory, EffectFindResumeJobs, EffectBrowseHistory:
		return m.runHistoryEffect(request)
	case EffectInspectTarget, EffectStartJob, EffectPauseJob, EffectResumeJob, EffectStopJob, EffectStopNow:
		return m.runRecoveryEffect(request)
	default:
		return unsupportedEffect(request)
	}
}
