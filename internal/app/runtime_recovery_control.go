package app

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"discrescue/internal/platform"
)

func (m ProgramModel) startRecoveryJob() tea.Msg {
	jobID := fmt.Sprintf("job-%d", time.Now().UnixNano())
	job, err := m.runtime.Recovery.StartImageRecovery(platform.RecoveryInput{
		DevicePath:        m.SelectedDrive.Path,
		OutputPath:        m.Setup.OutputPath,
		LogicalSectorSize: m.MediaLogicalSectorSize,
		CapacitySectors:   m.MediaCapacitySectors,
		Method:            m.Setup.Method,
		RetryUnresolved:   m.Setup.RetryUnresolved,
		ReadSpeed:         m.Setup.ReadSpeed,
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
		ScannedSectors:    snapshot.ScannedSectors,
		DeferredSectors:   snapshot.DeferredSectors,
		UnreadableSectors: snapshot.UnreadableSectors,
	}
}
