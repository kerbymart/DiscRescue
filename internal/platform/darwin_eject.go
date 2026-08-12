//go:build darwin

package platform

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

func nativeDarwinEject(path string, force bool) error {
	whole, err := normalizeDarwinOpticalDevice(path)
	if err != nil {
		return err
	}
	if force {
		return nativeDarwinDiskutilEject(whole, "force")
	}
	return nativeDarwinDiskutilEject(whole, "normal")
}

// nativeDarwinDiskutilEject is the macOS eject request for a normalized raw
// optical device. Unlike DKIOCEJECT, Disk Utility coordinates a mounted-volume
// eject. Force mode reaches this same mechanism only after explicit UI
// confirmation; it is retained for platforms with a distinct escalation.
func nativeDarwinDiskutilEject(rawPath, mode string) error {
	ctx, cancel := context.WithTimeout(context.Background(), darwinForceEjectTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, "/usr/sbin/diskutil", "eject", rawPath)
	var diagnostics limitedDarwinEjectDiagnostics
	command.Stderr = &diagnostics
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("macOS %s eject timed out after %s: %w", mode, darwinForceEjectTimeout, ctx.Err())
		}
		if detail := strings.TrimSpace(diagnostics.String()); detail != "" {
			return fmt.Errorf("macOS %s eject: %w: %s", mode, err, detail)
		}
		return fmt.Errorf("macOS %s eject: %w", mode, err)
	}
	return nil
}

// limitedDarwinEjectDiagnostics preserves enough native utility context for
// the user without allowing an external command to retain unbounded output.
type limitedDarwinEjectDiagnostics struct{ bytes.Buffer }

func (b *limitedDarwinEjectDiagnostics) Write(value []byte) (int, error) {
	remaining := darwinEjectDiagnosticMax - b.Len()
	if remaining > 0 {
		if len(value) > remaining {
			_, _ = b.Buffer.Write(value[:remaining])
		} else {
			_, _ = b.Buffer.Write(value)
		}
	}
	return len(value), nil
}
