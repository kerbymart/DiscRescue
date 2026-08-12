package recovery

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"time"
)

func readAtWithDeadline(ctx context.Context, source io.ReaderAt, buffer []byte, offset int64, deadline time.Duration) (int, error) {
	if deadline <= 0 {
		return source.ReadAt(buffer, offset)
	}
	readCtx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()
	if reader, ok := source.(ContextReaderAt); ok {
		return reader.ReadAtContext(readCtx, buffer, offset)
	}
	// Test and in-memory sources are already bounded by their implementation.
	// Native device adapters must implement ContextReaderAt so a timeout can
	// close and safely reopen the physical source instead of leaking a read.
	return source.ReadAt(buffer, offset)
}
func fatalRecoveryReadError(readErr error, classify ReadErrorClassifier) error {
	if readErr == nil {
		return nil
	}
	if errors.Is(readErr, ErrStopRequested) {
		return context.Canceled
	}
	if errors.Is(readErr, io.ErrClosedPipe) || errors.Is(readErr, io.ErrUnexpectedEOF) {
		return nil
	}
	if errors.Is(readErr, fs.ErrPermission) || errors.Is(readErr, fs.ErrNotExist) {
		return readErr
	}
	if classify != nil {
		return classify(readErr)
	}
	return nil
}
func readFailureDetail(action string, lba, sectors uint64, n, expected int, readErr error) string {
	rangeText := fmt.Sprintf("LBA %d-%d", lba, lba+sectors-1)
	if readErr != nil {
		return fmt.Sprintf("%s %s after read error: %v.", action, rangeText, readErr)
	}
	return fmt.Sprintf("%s %s after a short read (%d of %d bytes).", action, rangeText, n, expected)
}
func readSucceeded(n, expected int, err error) bool {
	return n == expected && (err == nil || errors.Is(err, io.EOF))
}
