package linux

import "fmt"

type IOCTLRequest struct {
	Name           string
	MaximumPayload uint32
}

func (r IOCTLRequest) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("validate ioctl request: name is required")
	}
	if r.MaximumPayload == 0 {
		return fmt.Errorf("validate ioctl request: maximum payload must be greater than zero")
	}
	if r.MaximumPayload > 1<<20 {
		return fmt.Errorf("validate ioctl request: maximum payload %d exceeds 1 MiB", r.MaximumPayload)
	}
	return nil
}
