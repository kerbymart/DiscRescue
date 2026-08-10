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
	closed chan struct{}
	once   sync.Once
}

func newBlockingReader() *blockingReader { return &blockingReader{closed: make(chan struct{})} }

func (r *blockingReader) ReadAt([]byte, int64) (int, error) {
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

func TestReopenableReaderInterruptsAndReopensCanceledRead(t *testing.T) {
	first := newBlockingReader()
	var reopened *readyReader
	reader, err := NewReopenableReaderAt(first, first.Close, func() (io.ReaderAt, error) {
		reopened = &readyReader{}
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
}
