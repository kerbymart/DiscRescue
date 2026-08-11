package recovery

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
)

// ContextReaderAt is implemented by device sources that can interrupt an
// active read when the context is canceled. Implementations must not return
// from ReadAtContext until the underlying request has stopped touching the
// source, so the scheduler never starts concurrent reads against one device.
type ContextReaderAt interface {
	ReadAtContext(context.Context, []byte, int64) (int, error)
}

// ReadInterruptor closes the currently active source request without waiting
// for the serialized reader operation to finish. Native recovery jobs use it
// for bounded force-stop escalation; the request still has to join through
// ReadAtContext before another read can begin.
type ReadInterruptor interface {
	Interrupt() error
}

// ReopenableReaderAt serializes source access and replaces a source after a
// canceled read. Platform adapters use it around an os.File so a timed-out
// raw-device request can be closed, fully joined, and reopened before the
// recovery scheduler advances to the next range.
type ReopenableReaderAt struct {
	opMu   sync.Mutex
	mu     sync.Mutex
	source io.ReaderAt
	close  func() error
	reopen func() (io.ReaderAt, error)
	closed bool
}

func NewReopenableReaderAt(source io.ReaderAt, close func() error, reopen func() (io.ReaderAt, error)) (*ReopenableReaderAt, error) {
	if source == nil || close == nil || reopen == nil {
		return nil, fmt.Errorf("new reopenable reader: source, close, and reopen are required")
	}
	return &ReopenableReaderAt{source: source, close: close, reopen: reopen}, nil
}

func (r *ReopenableReaderAt) ReadAt(p []byte, off int64) (int, error) {
	return r.ReadAtContext(context.Background(), p, off)
}

func (r *ReopenableReaderAt) ReadAtContext(ctx context.Context, p []byte, off int64) (int, error) {
	if ctx == nil {
		return 0, fmt.Errorf("reopenable reader: context is required")
	}
	r.opMu.Lock()
	defer r.opMu.Unlock()

	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return 0, io.ErrClosedPipe
	}
	source := r.source
	r.mu.Unlock()

	type result struct {
		n   int
		err error
	}
	resultCh := make(chan result, 1)
	go func() {
		n, err := source.ReadAt(p, off)
		resultCh <- result{n: n, err: err}
	}()

	select {
	case result := <-resultCh:
		return result.n, result.err
	case <-ctx.Done():
		// Closing is the platform-independent cancellation boundary exposed by
		// os.File. The caller waits for the read goroutine before reopening.
		_ = r.closeCurrent()
		<-resultCh
		if errors.Is(ctx.Err(), context.Canceled) {
			return 0, ctx.Err()
		}
		if err := r.reopenCurrent(); err != nil {
			return 0, fmt.Errorf("reopen source after timed-out read: %w", err)
		}
		return 0, ctx.Err()
	}
}

// Interrupt closes the current underlying source without taking opMu. This
// method is deliberately separate from Close: Close joins the active read and
// is therefore safe for normal shutdown, while force-stop must first signal
// the native descriptor without blocking the caller on that join.
func (r *ReopenableReaderAt) Interrupt() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return io.ErrClosedPipe
	}
	return closeReader(r.source, r.close)
}

func (r *ReopenableReaderAt) Close() error {
	r.mu.Lock()
	r.closed = true
	r.mu.Unlock()
	closeErr := r.closeCurrent()
	r.opMu.Lock()
	defer r.opMu.Unlock()
	return closeErr
}

func (r *ReopenableReaderAt) closeCurrent() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return closeReader(r.source, r.close)
}

func closeReader(source io.ReaderAt, fallback func() error) error {
	if source == nil {
		return nil
	}
	if closer, ok := source.(io.Closer); ok {
		return closer.Close()
	}
	return fallback()
}

func (r *ReopenableReaderAt) reopenCurrent() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return io.ErrClosedPipe
	}
	source, err := r.reopen()
	if err != nil {
		return err
	}
	r.source = source
	return nil
}
