package device

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScopedSourceAuditRejectsForbiddenOperationPaths(t *testing.T) {
	forbiddenFragments := []string{
		"format_unit",
		"blank_disc",
		"close_track",
		"reserve_track",
		"write_blocks",
		"controller_reset",
		"bus_reset",
		"scsi_reset",
		"unmount_media",
		"umount",
		"exec.command",
		"cmd.exe",
		"/bin/sh",
	}

	roots := []string{
		".",
		"linux",
		filepath.Join("..", "platform"),
		filepath.Join("..", "..", "cmd", "discrescue"),
	}

	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			t.Fatalf("read dir %q: %v", root, err)
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if filepath.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") {
				continue
			}

			path := filepath.Join(root, name)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read file %q: %v", path, err)
			}

			content := strings.ToLower(string(data))
			for _, fragment := range forbiddenFragments {
				// ADR 0009 records the sole process exception: macOS eject uses
				// a fixed, bounded Disk Utility request against the normalized
				// raw optical-device node. It has no user-controlled executable
				// or argument syntax.
				if fragment == "exec.command" && filepath.Clean(path) == filepath.Clean(filepath.Join("..", "platform", "darwin_native.go")) {
					continue
				}
				if strings.Contains(content, fragment) {
					t.Fatalf("forbidden operation fragment %q found in %s", fragment, path)
				}
			}
		}
	}
}
