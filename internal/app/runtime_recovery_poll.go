package app

import (
	"errors"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"discrescue/internal/catalog"
	"discrescue/internal/recovery"
)

func (m ProgramModel) followUp(msg tea.Msg) tea.Cmd {
	switch msg.(type) {
	case JobStartedMsg, JobPausedMsg, ProgressMsg, StatusMsg:
		if m.state != nil && m.state.activeRecovery != nil {
			return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
				activeRecovery := m.state.activeRecovery
				if activeRecovery == nil {
					return nil
				}
				snapshot := activeRecovery.Snapshot()
				totalSectors := m.MediaCapacitySectors
				logicalSectorSize := uint64(m.MediaLogicalSectorSize)
				if logicalSectorSize == 0 {
					logicalSectorSize = 2048
				}
				if snapshot.Done {
					m.state.activeRecovery = nil
					if m.state.pendingPause {
						m.state.pendingPause = false
						return JobPausedMsg{
							OutputPath:        m.Setup.OutputPath,
							MapPath:           snapshot.MapPath,
							RecoveredSectors:  snapshot.CopiedBytes / logicalSectorSize,
							TotalSectors:      totalSectors,
							UnreadableSectors: snapshot.UnreadableSectors,
						}
					}
					summary := JobSummary{
						ImagePath:         m.Setup.OutputPath,
						MapPath:           snapshot.MapPath,
						RecoveredSectors:  snapshot.CopiedBytes / logicalSectorSize,
						TotalSectors:      totalSectors,
						UnresolvedSectors: snapshot.DeferredSectors + snapshot.UnreadableSectors,
						DeferredSectors:   snapshot.DeferredSectors,
						Duration:          time.Since(snapshot.StartedAt).Round(time.Second).String(),
						CatalogStatus:     catalog.CatalogWriteStatus{State: catalog.CatalogWriteNotAttempted},
					}
					if snapshot.Canceled {
						summary.Outcome = "Recovery stopped"
						summary.NextAction = "The image and recovery map are safe to resume later."
					} else if snapshot.ErrText != "" {
						summary.Outcome = "Recovery failed"
						summary.NextAction = "Fix the reported problem or choose a different output target."
						return JobStoppedMsg{Summary: summary, Err: errors.New(snapshot.ErrText)}
					} else if snapshot.UnreadableSectors > 0 {
						summary.Outcome = "Recovery finished with unreadable sectors"
						summary.NextAction = "Retry unresolved sectors or inspect the saved recovery image."
					} else if snapshot.DeferredSectors > 0 {
						summary.Outcome = "Recovery finished with deferred sectors"
						summary.NextAction = "Retry deferred sectors with the saved recovery image and map."
					} else {
						summary.Outcome = "Recovery complete"
						summary.NextAction = "Recovery image is ready."
					}
					return JobStoppedMsg{Summary: summary}
				}
				phase := "Reading optical sectors"
				status := "Reading sectors from the selected optical drive."
				if snapshot.Resumed {
					phase = "Resuming optical recovery"
					status = "Continuing from the saved recovery map."
				}
				return ProgressMsg{
					Snapshot: ProgressSnapshot{
						Phase:              phase,
						RecoveredSectors:   snapshot.CopiedBytes / logicalSectorSize,
						ScannedSectors:     snapshot.ScannedSectors,
						DeferredSectors:    snapshot.DeferredSectors,
						TotalSectors:       totalSectors,
						UnreadableSectors:  snapshot.UnreadableSectors,
						Status:             status,
						Elapsed:            elapsedLabel(snapshot.StartedAt),
						Remaining:          humanBytes(snapshot.TotalBytes-snapshot.CopiedBytes) + " remaining",
						ETA:                telemetryETA(snapshot.Telemetry),
						Throughput:         telemetryThroughput(snapshot.Telemetry),
						LastIssue:          append([]string(nil), snapshot.LastIssue...),
						OutputPath:         m.Setup.OutputPath,
						PausePending:       m.state.pendingPause,
						StopPending:        snapshot.StopIntent == recovery.StopIntentStop,
						ForceStopAvailable: snapshot.CanForceStop,
					},
				}
			})
		}
	}
	return nil
}
func telemetryThroughput(telemetry recovery.SessionTelemetry) string {
	if telemetry.RateBytesPerSecond <= 0 {
		return ""
	}
	return humanBytes(uint64(telemetry.RateBytesPerSecond)) + "/s"
}
func elapsedLabel(startedAt time.Time) string {
	elapsed := time.Since(startedAt)
	if elapsed < time.Second {
		return "less than 1 second"
	}
	return elapsed.Round(time.Second).String()
}
func telemetryETA(telemetry recovery.SessionTelemetry) string {
	if !telemetry.ETAKnown || telemetry.ETA <= 0 {
		return ""
	}
	if telemetry.ETA < time.Second {
		return "less than 1 second left"
	}
	return "about " + telemetry.ETA.Round(time.Second).String() + " left"
}
func humanBytes(value uint64) string {
	const unit = 1024
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}
	div, exp := uint64(unit), 0
	for n := value / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(value)/float64(div), "KMGTPE"[exp])
}
