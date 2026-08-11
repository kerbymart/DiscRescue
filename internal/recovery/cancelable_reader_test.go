package recovery

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"
)

type blockingReader struct {
	closed    chan struct{}
	started   chan struct{}
	once      sync.Once
	startOnce sync.Once
}

func newBlockingReader() *blockingReader {
	return &blockingReader{closed: make(chan struct{}), started: make(chan struct{})}
}

func (r *blockingReader) ReadAt([]byte, int64) (int, error) {
	r.startOnce.Do(func() { close(r.started) })
	<-r.closed
	return 0, io.ErrClosedPipe
}

func (r *blockingReader) Close() error {
	r.once.Do(func() { close(r.closed) })
	return nil
}

type readyReader struct{}

func (readyReader) ReadAt(p []byte, _ int64) (int, error) {
	for i := range p {
		p[i] = byte(i)
	}
	return len(p), nil
}

type closableReadyReader struct {
	closed chan struct{}
	once   sync.Once
}

type stubbornReader struct {
	started   chan struct{}
	release   chan struct{}
	closed    chan struct{}
	startOnce sync.Once
	closeOnce sync.Once
}

func (r *stubbornReader) ReadAt([]byte, int64) (int, error) {
	r.startOnce.Do(func() { close(r.started) })
	<-r.release
	return 0, io.ErrClosedPipe
}

func (r *stubbornReader) Close() error {
	r.closeOnce.Do(func() { close(r.closed) })
	return nil
}

func (r *closableReadyReader) ReadAt(p []byte, _ int64) (int, error) {
	for i := range p {
		p[i] = byte(i)
	}
	return len(p), nil
}

func (r *closableReadyReader) Close() error {
	r.once.Do(func() { close(r.closed) })
	return nil
}

func TestReopenableReaderInterruptsAndReopensCanceledRead(t *testing.T) {
	first := newBlockingReader()
	var reopened *closableReadyReader
	reader, err := NewReopenableReaderAt(first, first.Close, func() (io.ReaderAt, error) {
		reopened = &closableReadyReader{closed: make(chan struct{})}
		return reopened, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	if _, err := reader.ReadAtContext(ctx, make([]byte, 4), 0); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("canceled read error = %v, want deadline exceeded", err)
	}
	if reopened == nil {
		t.Fatal("expected the source to reopen after the canceled read")
	}
	if n, err := reader.ReadAt(make([]byte, 4), 0); err != nil || n != 4 {
		t.Fatalf("read after reopen = n:%d err:%v, want four bytes", n, err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close reopened reader: %v", err)
	}
	select {
	case <-reopened.closed:
	default:
		t.Fatal("close did not close the current reopened source")
	}
}

func TestReopenableReaderInterruptsActiveReadWithoutJoiningOperationLock(t *testing.T) {
	first := &stubbornReader{
		started: make(chan struct{}), release: make(chan struct{}), closed: make(chan struct{}),
	}
	reader, err := NewReopenableReaderAt(first, first.Close, func() (io.ReaderAt, error) {
		return &readyReader{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		_, readErr := reader.ReadAt(make([]byte, 4), 0)
		done <- readErr
	}()

	select {
	case <-first.started:
	case <-time.After(time.Second):
		t.Fatal("active read did not start")
	}
	if err := reader.Interrupt(); err != nil {
		t.Fatalf("interrupt reader: %v", err)
	}
	select {
	case <-first.closed:
	case <-time.After(time.Second):
		t.Fatal("interrupt did not signal the active source")
	}
	select {
	case <-done:
		t.Fatal("interrupt waited for the blocked read to join")
	default:
	}
	close(first.release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("active read did not join after release")
	}
}

func TestReopenableReaderCanceledReadDoesNotReopenSource(t *testing.T) {
	first := newBlockingReader()
	var reopenCount int
	reader, err := NewReopenableReaderAt(first, first.Close, func() (io.ReaderAt, error) {
		reopenCount++
		return &readyReader{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, readErr := reader.ReadAtContext(ctx, make([]byte, 4), 0)
		done <- readErr
	}()
	select {
	case <-first.started:
	case <-time.After(time.Second):
		t.Fatal("active read did not start")
	}
	cancel()
	select {
	case readErr := <-done:
		if !errors.Is(readErr, context.Canceled) {
			t.Fatalf("canceled read error = %v, want context canceled", readErr)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled read did not finish")
	}
	if reopenCount != 0 {
		t.Fatalf("canceled read reopened source %d times, want zero", reopenCount)
	}
}
