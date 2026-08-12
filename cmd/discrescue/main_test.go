package main

import (
	"bytes"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"discrescue/internal/app"
	"discrescue/internal/platform"
)

type testClock struct{}

func (testClock) Now() time.Time { return time.Date(2026, time.August, 6, 7, 0, 0, 0, time.UTC) }

func TestNewProgramUsesExecutableCompositionPath(t *testing.T) {
	runtime := platform.NewRuntime()
	runtime.Clock = testClock{}
	var output bytes.Buffer

	program := newProgram(app.Services{Optical: runtime.Optical, Recovery: runtime.Recovery}, &output,
		tea.WithInput(bytes.NewBufferString("q")),
		tea.WithWindowSize(80, 24),
		tea.WithoutSignals(),
	)

	if program == nil {
		t.Fatal("expected program instance")
	}
}
