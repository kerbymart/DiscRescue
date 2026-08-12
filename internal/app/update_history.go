package app

import tea "charm.land/bubbletea/v2"

func defaultPriorProcessingView() PriorProcessingViewModel {
	return PriorProcessingViewModel{
		Kind:        PriorProcessingNone,
		HistoryLine: "Checking this computer for matching saved work.",
	}
}
func buildProcessedMediaDetails(item ProcessedMediaViewModel) []string {
	lines := []string{
		"Image: " + item.ImagePath,
		"Status: " + item.Status,
	}
	if item.MapPath != "" {
		lines = append(lines, "Map: "+item.MapPath)
	}
	if item.ModifiedAt != "" {
		lines = append(lines, "Updated: "+item.ModifiedAt)
	}
	if item.Detail != "" {
		lines = append(lines, "Notes: "+item.Detail)
	}
	return lines
}

func (m Model) updatePriorProcessingLookup(msg PriorProcessingLookupMsg) (tea.Model, tea.Cmd) {
	if msg.RequestID != m.ActiveLookupRequest {
		return m, nil
	}
	if msg.Err != nil {
		m.setErrorNotice(contextHistory, msg.Err)
		m.Page = PageChooseAction
		m.Cursor = 0
		return m, nil
	}
	if msg.View.HistoryLine != "" {
		m.PriorView = msg.View
	}
	m.PriorRecords = append([]PriorProcessingRecord(nil), msg.Records...)
	m.ResumeJobs = append([]ResumableJobViewModel(nil), msg.Jobs...)
	m.ResumeList.SetItems(resumeItems(m.ResumeJobs))
	resizeCompactLists(interactiveWidth(m.Width), maxInt(1, m.Height-11), &m.ResumeList)
	if msg.View.Kind == PriorProcessingStrongResumable && len(msg.Jobs) > 0 {
		m.Page = PagePriorProcessing
		m.Cursor = 0
		m.Notice = &NoticeModel{Text: "Matching recovery work was found on this computer.", Severity: SeverityInfo}
		return m, nil
	}
	return m, nil
}

func (m Model) updateProcessedMediaDiscovered(msg ProcessedMediaDiscoveredMsg) (tea.Model, tea.Cmd) {
	if msg.RequestID != m.ActiveHistoryRequest {
		return m, nil
	}
	if msg.Err != nil {
		m.HistoryItems = nil
		m.Page = PageChooseAction
		m.setErrorNotice(contextHistory, msg.Err)
		return m, nil
	}
	m.HistoryItems = append([]ProcessedMediaViewModel(nil), msg.Items...)
	m.HistoryList.SetItems(historyItems(m.HistoryItems))
	resizeCompactLists(interactiveWidth(m.Width), maxInt(1, m.Height-11), &m.HistoryList)
	m.Page = PageHistory
	m.Cursor = 0
	if len(m.HistoryItems) == 0 {
		m.Notice = &NoticeModel{Text: "No processed media were found in the current output folder.", Severity: SeverityInfo}
	} else {
		m.Notice = &NoticeModel{Text: "Browse saved images and recovery maps from the current output folder.", Severity: SeverityInfo}
	}
	return m, nil
}

func (m Model) updateResumableJobsDiscovered(msg ResumableJobsDiscoveredMsg) (tea.Model, tea.Cmd) {
	if msg.RequestID != m.ActiveResumeRequest {
		return m, nil
	}
	if msg.Err != nil {
		m.ResumeJobs = nil
		m.Page = PageChooseAction
		m.setErrorNotice(contextHistory, msg.Err)
		return m, nil
	}
	m.ResumeJobs = append([]ResumableJobViewModel(nil), msg.Jobs...)
	m.Page = PageResumeJobs
	m.Cursor = 0
	if len(m.ResumeJobs) == 0 {
		m.Notice = &NoticeModel{Text: "No resumable recoveries were found in the current output folder.", Severity: SeverityInfo}
	} else {
		m.Notice = &NoticeModel{Text: "Choose a saved recovery to resume safely.", Severity: SeverityInfo}
	}
	return m, nil
}
