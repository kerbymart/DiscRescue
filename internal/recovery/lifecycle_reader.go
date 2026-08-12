package recovery

import (
	"context"
	"fmt"
	"io"
	"sync/atomic"
)

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
