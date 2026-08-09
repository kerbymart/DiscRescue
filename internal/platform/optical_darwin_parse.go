package platform

import (
	"fmt"
	"path/filepath"
	"strings"
)

func normalizeDarwinOpticalDevice(path string) (string, error) {
	path = strings.TrimSpace(path)
	if !strings.HasPrefix(path, "/dev/") {
		return "", fmt.Errorf("device path %q is not a Darwin disk device", path)
	}
	name := strings.TrimPrefix(path, "/dev/")
	if strings.HasPrefix(name, "r") {
		name = strings.TrimPrefix(name, "r")
	}
	if !strings.HasPrefix(name, "disk") || len(name) == len("disk") {
		return "", fmt.Errorf("device path %q is not a Darwin disk device", path)
	}
	for _, r := range strings.TrimPrefix(name, "disk") {
		if r < '0' || r > '9' {
			return "", fmt.Errorf("device path %q is not a whole Darwin disk", path)
		}
	}
	return filepath.Join("/dev", "r"+name), nil
}
