package devtool

import (
	"os"
	"runtime"
)

func raceSupported() bool {
	if os.Getenv("CGO_ENABLED") == "0" {
		return false
	}
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		return false
	}
	return runtime.GOARCH == "amd64" || runtime.GOARCH == "arm64"
}
