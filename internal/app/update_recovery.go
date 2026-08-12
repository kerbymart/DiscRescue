package app

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"discrescue/internal/platform"
)

func buildRecoveryDetails(m Model) []string {
	lines := []string{
		"Drive: " + firstNonEmpty(m.SelectedDrive.DisplayName, m.SelectedDrive.Path, "not selected"),
		"Media: " + firstNonEmpty(m.Identity.Detail, "not identified"),
		"Output: " + firstNonEmpty(m.Recovery.OutputPath, m.Setup.OutputPath),
		"Map: " + firstNonEmpty(m.Setup.ResumeMapPath, replaceExtension(firstNonEmpty(m.Recovery.OutputPath, m.Setup.OutputPath), ".drmap")),
		"Phase: " + firstNonEmpty(m.Recovery.Phase, "Waiting to start"),
		"Recovered sectors: " + formatCount(m.Recovery.RecoveredSectors),
		"Unreadable sectors: " + formatCount(m.Recovery.UnreadableSectors),
	}
	if m.Recovery.Remaining != "" {
		lines = append(lines, "Remaining: "+m.Recovery.Remaining)
	}
	if m.Recovery.Elapsed != "" {
		lines = append(lines, "Elapsed: "+m.Recovery.Elapsed)
	}
	if m.Recovery.ETA != "" {
		lines = append(lines, "ETA: "+m.Recovery.ETA)
	}
	if m.Recovery.Throughput != "" {
		lines = append(lines, "Rate: "+m.Recovery.Throughput)
	}
	lines = append(lines, "Status: "+recoveryDetailsStatusLine(m))
	lines = append(lines, m.Recovery.LastIssue...)
	return lines
}
func buildSummaryDetails(m Model, msg JobStoppedMsg) []string {
	lines := []string{
		"Drive: " + firstNonEmpty(m.SelectedDrive.DisplayName, m.SelectedDrive.Path, "not selected"),
		"Media: " + firstNonEmpty(m.Identity.Detail, "not identified"),
		"Image: " + msg.Summary.ImagePath,
	}
	if msg.Summary.MapPath != "" {
		lines = append(lines, "Map: "+msg.Summary.MapPath)
	}
	if msg.Err != nil {
		lines = append(lines, "Error: "+msg.Err.Error())
	} else if msg.Summary.NextAction != "" {
		lines = append(lines, "Next step: "+msg.Summary.NextAction)
	}
	return lines
}
func nextMethodLabel(current string) string {
	switch current {
	case "Balanced recovery":
		return "Fast recovery"
	case "Fast recovery":
		return "Gentle recovery"
	default:
		return "Balanced recovery"
	}
}
func nextRecoveryMethod(current platform.RecoveryMethod) (platform.RecoveryMethod, string) {
	switch current {
	case platform.RecoveryMethodBalanced:
		return platform.RecoveryMethodFast, "Fast recovery"
	case platform.RecoveryMethodFast:
		return platform.RecoveryMethodGentle, "Gentle recovery"
	default:
		return platform.RecoveryMethodBalanced, "Balanced recovery"
	}
}
func nextCopyLabel(current string) string {
	if current == "Not set (optional)" {
		return "Shelf B · Disc 14"
	}
	return "Not set (optional)"
}
func recoveryDetailsStatusLine(m Model) string {
	if m.Page == PagePausing || m.Recovery.PausePending {
		return "Pause requested. Waiting for the current drive request to finish safely."
	}
	if m.Page == PageSummary && m.Summary.NextAction != "" {
		return m.Summary.NextAction
	}
	return firstNonEmpty(m.Recovery.Status, "Waiting to start")
}
func summarySecondaryActionLabel(m Model) string {
	if m.Recovery.UnreadableSectors > 0 {
		return "Retry unreadable sectors"
	}
	return "Verify an existing image"
}
func describeTargetSpace(status platform.RecoveryTargetStatus) string {
	if !status.SpaceKnown {
		if status.RequiredBytes > 0 {
			return "Need " + humanBytes(status.RequiredBytes) + " for a full image; free space is unknown"
		}
		return "Free space is unknown for the selected target"
	}
	if status.RequiredBytes == 0 {
		return humanBytes(status.AvailableBytes) + " free on the selected drive"
	}
	if status.AvailableBytes >= status.RequiredBytes {
		return humanBytes(status.AvailableBytes) + " free; need " + humanBytes(status.RequiredBytes)
	}
	return humanBytes(status.AvailableBytes) + " free; need " + humanBytes(status.RequiredBytes) + " — choose another target"
}

func (m Model) updateJobStarted(msg JobStartedMsg) (tea.Model, tea.Cmd) {
	m.Page = PageRecovering
	m.Recovery.Status = msg.Status
	m.Recovery.OutputPath = msg.OutputPath
	m.Recovery.Phase = msg.Phase
	m.Recovery.TotalSectors = msg.TotalSectors
	m.Recovery.ScannedSectors = msg.ScannedSectors
	m.Recovery.RecoveredSectors = msg.RecoveredSectors
	m.Recovery.DeferredSectors = msg.DeferredSectors
	m.Recovery.UnreadableSectors = msg.UnreadableSectors
	m.Setup.RetryUnresolved = false
	m.Recovery.PausePending = false
	m.Recovery.StopPending = false
	m.Recovery.ForceStopAvailable = false
	m.Details.Lines = buildRecoveryDetails(m)
	m.Notice = nil
	return m, nil
}

func (m Model) updateJobPaused(msg JobPausedMsg) (tea.Model, tea.Cmd) {
	m.Page = PagePaused
	m.Recovery.Status = "Recovery paused"
	m.Recovery.OutputPath = msg.OutputPath
	m.Recovery.RecoveredSectors = msg.RecoveredSectors
	m.Recovery.TotalSectors = msg.TotalSectors
	m.Recovery.UnreadableSectors = msg.UnreadableSectors
	m.Recovery.PausePending = false
	m.Recovery.StopPending = false
	m.Recovery.ForceStopAvailable = false
	if msg.MapPath != "" {
		m.Setup.ResumeMapPath = msg.MapPath
	}
	m.Details.Lines = buildRecoveryDetails(m)
	m.Notice = &NoticeModel{Text: "Progress saved. Continue recovery when you are ready.", Severity: SeverityInfo}
	return m, nil
}

func (m Model) updateJobStartFailed(msg JobStartFailedMsg) (tea.Model, tea.Cmd) {
	m.Page = PageChooseOutput
	if msg.Err != nil {
		m.setErrorNotice(contextRecovery, msg.Err)
	}
	return m, nil
}

func (m Model) updateProgress(msg ProgressMsg) (tea.Model, tea.Cmd) {
	m.Recovery.Phase = msg.Snapshot.Phase
	m.Recovery.ScannedSectors = msg.Snapshot.ScannedSectors
	m.Recovery.RecoveredSectors = msg.Snapshot.RecoveredSectors
	m.Recovery.DeferredSectors = msg.Snapshot.DeferredSectors
	m.Recovery.TotalSectors = msg.Snapshot.TotalSectors
	m.Recovery.UnreadableSectors = msg.Snapshot.UnreadableSectors
	m.Recovery.Status = msg.Snapshot.Status
	m.Recovery.Elapsed = msg.Snapshot.Elapsed
	m.Recovery.Remaining = msg.Snapshot.Remaining
	m.Recovery.ETA = msg.Snapshot.ETA
	m.Recovery.Throughput = msg.Snapshot.Throughput
	m.Recovery.LastIssue = append([]string(nil), msg.Snapshot.LastIssue...)
	m.Recovery.PausePending = msg.Snapshot.PausePending
	m.Recovery.StopPending = msg.Snapshot.StopPending
	m.Recovery.ForceStopAvailable = msg.Snapshot.ForceStopAvailable
	if msg.Snapshot.OutputPath != "" {
		m.Recovery.OutputPath = msg.Snapshot.OutputPath
	}
	m.Details.Lines = buildRecoveryDetails(m)
	m.Notice = nil
	return m, nil
}

func (m Model) updateStatus(msg StatusMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		context := msg.Context
		if context == "" {
			context = contextRecovery
		}
		m.setErrorNotice(context, msg.Err)
		return m, nil
	}
	m.Notice = &NoticeModel{Text: msg.Text, Severity: msg.Severity}
	return m, nil
}

func (m Model) updateJobCheckpointed(msg JobCheckpointedMsg) (tea.Model, tea.Cmd) {
	m.Notice = &NoticeModel{Text: fmt.Sprintf("Progress saved at %s.", msg.At.Format(time.RFC822)), Severity: SeverityInfo}
	return m, nil
}

func (m Model) updateJobStopped(msg JobStoppedMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.setErrorNotice(contextRecovery, msg.Err)
	}
	m.Page = PageSummary
	m.Summary = msg.Summary
	m.Recovery.Status = msg.Summary.Outcome
	if msg.Summary.ImagePath != "" {
		m.Recovery.OutputPath = msg.Summary.ImagePath
	}
	m.Recovery.RecoveredSectors = msg.Summary.RecoveredSectors
	m.Recovery.TotalSectors = msg.Summary.TotalSectors
	m.Recovery.DeferredSectors = msg.Summary.DeferredSectors
	m.Recovery.UnreadableSectors = 0
	if msg.Summary.UnresolvedSectors > msg.Summary.DeferredSectors {
		m.Recovery.UnreadableSectors = msg.Summary.UnresolvedSectors - msg.Summary.DeferredSectors
	}
	m.Recovery.PausePending = false
	m.Recovery.StopPending = false
	m.Recovery.ForceStopAvailable = false
	m.Details.Lines = buildSummaryDetails(m, msg)
	m.Cursor = 0
	return m, nil
}

func (m Model) updateWorkerUnresponsive(msg WorkerUnresponsiveMsg) (tea.Model, tea.Cmd) {
	m.Notice = &NoticeModel{Text: fmt.Sprintf("Worker unresponsive for %s. Checkpoint requested.", msg.Since.Round(time.Second)), Severity: SeverityWarning}
	return m, nil
}

func (m Model) updateFatal(msg FatalMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.setErrorNotice(contextRecovery, msg.Err)
	}
	m.Page = PageSummary
	return m, nil
}
