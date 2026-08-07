package app

import (
	"bytes"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"discrescue/internal/platform"
)

type stubOptical struct {
	drives []platform.OpticalDrive
	err    error
}

type synchronizedBuffer struct {
	mu sync.Mutex
	bytes.Buffer
}

func (b *synchronizedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Buffer.Write(p)
}

func (b *synchronizedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Buffer.String()
}

func (s stubOptical) DiscoverOpticalDrives() ([]platform.OpticalDrive, error) {
	return append([]platform.OpticalDrive(nil), s.drives...), s.err
}

func (s stubOptical) IdentifyOpticalMedia(path string) (platform.OpticalMedia, error) {
	for _, drive := range s.drives {
		if drive.Path == path {
			return platform.OpticalMedia{
				Summary: "Media inspection completed.",
				Detail:  drive.DisplayName + " - " + drive.Path,
			}, nil
		}
	}
	return platform.OpticalMedia{}, platform.ErrUnsupportedEnvironment
}

type stubClock struct{}

func (stubClock) Now() time.Time { return time.Date(2026, time.August, 6, 7, 0, 0, 0, time.UTC) }

type stubRecoveryJob struct {
	snapshot platform.RecoverySnapshot
}

func (s stubRecoveryJob) Snapshot() platform.RecoverySnapshot {
	return s.snapshot
}

func (stubRecoveryJob) Cancel() {}

func TestProgramModelStartupWorkflowLeavesDiscoveryAndSelectsDrive(t *testing.T) {
	runtime := platform.Runtime{
		Clock: stubClock{},
		Optical: stubOptical{drives: []platform.OpticalDrive{
			{Path: "D:\\", DisplayName: "Virtual DVD Drive", Status: "available"},
			{Path: "E:\\", DisplayName: "Physical Blu-ray Drive", Status: "available"},
		}},
	}

	inputReader, inputWriter := io.Pipe()
	var output synchronizedBuffer

	program := tea.NewProgram(
		NewProgramModel(runtime),
		tea.WithInput(inputReader),
		tea.WithOutput(&output),
		tea.WithWindowSize(80, 24),
		tea.WithoutSignals(),
	)

	done := make(chan struct{})
	go func() {
		sentEnter := false
		for {
			time.Sleep(time.Millisecond)
			text := output.String()
			if !sentEnter && strings.Contains(text, "Choose a drive") {
				_, _ = inputWriter.Write([]byte("\r"))
				sentEnter = true
			}
			if sentEnter && strings.Contains(text, "What do you want to do?") {
				program.Quit()
				close(done)
				return
			}
		}
	}()

	if _, err := program.Run(); err != nil {
		t.Fatalf("run program: %v", err)
	}

	<-done
	rendered := output.String()
	if !strings.Contains(rendered, "Choose a drive") {
		t.Fatalf("expected startup workflow to render the drive list, got %q", rendered)
	}
	if !strings.Contains(rendered, "Virtual DVD Drive") {
		t.Fatalf("expected discovered drive in output, got %q", rendered)
	}
	if !strings.Contains(rendered, "What do you want to do?") {
		t.Fatalf("expected actionable next step after drive selection, got %q", rendered)
	}
}

func TestProgramModelDiscoveryCanReachNoDriveState(t *testing.T) {
	runtime := platform.Runtime{
		Clock:   stubClock{},
		Optical: stubOptical{},
	}

	model := NewProgramModel(runtime)
	msg := model.Init()()
	next, _ := model.Update(msg)
	updated := next.(ProgramModel)

	if updated.Page != PageNoDrives {
		t.Fatalf("unexpected page: got %v want %v", updated.Page, PageNoDrives)
	}
	if updated.Notice == nil || updated.Notice.Text != "No usable optical drives found." {
		t.Fatalf("unexpected notice: %+v", updated.Notice)
	}
}

func TestFollowUpIgnoresStaleRecoveryTickAfterPauseCompletes(t *testing.T) {
	model := NewProgramModel(platform.Runtime{})
	model.state.activeRecovery = stubRecoveryJob{
		snapshot: platform.RecoverySnapshot{
			StartedAt:  time.Now(),
			TotalBytes: 2048,
		},
	}

	cmd := model.followUp(StatusMsg{Text: "Pausing recovery after the current read completes.", Severity: SeverityInfo})
	if cmd == nil {
		t.Fatal("expected follow-up tick command")
	}

	model.state.activeRecovery = nil

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("follow-up tick panicked after recovery was cleared: %v", recovered)
		}
	}()

	if msg := cmd(); msg != nil {
		t.Fatalf("expected stale tick to return no message, got %#v", msg)
	}
}
