//go:build darwin

// macos-eject-probe isolates macOS optical eject behavior from DiscRescue.
// It is intentionally opt-in: force mode ejects the supplied removable disk.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"syscall"
	"time"
)

const dkioEject = uintptr(0x20006415)

var wholeDiskPath = regexp.MustCompile(`^/dev/(?:r)?disk[0-9]+$`)

func main() {
	devicePath := flag.String("device", "", "whole macOS disk path, for example /dev/disk4")
	force := flag.Bool("force", false, "eject the raw device through macOS Disk Utility")
	flag.Parse()
	whole, err := normalizeWholeDisk(*devicePath)
	if err != nil {
		fatal(err)
	}
	if *force {
		err = forceEject(whole)
	} else {
		err = normalEject(whole)
	}
	if err != nil {
		fatal(err)
	}
	fmt.Printf("eject accepted for %s\n", whole)
}

func normalizeWholeDisk(path string) (string, error) {
	if !wholeDiskPath.MatchString(path) {
		return "", fmt.Errorf("device must be a whole /dev/diskN or /dev/rdiskN path, got %q", path)
	}
	return filepath.Join("/dev", "r"+regexp.MustCompile(`^r`).ReplaceAllString(filepath.Base(path), "")), nil
}

func normalEject(path string) error {
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), dkioEject, 0); errno != 0 {
		if errors.Is(errno, syscall.EBUSY) {
			return fmt.Errorf("normal DKIOCEJECT is busy; the volume is mounted or in use")
		}
		return fmt.Errorf("normal DKIOCEJECT: %w", errno)
	}
	return nil
}

func forceEject(path string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return runDiskutil(ctx, "eject", path)
}

func runDiskutil(ctx context.Context, arguments ...string) error {
	command := exec.CommandContext(ctx, "/usr/sbin/diskutil", arguments...)
	output, err := command.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("diskutil timed out: %w", ctx.Err())
		}
		return fmt.Errorf("diskutil %v: %w: %s", arguments, err, output)
	}
	return nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "macos-eject-probe:", err)
	os.Exit(1)
}
