package app

import (
	tea "charm.land/bubbletea/v2"

	"discrescue/internal/platform"
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
	spinnerCmd := tea.Cmd(nil)
	if model.RestartLoadingSpinner {
		nextModel.RestartLoadingSpinner = false
		spinnerCmd = nextModel.LoadingSpinner.Tick
	}
	return nextModel, tea.Batch(nextModel.resolve(cmd), spinnerCmd, nextModel.followUp(msg))
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
