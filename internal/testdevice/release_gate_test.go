package testdevice

import (
	"runtime"
	"testing"
)

func TestScenarioValidationSoak(t *testing.T) {
	scenarios := []Scenario{
		releaseGateScenario("healthy-soak", nil),
		releaseGateScenario("hung-soak", []FailureSpec{{
			Range:    Range{StartLBA: 8, Sectors: 1},
			Mode:     FailureHangWorker,
			Selector: AttemptSelector{Pass: PassScrape, AttemptMin: 1, AttemptMax: 1},
		}}),
	}

	for iteration := 0; iteration < 500; iteration++ {
		for _, scenario := range scenarios {
			if err := scenario.Validate(); err != nil {
				t.Fatalf("iteration %d validate scenario %q: %v", iteration, scenario.Name, err)
			}
		}
	}
}

func TestScenarioValidationNoGoroutineLeak(t *testing.T) {
	before := runtime.NumGoroutine()

	for iteration := 0; iteration < 200; iteration++ {
		scenario := releaseGateScenario("leak-check", []FailureSpec{{
			Range:    Range{StartLBA: 24, Sectors: 1},
			Mode:     FailureCrashWorker,
			Selector: AttemptSelector{Pass: PassAdaptive, AttemptMin: 1, AttemptMax: 1},
		}})
		if err := scenario.Validate(); err != nil {
			t.Fatalf("iteration %d validate scenario: %v", iteration, err)
		}
	}

	runtime.GC()
	after := runtime.NumGoroutine()
	if after > before+1 {
		t.Fatalf("unexpected goroutine growth: before=%d after=%d", before, after)
	}
}

func releaseGateScenario(name string, failures []FailureSpec) Scenario {
	return Scenario{
		Name: name,
		Media: Media{
			MediaID:           "disc-release-gate",
			Profile:           0x0010,
			LogicalSectorSize: 2048,
			SectorCount:       64,
			Sessions:          1,
			Tracks:            1,
			Data: []DataSpec{
				{Range: Range{StartLBA: 0, Sectors: 64}, Pattern: PatternLBA},
			},
		},
		Failures: failures,
	}
}
