package recovery

import (
	"fmt"

	"discrescue/internal/mapfile"
)

type ScrapeSpeedProfile string

const (
	ScrapeSpeedAuto    ScrapeSpeedProfile = "auto"
	ScrapeSpeedMinimum ScrapeSpeedProfile = "minimum"
)

type ScrapeReadGroup string

const (
	ScrapeGroupImmediate  ScrapeReadGroup = "immediate"
	ScrapeGroupReverse    ScrapeReadGroup = "reverse"
	ScrapeGroupLowerSpeed ScrapeReadGroup = "lower_speed"
	ScrapeGroupReopen     ScrapeReadGroup = "reopen"
	ScrapeGroupReset      ScrapeReadGroup = "reset"
)

type ScrapeDecisionKind string

const (
	ScrapeDecisionRead             ScrapeDecisionKind = "read"
	ScrapeDecisionDelay            ScrapeDecisionKind = "delay"
	ScrapeDecisionSetSpeed         ScrapeDecisionKind = "set_speed"
	ScrapeDecisionReopen           ScrapeDecisionKind = "reopen"
	ScrapeDecisionReset            ScrapeDecisionKind = "reset"
	ScrapeDecisionWaitBackpressure ScrapeDecisionKind = "wait_backpressure"
	ScrapeDecisionDone             ScrapeDecisionKind = "done"
)

type ScrapeDecision struct {
	Kind      ScrapeDecisionKind
	Request   Request
	Reverse   bool
	Speed     ScrapeSpeedProfile
	SectorLBA uint64
}

type ScrapeSector struct {
	LBA           uint64
	Group         ScrapeReadGroup
	ReadsInGroup  uint8
	DelayPending  bool
	SpeedPending  bool
	ReopenPending bool
	ResetPending  bool
}

type ScrapePassState struct {
	Queue            []ScrapeSector
	DelayBudget      uint16
	SpeedBudget      uint16
	ReopenBudget     uint16
	ResetBudget      uint16
	ResetEnabled     bool
	SpeedSupported   bool
	CurrentSpeed     ScrapeSpeedProfile
	ActiveDecision   *ScrapeDecision
	RecoveredSectors []uint64
	FailedSectors    []uint64
}

type ScrapePassOutcome struct {
	SectorLBA uint64
	Success   bool
}

func StartScrapePass(extents []mapfile.Extent, delayBudget uint16, speedBudget uint16, reopenBudget uint16, resetBudget uint16, speedSupported bool, resetEnabled bool) (ScrapePassState, error) {
	if err := mapfile.ValidateExtentSet(extents); err != nil {
		return ScrapePassState{}, err
	}
	queue := make([]ScrapeSector, 0)
	for _, extent := range extents {
		if !schedulableScrapeState(extent.State) {
			continue
		}
		for lba := extent.StartLBA; lba < extent.EndLBA(); lba++ {
			queue = append(queue, ScrapeSector{
				LBA:   lba,
				Group: ScrapeGroupImmediate,
			})
		}
	}
	return ScrapePassState{
		Queue:          queue,
		DelayBudget:    delayBudget,
		SpeedBudget:    speedBudget,
		ReopenBudget:   reopenBudget,
		ResetBudget:    resetBudget,
		ResetEnabled:   resetEnabled,
		SpeedSupported: speedSupported,
		CurrentSpeed:   ScrapeSpeedAuto,
	}, nil
}

func DispatchScrapePass(state ScrapePassState, writerAvailable bool) (ScrapePassState, ScrapeDecision, error) {
	if state.ActiveDecision != nil {
		return state, ScrapeDecision{}, fmt.Errorf("dispatch scrape pass: previous action is still active")
	}
	if len(state.Queue) == 0 {
		return state, ScrapeDecision{Kind: ScrapeDecisionDone}, nil
	}

	sector := state.Queue[0]
	var decision ScrapeDecision
	switch {
	case sector.DelayPending:
		decision = ScrapeDecision{Kind: ScrapeDecisionDelay, SectorLBA: sector.LBA}
	case sector.SpeedPending:
		decision = ScrapeDecision{Kind: ScrapeDecisionSetSpeed, Speed: ScrapeSpeedMinimum, SectorLBA: sector.LBA}
	case sector.ReopenPending:
		decision = ScrapeDecision{Kind: ScrapeDecisionReopen, SectorLBA: sector.LBA}
	case sector.ResetPending:
		decision = ScrapeDecision{Kind: ScrapeDecisionReset, SectorLBA: sector.LBA}
	default:
		if !writerAvailable {
			return state, ScrapeDecision{Kind: ScrapeDecisionWaitBackpressure}, nil
		}
		decision = ScrapeDecision{
			Kind:      ScrapeDecisionRead,
			Request:   ScrapePass(sector.LBA, 1),
			Reverse:   sector.Group == ScrapeGroupReverse,
			SectorLBA: sector.LBA,
		}
	}

	next := state
	next.ActiveDecision = &decision
	return next, decision, nil
}

func ResolveScrapePass(state ScrapePassState, outcome ScrapePassOutcome) (ScrapePassState, error) {
	if state.ActiveDecision == nil {
		return state, fmt.Errorf("resolve scrape pass: no active action")
	}
	if len(state.Queue) == 0 {
		return state, fmt.Errorf("resolve scrape pass: no queued sectors")
	}

	decision := *state.ActiveDecision
	sector := state.Queue[0]
	if decision.SectorLBA != sector.LBA {
		return state, fmt.Errorf("resolve scrape pass: active sector %d does not match queue head %d", decision.SectorLBA, sector.LBA)
	}
	if outcome.SectorLBA != 0 && outcome.SectorLBA != sector.LBA {
		return state, fmt.Errorf("resolve scrape pass: outcome sector %d does not match queue head %d", outcome.SectorLBA, sector.LBA)
	}

	next := state
	next.ActiveDecision = nil
	next.Queue = append([]ScrapeSector(nil), state.Queue[1:]...)

	switch decision.Kind {
	case ScrapeDecisionDelay:
		if next.DelayBudget == 0 {
			return state, fmt.Errorf("resolve scrape pass: delay budget exhausted")
		}
		sector.DelayPending = false
		next.DelayBudget--
		next.Queue = append(next.Queue, sector)
		return next, nil

	case ScrapeDecisionSetSpeed:
		if next.SpeedBudget == 0 {
			return state, fmt.Errorf("resolve scrape pass: speed budget exhausted")
		}
		sector.SpeedPending = false
		next.SpeedBudget--
		next.CurrentSpeed = decision.Speed
		next.Queue = append(next.Queue, sector)
		return next, nil

	case ScrapeDecisionReopen:
		if next.ReopenBudget == 0 {
			return state, fmt.Errorf("resolve scrape pass: reopen budget exhausted")
		}
		sector.ReopenPending = false
		next.ReopenBudget--
		next.Queue = append(next.Queue, sector)
		return next, nil

	case ScrapeDecisionReset:
		if next.ResetBudget == 0 {
			return state, fmt.Errorf("resolve scrape pass: reset budget exhausted")
		}
		sector.ResetPending = false
		next.ResetBudget--
		next.Queue = append(next.Queue, sector)
		return next, nil

	case ScrapeDecisionRead:
		if outcome.Success {
			next.RecoveredSectors = append(next.RecoveredSectors, sector.LBA)
			return next, nil
		}
		advanceScrapeSector(&sector, &next)
		if sector.Group == "" {
			next.FailedSectors = append(next.FailedSectors, decision.SectorLBA)
			return next, nil
		}
		next.Queue = append(next.Queue, sector)
		return next, nil

	default:
		return state, fmt.Errorf("resolve scrape pass: unsupported decision %s", decision.Kind)
	}
}

func advanceScrapeSector(sector *ScrapeSector, state *ScrapePassState) {
	sector.ReadsInGroup++
	if sector.ReadsInGroup < scrapeReadsPerGroup(sector.Group) {
		if state.DelayBudget > 0 {
			sector.DelayPending = true
		}
		return
	}

	sector.ReadsInGroup = 0
	sector.DelayPending = false
	switch sector.Group {
	case ScrapeGroupImmediate:
		sector.Group = ScrapeGroupReverse
	case ScrapeGroupReverse:
		if state.SpeedSupported && state.SpeedBudget > 0 {
			sector.Group = ScrapeGroupLowerSpeed
			sector.SpeedPending = true
		} else if state.ReopenBudget > 0 {
			sector.Group = ScrapeGroupReopen
			sector.ReopenPending = true
		} else if state.ResetEnabled && state.ResetBudget > 0 {
			sector.Group = ScrapeGroupReset
			sector.ResetPending = true
		} else {
			sector.Group = ""
		}
	case ScrapeGroupLowerSpeed:
		if state.ReopenBudget > 0 {
			sector.Group = ScrapeGroupReopen
			sector.ReopenPending = true
		} else if state.ResetEnabled && state.ResetBudget > 0 {
			sector.Group = ScrapeGroupReset
			sector.ResetPending = true
		} else {
			sector.Group = ""
		}
	case ScrapeGroupReopen:
		if state.ResetEnabled && state.ResetBudget > 0 {
			sector.Group = ScrapeGroupReset
			sector.ResetPending = true
		} else {
			sector.Group = ""
		}
	case ScrapeGroupReset:
		sector.Group = ""
	default:
		sector.Group = ""
	}
}

func scrapeReadsPerGroup(group ScrapeReadGroup) uint8 {
	switch group {
	case ScrapeGroupImmediate, ScrapeGroupReverse, ScrapeGroupLowerSpeed:
		return 2
	case ScrapeGroupReopen, ScrapeGroupReset:
		return 1
	default:
		return 0
	}
}

func schedulableScrapeState(state mapfile.SectorState) bool {
	switch state {
	case mapfile.SectorStateUnknown, mapfile.SectorStateSkipped, mapfile.SectorStateIOError, mapfile.SectorStateMissing, mapfile.SectorStateChecksumError:
		return true
	default:
		return false
	}
}
