package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"discrescue/internal/platform"
	"discrescue/internal/recovery"
)

type ProgramModel struct {
	Model
	runtime platform.Runtime
	state   *programState
}

type programState struct {
	activeRecovery platform.RecoveryJob
	activeJobID    string
	pendingPause   bool
}

func NewProgramModel(runtime platform.Runtime) ProgramModel {
	return ProgramModel{
		Model:   NewModel(),
		runtime: runtime,
		state:   &programState{},
	}
}

func (m ProgramModel) Init() tea.Cmd {
	return m.resolve(m.Model.Init())
}

func (m ProgramModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.Model.Update(msg)
	model := next.(Model)
	nextModel := ProgramModel{Model: model, runtime: m.runtime, state: m.state}
	return nextModel, tea.Batch(nextModel.resolve(cmd), nextModel.followUp(msg))
}

func (m ProgramModel) View() tea.View {
	return m.Model.View()
}

func (m ProgramModel) resolve(cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		msg := cmd()
		request, ok := msg.(EffectRequestedMsg)
		if !ok {
			return msg
		}
		return m.runEffect(request)
	}
}

func (m ProgramModel) runEffect(request EffectRequestedMsg) tea.Msg {
	switch request.Kind {
	case EffectDiscoverDevices:
		drives, err := m.runtime.Optical.DiscoverOpticalDrives()
		return DevicesDiscoveredMsg{
			RequestID: request.RequestID,
			Devices:   toDeviceSummaries(drives),
			Err:       err,
		}
	case EffectIdentifyMedia:
		media, err := m.runtime.Optical.IdentifyOpticalMedia(request.DevicePath)
		if err != nil {
			return MediaIdentifiedMsg{RequestID: request.RequestID, Err: err}
		}
		return MediaIdentifiedMsg{
			RequestID:           request.RequestID,
			Identity:            ContentIdentityViewModel{Summary: media.Summary, Detail: media.Detail},
			FileSystem:          media.FileSystem,
			VolumeLabel:         media.VolumeLabel,
			LogicalSectorSize:   media.LogicalSectorSize,
			CapacitySectors:     media.CapacitySectors,
			SuggestedOutputPath: media.SuggestedOutputPath,
			Recoverable:         media.Recoverable,
			RecoverabilityNote:  media.RecoverabilityNote,
		}
	case EffectLookupHistory:
		view, records, jobs, err := m.lookupPriorProcessing(request.BasePath)
		return PriorProcessingLookupMsg{
			RequestID: request.RequestID,
			View:      view,
			Records:   records,
			Jobs:      jobs,
			Err:       err,
		}
	case EffectInspectTarget:
		status, err := m.runtime.Recovery.InspectRecoveryTarget(platform.RecoveryInput{
			DevicePath:        m.SelectedDrive.Path,
			OutputPath:        request.OutputPath,
			LogicalSectorSize: m.MediaLogicalSectorSize,
			CapacitySectors:   m.MediaCapacitySectors,
		})
		return RecoveryTargetInspectedMsg{
			RequestID: request.RequestID,
			Status:    status,
			Err:       err,
		}
	case EffectFindResumeJobs:
		jobs, err := m.findResumableJobs(request.BasePath)
		return ResumableJobsDiscoveredMsg{
			RequestID: request.RequestID,
			Jobs:      jobs,
			Err:       err,
		}
	case EffectBrowseHistory:
		items, err := m.findProcessedMedia(request.BasePath)
		return ProcessedMediaDiscoveredMsg{
			RequestID: request.RequestID,
			Items:     items,
			Err:       err,
		}
	case EffectStartJob:
		return m.startRecoveryJob()
	case EffectPauseJob:
		if m.state.activeRecovery == nil {
			return StatusMsg{Text: "No active recovery job is available to pause.", Severity: SeverityWarning}
		}
		m.state.pendingPause = true
		if job, ok := m.state.activeRecovery.(platform.StoppableRecoveryJob); ok {
			if err := job.RequestStop(platform.StopIntentPause); err != nil {
				return StatusMsg{Text: "Could not pause recovery: " + err.Error(), Severity: SeverityWarning}
			}
			return StatusMsg{Text: "Pause requested; saving durable recovery state.", Severity: SeverityInfo}
		}
		m.state.activeRecovery.Cancel()
		return StatusMsg{Text: "Pause requested after the current read completes.", Severity: SeverityInfo}
	case EffectResumeJob:
		if m.state.activeRecovery != nil {
			return StatusMsg{Text: "Recovery is already running.", Severity: SeverityWarning}
		}
		return m.startRecoveryJob()
	case EffectStopJob:
		if m.state.activeRecovery == nil {
			return StatusMsg{Text: "No active recovery job is available to stop.", Severity: SeverityWarning}
		}
		m.state.pendingPause = false
		if job, ok := m.state.activeRecovery.(platform.StoppableRecoveryJob); ok {
			if err := job.RequestStop(platform.StopIntentStop); err != nil {
				return StatusMsg{Text: "Could not stop recovery: " + err.Error(), Severity: SeverityWarning}
			}
			return StatusMsg{Text: "Stop requested; saving durable recovery state.", Severity: SeverityWarning}
		}
		m.state.activeRecovery.Cancel()
		return StatusMsg{Text: "Stop requested after the current read completes.", Severity: SeverityWarning}
	case EffectStopNow:
		if m.state.activeRecovery == nil {
			return StatusMsg{Text: "No active recovery job is available to stop.", Severity: SeverityWarning}
		}
		job, ok := m.state.activeRecovery.(platform.StoppableRecoveryJob)
		if !ok || !job.Snapshot().CanForceStop {
			return StatusMsg{Text: "Force stop is unavailable until the recovery worker needs escalation.", Severity: SeverityWarning}
		}
		m.state.pendingPause = false
		if err := job.ForceStop(); err != nil {
			return StatusMsg{Text: "Could not force stop recovery: " + err.Error(), Severity: SeverityWarning}
		}
		return StatusMsg{Text: "Force-stopping the recovery worker; durable state is preserved.", Severity: SeverityWarning}
	default:
		return FatalMsg{Err: fmt.Errorf("unsupported effect: %s", request.Kind)}
	}
}

func (m ProgramModel) startRecoveryJob() tea.Msg {
	jobID := fmt.Sprintf("job-%d", time.Now().UnixNano())
	job, err := m.runtime.Recovery.StartImageRecovery(platform.RecoveryInput{
		DevicePath:        m.SelectedDrive.Path,
		OutputPath:        m.Setup.OutputPath,
		LogicalSectorSize: m.MediaLogicalSectorSize,
		CapacitySectors:   m.MediaCapacitySectors,
		Method:            m.Setup.Method,
	})
	if err != nil {
		return JobStartFailedMsg{Err: err}
	}
	m.state.activeRecovery = job
	m.state.activeJobID = jobID
	m.state.pendingPause = false
	logicalSectorSize := uint64(m.MediaLogicalSectorSize)
	if logicalSectorSize == 0 {
		logicalSectorSize = 2048
	}
	snapshot := job.Snapshot()
	phase := "Reading optical sectors"
	status := "Reading sectors from the selected optical drive."
	if snapshot.Resumed {
		phase = "Resuming optical recovery"
		status = "Resuming from the saved recovery map."
	}
	return JobStartedMsg{
		JobID:             jobID,
		OutputPath:        m.Setup.OutputPath,
		Phase:             phase,
		Status:            status,
		TotalSectors:      m.MediaCapacitySectors,
		RecoveredSectors:  snapshot.CopiedBytes / logicalSectorSize,
		UnreadableSectors: snapshot.UnreadableSectors,
	}
}

func (m ProgramModel) lookupPriorProcessing(basePath string) (PriorProcessingViewModel, []PriorProcessingRecord, []ResumableJobViewModel, error) {
	jobs, err := m.findResumableJobs(basePath)
	if err != nil {
		return PriorProcessingViewModel{}, nil, nil, err
	}
	if len(jobs) == 0 {
		return PriorProcessingViewModel{
			Kind:        PriorProcessingNone,
			HistoryLine: "No matching contents found on this computer.",
		}, nil, nil, nil
	}

	view := PriorProcessingViewModel{
		Kind:        PriorProcessingStrongResumable,
		Title:       "Matching contents were found on this computer",
		HistoryLine: fmt.Sprintf("Found %d resumable matching recoveries in %s.", len(jobs), strings.TrimSpace(basePath)),
		ImagePath:   jobs[0].OutputPath,
		Recovered:   formatCount(jobs[0].RecoveredSectors) + " sectors",
		Options: []string{
			"Resume the matching recovery",
			"Start a new recovery instead",
			"Choose another drive",
		},
	}
	if jobs[0].UnreadableSectors > 0 {
		view.UnreadableSectors = formatCount(jobs[0].UnreadableSectors) + " sectors"
	}
	view.Body = []string{
		"The current disc matches saved recovery work on this computer.",
		"Use Resume an unfinished recovery to continue from the saved map.",
	}
	records := make([]PriorProcessingRecord, 0, len(jobs))
	for _, job := range jobs {
		records = append(records, PriorProcessingRecord{
			Title:  filepath.Base(job.OutputPath),
			Detail: job.Detail,
		})
	}
	return view, records, jobs, nil
}

func (m ProgramModel) findResumableJobs(basePath string) ([]ResumableJobViewModel, error) {
	basePath = strings.TrimSpace(basePath)
	if basePath == "" {
		basePath = "."
	}
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return nil, fmt.Errorf("find resumable recoveries in %s: %w", basePath, err)
	}

	jobs := make([]ResumableJobViewModel, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.EqualFold(filepath.Ext(entry.Name()), ".iso") {
			continue
		}
		outputPath := filepath.Join(basePath, entry.Name())
		status, err := m.runtime.Recovery.InspectRecoveryTarget(platform.RecoveryInput{
			DevicePath:        m.SelectedDrive.Path,
			OutputPath:        outputPath,
			LogicalSectorSize: m.MediaLogicalSectorSize,
			CapacitySectors:   m.MediaCapacitySectors,
		})
		if err != nil || !status.CanResume {
			continue
		}
		jobs = append(jobs, ResumableJobViewModel{
			OutputPath:        outputPath,
			MapPath:           status.MapPath,
			RecoveredSectors:  status.RecoveredSectors,
			UnreadableSectors: status.UnreadableSectors,
			Detail:            status.Detail,
		})
	}

	sort.Slice(jobs, func(i, j int) bool {
		return strings.ToLower(jobs[i].OutputPath) < strings.ToLower(jobs[j].OutputPath)
	})
	return jobs, nil
}

func (m ProgramModel) findProcessedMedia(basePath string) ([]ProcessedMediaViewModel, error) {
	basePath = strings.TrimSpace(basePath)
	if basePath == "" {
		basePath = "."
	}
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return nil, fmt.Errorf("browse processed media in %s: %w", basePath, err)
	}

	items := make([]ProcessedMediaViewModel, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".iso") {
			continue
		}
		outputPath := filepath.Join(basePath, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("inspect processed media %s: %w", outputPath, err)
		}
		mapPath := replaceExtension(outputPath, ".drmap")
		item := ProcessedMediaViewModel{
			Title:      entry.Name(),
			ImagePath:  outputPath,
			ModifiedAt: info.ModTime().Local().Format("2006-01-02 15:04"),
			Status:     "Image only",
			Detail:     "No recovery map was found next to this image.",
		}

		status, inspectErr := m.runtime.Recovery.InspectRecoveryTarget(platform.RecoveryInput{
			DevicePath:        m.SelectedDrive.Path,
			OutputPath:        outputPath,
			LogicalSectorSize: m.MediaLogicalSectorSize,
			CapacitySectors:   m.MediaCapacitySectors,
		})
		switch {
		case inspectErr == nil && status.CanResume:
			item.MapPath = status.MapPath
			item.Status = "Resumable"
			item.Detail = status.Detail
		case fileExists(mapPath):
			item.MapPath = mapPath
			item.Status = "Saved with map"
			item.Detail = "A recovery map exists, but it does not match the currently selected disc."
		default:
			_ = status
		}

		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		return strings.ToLower(items[i].Title) < strings.ToLower(items[j].Title)
	})
	return items, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

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
						UnresolvedSectors: snapshot.UnreadableSectors,
						Duration:          time.Since(snapshot.StartedAt).Round(time.Second).String(),
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
						summary.NextAction = "Review unreadable sectors before deciding whether to retry."
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
						Phase:             phase,
						RecoveredSectors:  snapshot.CopiedBytes / logicalSectorSize,
						TotalSectors:      totalSectors,
						UnreadableSectors: snapshot.UnreadableSectors,
						Status:            status,
						Elapsed:           elapsedLabel(snapshot.StartedAt),
						Remaining:         humanBytes(snapshot.TotalBytes-snapshot.CopiedBytes) + " remaining",
						ETA:               telemetryETA(snapshot.Telemetry),
						Throughput:        telemetryThroughput(snapshot.Telemetry),
						LastIssue:         append([]string(nil), snapshot.LastIssue...),
						OutputPath:        m.Setup.OutputPath,
						PausePending:      m.state.pendingPause,
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

func toDeviceSummaries(drives []platform.OpticalDrive) []DeviceSummary {
	items := make([]DeviceSummary, 0, len(drives))
	seen := map[string]struct{}{}
	for _, drive := range drives {
		key := strings.TrimSpace(drive.Path)
		if key == "" {
			key = strings.TrimSpace(drive.DisplayName)
		}
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}

		displayName := strings.TrimSpace(drive.DisplayName)
		if displayName == "" {
			displayName = drive.Path
		}
		status := strings.TrimSpace(drive.Status)
		if status == "" {
			status = "available"
		}
		items = append(items, DeviceSummary{
			Path:        drive.Path,
			DisplayName: displayName,
			Status:      status,
		})
	}
	return items
}
