package app

import tea "charm.land/bubbletea/v2"

func (m ProgramModel) runHistoryEffect(request EffectRequestedMsg) tea.Msg {
	switch request.Kind {
	case EffectLookupHistory:
		view, records, jobs, err := m.lookupPriorProcessing(request.BasePath)
		return PriorProcessingLookupMsg{RequestID: request.RequestID, View: view, Records: records, Jobs: jobs, Err: err}
	case EffectFindResumeJobs:
		jobs, err := m.findResumableJobs(request.BasePath)
		return ResumableJobsDiscoveredMsg{RequestID: request.RequestID, Jobs: jobs, Err: err}
	case EffectBrowseHistory:
		items, err := m.findProcessedMedia(request.BasePath)
		return ProcessedMediaDiscoveredMsg{RequestID: request.RequestID, Items: items, Err: err}
	default:
		return unsupportedEffect(request)
	}
}
