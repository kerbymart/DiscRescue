//go:build windows

package platform

import (
	"errors"

	"golang.org/x/sys/windows"
)

func platformFatalSourceReadError(err error) bool {
	return errors.Is(err, windows.ERROR_DEVICE_NOT_CONNECTED) ||
		errors.Is(err, windows.ERROR_FILE_NOT_FOUND) ||
		errors.Is(err, windows.ERROR_PATH_NOT_FOUND)
}
