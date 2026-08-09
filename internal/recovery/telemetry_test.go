package recovery

import (
	"testing"
	"time"
)

type telemetryClock struct{ now time.Time }

func (c *telemetryClock) Now() time.Time { return c.now }

func TestTelemetryUsesSessionBaselineForResume(t *testing.T) {
	clock := &telemetryClock{now: time.Unix(100, 0)}
	recorder := NewTelemetryRecorder(clock, 1000)
	clock.now = clock.now.Add(10 * time.Second)
	telemetry := recorder.Snapshot(1000, 2000, true)
	if telemetry.RecoveredBytes != 0 || telemetry.RateBytesPerSecond != 0 || telemetry.ETAKnown {
		t.Fatalf("zero-work resume telemetry = %+v", telemetry)
	}
	clock.now = clock.now.Add(10 * time.Second)
	telemetry = recorder.Snapshot(1200, 2000, true)
	if telemetry.RecoveredBytes != 200 || telemetry.RateBytesPerSecond != 10 || !telemetry.ETAKnown {
		t.Fatalf("session telemetry = %+v", telemetry)
	}
}

func TestTelemetryHidesETAAfterStaleDurableProgress(t *testing.T) {
	clock := &telemetryClock{now: time.Unix(100, 0)}
	recorder := NewTelemetryRecorder(clock, 0)
	clock.now = clock.now.Add(3 * time.Second)
	if telemetry := recorder.Snapshot(300, 1000, true); !telemetry.ETAKnown {
		t.Fatalf("expected ETA after warmup: %+v", telemetry)
	}
	clock.now = clock.now.Add(RateStaleAfter + time.Second)
	telemetry := recorder.Snapshot(300, 1000, true)
	if telemetry.ETAKnown {
		t.Fatalf("expected stale ETA to be hidden: %+v", telemetry)
	}
}
