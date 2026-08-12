package recovery

import (
	"fmt"
	"io"

	"discrescue/internal/mapfile"
)

func persistRecoveredRead(output recoverySyncWriter, store recoveryExtentStore, data []byte, offset int64, extent mapfile.Extent) error {
	if persistence, ok := store.(recoveryDataPersistence); ok {
		return persistence.PersistRecovered(data, offset, extent)
	}
	if err := writeFullAtWriter(output, data, offset); err != nil {
		return fmt.Errorf("write recovered data at byte %d: %w", offset, err)
	}
	if err := output.Sync(); err != nil {
		return fmt.Errorf("sync recovered data at byte %d: %w", offset, err)
	}
	if batched, ok := store.(recoveryBatchedStore); ok {
		if err := batched.StageExtent(extent); err != nil {
			return fmt.Errorf("persist recovered extent [%d,%d): %w", extent.StartLBA, extent.EndLBA(), err)
		}
		return nil
	}
	if err := store.ApplyExtent(extent); err != nil {
		return fmt.Errorf("persist recovered extent [%d,%d): %w", extent.StartLBA, extent.EndLBA(), err)
	}
	return nil
}
func writeFullAtWriter(writer io.WriterAt, data []byte, offset int64) error {
	written := 0
	for written < len(data) {
		n, err := writer.WriteAt(data[written:], offset+int64(written))
		written += n
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}
