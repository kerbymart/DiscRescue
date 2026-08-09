package device

import "fmt"

type ReadSpeed struct{ KilobytesPerSecond uint32 }

type ReadSpeedMode string

const (
	ReadSpeedAuto     ReadSpeedMode = "auto"
	ReadSpeedExplicit ReadSpeedMode = "explicit"
)

type ReadSpeedRequest struct {
	Mode  ReadSpeedMode
	Speed ReadSpeed
}

func (r ReadSpeedRequest) Validate() error {
	if r.Mode != ReadSpeedAuto && r.Mode != ReadSpeedExplicit {
		return fmt.Errorf("validate read speed: unsupported mode %q", r.Mode)
	}
	if r.Mode == ReadSpeedExplicit && r.Speed.KilobytesPerSecond == 0 {
		return fmt.Errorf("validate read speed: explicit speed is required")
	}
	return nil
}

type ReadSpeedOption struct {
	Speed ReadSpeed
	Label string
}

type ReadSpeedOptions struct {
	Capability     Capability
	AutoAvailable  bool
	Speeds         []ReadSpeedOption
	CurrentPresent bool
	Current        ReadSpeed
}

type ReadSpeedResult struct {
	Requested        ReadSpeedRequest
	Applied          OperationStatus
	EffectivePresent bool
	Effective        ReadSpeed
	Restorable       bool
	Detail           string
}

func DefaultReadSpeedRequest() ReadSpeedRequest { return ReadSpeedRequest{Mode: ReadSpeedAuto} }
