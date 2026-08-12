package app

import (
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
)

func (m Model) handleOutputPathInput(msg tea.KeyPressMsg, key string) (tea.Model, tea.Cmd) {
	if m.Setup.OutputEditing {
		if (m.DirectoryInput.Value() == "" || m.DirectoryInput.Value() == ".") && m.Setup.OutputDirectory != "" && m.Setup.OutputDirectory != "." {
			m.DirectoryInput.SetValue(m.Setup.OutputDirectory)
		}
		if m.FileNameInput.Value() == "" && m.Setup.OutputFileName != "" {
			m.FileNameInput.SetValue(m.Setup.OutputFileName)
		}
		if m.Setup.ActiveOutputField == OutputFieldDirectory {
			m.DirectoryInput.Focus()
		} else {
			m.FileNameInput.Focus()
		}
	}
	switch {
	case matchesKey(key, DefaultKeys().Details) && !m.Setup.OutputEditing && m.noticeHasTechnicalDetail():
		m.PreviousPage = m.Page
		m.Page = PageDetails
		m.syncDetailsViewport()
		return m, nil
	case matchesKey(key, DefaultKeys().Back):
		if m.Setup.OutputEditing {
			m.Setup.OutputEditing = false
			m.blurOutputInputs()
			m.Notice = &NoticeModel{Text: "Stopped editing the output target.", Severity: SeverityInfo}
			return m, nil
		}
		return m.handleBack()
	case key == "tab":
		m.blurOutputInputs()
		if m.Setup.ActiveOutputField == OutputFieldDirectory {
			m.Setup.ActiveOutputField = OutputFieldFileName
		} else {
			m.Setup.ActiveOutputField = OutputFieldDirectory
		}
		if m.Setup.OutputEditing {
			if m.Setup.ActiveOutputField == OutputFieldDirectory {
				m.Cursor = 0
				m.DirectoryInput.Focus()
			} else {
				m.Cursor = 1
				m.FileNameInput.Focus()
			}
		}
		return m, nil
	case matchesKey(key, DefaultKeys().Up):
		if m.Setup.OutputEditing {
			return m, nil
		}
		m.moveCursor(-1)
		return m, nil
	case matchesKey(key, DefaultKeys().Down):
		if m.Setup.OutputEditing {
			return m, nil
		}
		m.moveCursor(1)
		return m, nil
	case matchesKey(key, DefaultKeys().Select):
		if !m.Setup.OutputEditing {
			return m.handleSelect()
		}
		m.syncOutputValues()
		m.Setup.OutputDirectory = strings.TrimSpace(m.Setup.OutputDirectory)
		m.Setup.OutputFileName = strings.TrimSpace(m.Setup.OutputFileName)
		syncOutputPath(&m.Setup)
		if m.Setup.OutputDirectory == "" {
			m.Notice = &NoticeModel{Text: "Choose an output folder before continuing.", Severity: SeverityWarning}
			return m, nil
		}
		if m.Setup.OutputFileName == "" {
			m.Notice = &NoticeModel{Text: "Choose an output file name before continuing.", Severity: SeverityWarning}
			return m, nil
		}
		m.Setup.OutputEditing = false
		m.blurOutputInputs()
		m.Notice = &NoticeModel{Text: "Finished editing the output target.", Severity: SeverityInfo}
		return m, nil
	default:
		if m.Setup.OutputEditing {
			var cmd tea.Cmd
			if m.Setup.ActiveOutputField == OutputFieldDirectory {
				m.DirectoryInput, cmd = m.DirectoryInput.Update(msg)
			} else {
				m.FileNameInput, cmd = m.FileNameInput.Update(msg)
			}
			m.syncOutputValues()
			clearResumeTargetState(&m.Setup)
			return m, cmd
		}
		return m, nil
	}
}
func (m *Model) syncOutputInputs() {
	m.DirectoryInput.SetValue(m.Setup.OutputDirectory)
	m.FileNameInput.SetValue(m.Setup.OutputFileName)
}
func (m *Model) syncOutputValues() {
	m.Setup.OutputDirectory = m.DirectoryInput.Value()
	m.Setup.OutputFileName = m.FileNameInput.Value()
	syncOutputPath(&m.Setup)
}
func (m *Model) blurOutputInputs() {
	m.DirectoryInput.Blur()
	m.FileNameInput.Blur()
}
func applyOutputPath(setup *JobSetupModel, fullPath string) {
	setup.OutputDirectory, setup.OutputFileName = splitOutputPath(fullPath)
	syncOutputPath(setup)
}
func splitOutputPath(fullPath string) (string, string) {
	fullPath = strings.TrimSpace(fullPath)
	if fullPath == "" || fullPath == "Not chosen yet" {
		return ".", ""
	}
	directory := filepath.Dir(fullPath)
	fileName := filepath.Base(fullPath)
	if directory == "" {
		directory = "."
	}
	if fileName == "." || fileName == string(filepath.Separator) {
		fileName = ""
	}
	return directory, fileName
}
func syncOutputPath(setup *JobSetupModel) {
	directory := strings.TrimSpace(setup.OutputDirectory)
	fileName := strings.TrimSpace(setup.OutputFileName)
	switch {
	case directory == "" && fileName == "":
		setup.OutputPath = "Not chosen yet"
	case directory == "":
		setup.OutputPath = fileName
	case fileName == "":
		setup.OutputPath = filepath.Clean(directory)
	default:
		setup.OutputPath = filepath.Join(directory, fileName)
	}
}
func clearResumeTargetState(setup *JobSetupModel) {
	setup.ResumeReady = false
	setup.ResumeMapPath = ""
	setup.ResumeDetail = ""
	setup.ActionLabel = "Start a new recovery"
	setup.FreeSpace = "Check the selected target to see free space and required size"
}
func trimLastOutputRune(setup *JobSetupModel) {
	switch setup.ActiveOutputField {
	case OutputFieldDirectory:
		setup.OutputDirectory = trimLastRune(setup.OutputDirectory)
	default:
		setup.OutputFileName = trimLastRune(setup.OutputFileName)
	}
}
func appendOutputText(setup *JobSetupModel, value string) {
	switch setup.ActiveOutputField {
	case OutputFieldDirectory:
		setup.OutputDirectory += value
	default:
		setup.OutputFileName += value
	}
}
func trimLastRune(value string) string {
	if value == "" {
		return value
	}
	runes := []rune(value)
	return string(runes[:len(runes)-1])
}
