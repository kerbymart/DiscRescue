package catalog

import (
	"context"
	"fmt"
	"os"
)

func readBoundedFile(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > maxCatalogFileBytes {
		return nil, fmt.Errorf("catalog file %s exceeds %d bytes", path, maxCatalogFileBytes)
	}
	return os.ReadFile(path)
}
func contextError(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}
