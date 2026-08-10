package recovery

import (
	"context"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

var ErrStopRequested = fmt.Errorf("recovery stop requested")

// DefaultStopGracePeriod is the bounded cooperative window before the UI
// offers escalation for an active device request.
const DefaultStopGracePeriod = 5 * time.Second

// JobState is the monotonic lifecycle state of one recovery session.
type JobState string

const (
	JobStarting          JobState = "starting"
	JobRunning           JobState = "running"
	JobStopRequested     JobState = "stop_requested"
	JobCancelingRead     JobState = "canceling_read"
	JobAwaitingForceStop JobState = "awaiting_force_stop"
	JobForceStopping     JobState = "force_stopping"
	JobCheckpointing     JobState = "checkpointing"
	JobReleasingDevice   JobState = "releasing_device"
	JobStopped           JobState = "stopped"
	JobCompleted         JobState = "completed"
	JobFailed            JobState = "failed"
)

// StopIntent identifies why an active session is ending.
type StopIntent string

const (
	StopIntentPause StopIntent = "pause"
	StopIntentStop  StopIntent = "stop"
)

// Lifecycle is the synchronized state machine for one recovery session.
// It owns the scheduling gate: once a stop is requested, BeginRead rejects
// every new source request.
type Lifecycle struct {
	mu          sync.Mutex
	state       JobState
	stopIntent  StopIntent
	activeRead  bool
	canForce    bool
	lastRequest uint64
}

func NewLifecycle() *Lifecycle { return &Lifecycle{state: JobStarting} }

func (l *Lifecycle) State() JobState {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.state
}

func (l *Lifecycle) StopIntent() StopIntent {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.stopIntent
}

func (l *Lifecycle) CanForceStop() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.canForce
}

func (l *Lifecycle) Start() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.state != JobStarting {
		return fmt.Errorf("start lifecycle: state is %s", l.state)
	}
	l.state = JobRunning
	return nil
}

func (l *Lifecycle) BeginRead(requestID uint64) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if requestID == 0 {
		return fmt.Errorf("begin read: request ID is required")
	}
	if l.state != JobRunning {
		return fmt.Errorf("begin read: state is %s", l.state)
	}
	if l.activeRead {
		return fmt.Errorf("begin read: request %d is already active", l.lastRequest)
	}
	l.activeRead = true
	l.lastRequest = requestID
	return nil
}

func (l *Lifecycle) RequestStop(intent StopIntent) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if intent != StopIntentPause && intent != StopIntentStop {
		return fmt.Errorf("request stop: invalid intent %q", intent)
	}
	if l.state == JobStopped || l.state == JobCompleted || l.state == JobFailed {
		return fmt.Errorf("request stop: lifecycle is already terminal (%s)", l.state)
	}
	if l.state == JobStopRequested || l.state == JobCancelingRead || l.state == JobAwaitingForceStop || l.state == JobForceStopping {
		return nil
	}
	l.stopIntent = intent
	l.state = JobStopRequested
	if l.activeRead {
		l.state = JobCancelingRead
	} else {
		l.state = JobCheckpointing
	}
	return nil
}

func (l *Lifecycle) CompleteRead(requestID uint64) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.activeRead || requestID != l.lastRequest {
		return fmt.Errorf("complete read: request %d is not active", requestID)
	}
	l.activeRead = false
	if l.state == JobCancelingRead || l.state == JobForceStopping {
		l.state = JobCheckpointing
	} else if l.state == JobRunning {
		l.lastRequest = 0
	}
	return nil
}

func (l *Lifecycle) GraceExpired() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.state != JobCancelingRead {
		return fmt.Errorf("grace expired: state is %s", l.state)
	}
	l.state = JobAwaitingForceStop
	l.canForce = true
	return nil
}

func (l *Lifecycle) ForceStop() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.state != JobAwaitingForceStop || !l.canForce {
		return fmt.Errorf("force stop: escalation is not available in state %s", l.state)
	}
	l.state = JobForceStopping
	l.canForce = false
	return nil
}

func (l *Lifecycle) Checkpointed() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.state != JobCheckpointing && l.state != JobForceStopping {
		return fmt.Errorf("checkpointed: state is %s", l.state)
	}
	l.state = JobReleasingDevice
	return nil
}

// Complete enters the same checkpoint-and-release path used by a clean stop.
func (l *Lifecycle) Complete() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.state != JobRunning {
		return fmt.Errorf("complete lifecycle: state is %s", l.state)
	}
	l.state = JobCheckpointing
	return nil
}

func (l *Lifecycle) Released(completed bool) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.state != JobReleasingDevice {
		return fmt.Errorf("released: state is %s", l.state)
	}
	if completed {
		l.state = JobCompleted
	} else {
		l.state = JobStopped
	}
	return nil
}

func (l *Lifecycle) Fail() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.state, l.canForce = JobFailed, false
}

// LifecycleReaderAt applies the scheduling gate and read identity boundary to
// an existing source. It preserves the source's ReadAt behavior while making
// an active request visible to stop escalation.
type LifecycleReaderAt struct {
	Source    io.ReaderAt
	Lifecycle *Lifecycle
	nextID    atomic.Uint64
}

func (r *LifecycleReaderAt) ReadAt(p []byte, off int64) (int, error) {
	return r.ReadAtContext(context.Background(), p, off)
}

func (r *LifecycleReaderAt) ReadAtContext(ctx context.Context, p []byte, off int64) (int, error) {
	if r.Source == nil || r.Lifecycle == nil {
		return 0, fmt.Errorf("lifecycle reader: source and lifecycle are required")
	}
	id := r.nextID.Add(1)
	if err := r.Lifecycle.BeginRead(id); err != nil {
		return 0, ErrStopRequested
	}
	var n int
	var err error
	if source, ok := r.Source.(ContextReaderAt); ok {
		n, err = source.ReadAtContext(ctx, p, off)
	} else {
		n, err = r.Source.ReadAt(p, off)
	}
	if completeErr := r.Lifecycle.CompleteRead(id); err == nil && completeErr != nil {
		err = completeErr
	}
	return n, err
}
