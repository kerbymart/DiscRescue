//go:build windows

package platform

func identifyHostOpticalMedia(path string) (OpticalMedia, error) {
	return identifyWindowsOpticalMedia(path)
}
