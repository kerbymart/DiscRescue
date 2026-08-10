//go:build !linux && !darwin && !windows

package platform

func platformFatalSourceReadError(error) bool { return false }
