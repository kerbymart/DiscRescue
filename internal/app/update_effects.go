package app

import (
	tea "charm.land/bubbletea/v2"

	"discrescue/internal/device"
)

func discoverDevicesEffect(requestID int) tea.Cmd {
	return func() tea.Msg {
		return EffectRequestedMsg{Kind: EffectDiscoverDevices, RequestID: requestID}
	}
}
func identifyMediaEffect(devicePath string, requestID int) tea.Cmd {
	return func() tea.Msg {
		return EffectRequestedMsg{Kind: EffectIdentifyMedia, DevicePath: devicePath, RequestID: requestID}
	}
}
func ejectEffect(devicePath string, request device.EjectRequest) tea.Cmd {
	return func() tea.Msg {
		return EffectRequestedMsg{Kind: EffectEject, DevicePath: devicePath, Eject: request}
	}
}
func lookupHistoryEffect(basePath string, requestID int) tea.Cmd {
	return func() tea.Msg {
		return EffectRequestedMsg{Kind: EffectLookupHistory, BasePath: basePath, RequestID: requestID}
	}
}
func inspectTargetEffect(outputPath string, requestID int) tea.Cmd {
	return func() tea.Msg {
		return EffectRequestedMsg{Kind: EffectInspectTarget, OutputPath: outputPath, RequestID: requestID}
	}
}
func findResumeJobsEffect(basePath string, requestID int) tea.Cmd {
	return func() tea.Msg {
		return EffectRequestedMsg{Kind: EffectFindResumeJobs, BasePath: basePath, RequestID: requestID}
	}
}
func browseHistoryEffect(basePath string, requestID int) tea.Cmd {
	return func() tea.Msg {
		return EffectRequestedMsg{Kind: EffectBrowseHistory, BasePath: basePath, RequestID: requestID}
	}
}
func startJobEffect() tea.Cmd {
	return func() tea.Msg {
		return EffectRequestedMsg{Kind: EffectStartJob}
	}
}
func pauseJobEffect() tea.Cmd {
	return func() tea.Msg {
		return EffectRequestedMsg{Kind: EffectPauseJob}
	}
}
func resumeJobEffect() tea.Cmd {
	return func() tea.Msg {
		return EffectRequestedMsg{Kind: EffectResumeJob}
	}
}
func stopAfterCheckpointEffect() tea.Cmd {
	return func() tea.Msg {
		return EffectRequestedMsg{Kind: EffectStopJob}
	}
}
func stopImmediatelyEffect() tea.Cmd {
	return func() tea.Msg {
		return EffectRequestedMsg{Kind: EffectStopNow}
	}
}
