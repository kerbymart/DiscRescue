//go:build linux

package platform

import (
	"errors"
	"syscall"
)

func platformFatalSourceReadError(err error) bool {
	return errors.Is(err, syscall.ENODEV)
}
