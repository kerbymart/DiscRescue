package logging

func RedactPath(path string) string {
	if path == "" {
		return ""
	}
	return "[redacted]"
}
