package recovery

import (
	"fmt"
	"sync"
	"time"
)

// StopHooks are owned by the recovery job. They never perform TUI work.
type StopHooks struct {
	CancelRead    func() error
	ForceStop     func() error
	Checkpoint    func() error
	ReleaseDevice func() error
}

// StopController coordinates bounded cooperative cancellation and escalation.
// The controller never reports a terminal stop before checkpoint and device
// release hooks have both completed successfully.
type StopController struct {
	mu        sync.Mutex
	life      *Lifecycle
	hooks     StopHooks
	grace     time.Duration
	timer     *time.Timer
	finalized bool
}

func NewStopController(grace time.Duration, hooks StopHooks) (*StopController, error) {
	if grace <= 0 {
		return nil, fmt.Errorf("new stop controller: grace period must be greater than zero")
	}
	if hooks.Checkpoint == nil || hooks.ReleaseDevice == nil {
		return nil, fmt.Errorf("new stop controller: checkpoint and release hooks are required")
	}
	return &StopController{life: NewLifecycle(), hooks: hooks, grace: grace}, nil
}

func (c *StopController) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.life.Start()
}

func (c *StopController) BeginRead(requestID uint64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.life.BeginRead(requestID)
}

func (c *StopController) RequestStop(intent StopIntent) error {
	c.mu.Lock()
	if err := c.life.RequestStop(intent); err != nil {
		c.mu.Unlock()
		return err
	}
	state := c.life.State()
	if state == JobCancelingRead {
		c.timer = time.AfterFunc(c.grace, c.expireGrace)
	}
	c.mu.Unlock()
	if state == JobCancelingRead && c.hooks.CancelRead != nil {
		return c.hooks.CancelRead()
	}
	return c.finishIfCheckpointing()
}

func (c *StopController) expireGrace() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.life.State() == JobCancelingRead {
		_ = c.life.GraceExpired()
	}
}

// ReadFinished reports the active request's completion. A canceled read may
// be reported only once; stale completions are rejected by the lifecycle.
func (c *StopController) ReadFinished(requestID uint64) error {
	c.mu.Lock()
	if err := c.life.CompleteRead(requestID); err != nil {
		c.mu.Unlock()
		return err
	}
	if c.timer != nil {
		c.timer.Stop()
	}
	c.mu.Unlock()
	return c.finishIfCheckpointing()
}

func (c *StopController) ForceStop() error {
	c.mu.Lock()
	if err := c.life.ForceStop(); err != nil {
		c.mu.Unlock()
		return err
	}
	c.mu.Unlock()
	if c.hooks.ForceStop == nil {
		return fmt.Errorf("force stop: device cancellation hook is unavailable")
	}
	if err := c.hooks.ForceStop(); err != nil {
		c.mu.Lock()
		c.life.Fail()
		c.mu.Unlock()
		return fmt.Errorf("force stop active device request: %w", err)
	}
	return nil
}

func (c *StopController) finishIfCheckpointing() error {
	c.mu.Lock()
	if c.finalized || c.life.State() != JobCheckpointing {
		c.mu.Unlock()
		return nil
	}
	c.finalized = true
	c.mu.Unlock()
	if err := c.hooks.Checkpoint(); err != nil {
		c.mu.Lock()
		c.life.Fail()
		c.mu.Unlock()
		return fmt.Errorf("checkpoint stopped recovery: %w", err)
	}
	c.mu.Lock()
	if err := c.life.Checkpointed(); err != nil {
		c.life.Fail()
		c.mu.Unlock()
		return err
	}
	c.mu.Unlock()
	if err := c.hooks.ReleaseDevice(); err != nil {
		c.mu.Lock()
		c.life.Fail()
		c.mu.Unlock()
		return fmt.Errorf("release stopped recovery device: %w", err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.life.Released(false)
}

func (c *StopController) Snapshot() (JobState, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.life.State(), c.life.CanForceStop()
}
