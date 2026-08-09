package recovery

import "time"

const (
	RateWarmup     = 2 * time.Second
	RateStaleAfter = 15 * time.Second
)

type Clock interface{ Now() time.Time }

type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now() }

type SessionTelemetry struct {
	StartedAt           time.Time
	StartRecoveredBytes uint64
	RecoveredBytes      uint64
	RateBytesPerSecond  float64
	ETA                 time.Duration
	ETAKnown            bool
}

// TelemetryRecorder keeps cumulative progress separate from this session's
// work. Durable recovered bytes are the only input that advances the session.
type TelemetryRecorder struct {
	clock                 Clock
	sessionStartedAt      time.Time
	sessionStartRecovered uint64
	lastRecoveredAt       time.Time
	lastRecovered         uint64
}

func NewTelemetryRecorder(clock Clock, cumulativeRecovered uint64) *TelemetryRecorder {
	if clock == nil {
		clock = SystemClock{}
	}
	now := clock.Now()
	return &TelemetryRecorder{
		clock:                 clock,
		sessionStartedAt:      now,
		sessionStartRecovered: cumulativeRecovered,
		lastRecoveredAt:       now,
		lastRecovered:         cumulativeRecovered,
	}
}

func (t *TelemetryRecorder) Snapshot(cumulativeRecovered, totalBytes uint64, active bool) SessionTelemetry {
	now := t.clock.Now()
	if cumulativeRecovered < t.sessionStartRecovered {
		cumulativeRecovered = t.sessionStartRecovered
	}
	if cumulativeRecovered > t.lastRecovered {
		t.lastRecovered = cumulativeRecovered
		t.lastRecoveredAt = now
	}
	elapsed := now.Sub(t.sessionStartedAt)
	result := SessionTelemetry{
		StartedAt:           t.sessionStartedAt,
		StartRecoveredBytes: t.sessionStartRecovered,
		RecoveredBytes:      cumulativeRecovered - t.sessionStartRecovered,
	}
	if elapsed < RateWarmup || result.RecoveredBytes == 0 {
		return result
	}
	result.RateBytesPerSecond = float64(result.RecoveredBytes) / elapsed.Seconds()
	if active && now.Sub(t.lastRecoveredAt) <= RateStaleAfter && totalBytes >= cumulativeRecovered && result.RateBytesPerSecond > 0 {
		result.ETA = time.Duration(float64(totalBytes-cumulativeRecovered) / result.RateBytesPerSecond * float64(time.Second))
		result.ETAKnown = true
	}
	return result
}
