package device

import "fmt"

type EjectMode string

const (
	EjectNormal EjectMode = "normal"
	EjectForce  EjectMode = "force"
)

type EjectRequest struct {
	Mode            EjectMode
	ExplicitConfirm bool
}

func (r EjectRequest) Validate() error {
	if r.Mode != EjectNormal && r.Mode != EjectForce {
		return fmt.Errorf("validate eject: unsupported mode %q", r.Mode)
	}
	if r.Mode == EjectForce && !r.ExplicitConfirm {
		return fmt.Errorf("validate eject: force eject requires explicit confirmation")
	}
	return nil
}

type EjectVerification string

const (
	EjectConfirmed          EjectVerification = "confirmed"
	EjectAcceptedUnverified EjectVerification = "accepted_unverified"
	EjectStillPresent       EjectVerification = "still_present"
)

type EjectResult struct {
	Requested    EjectRequest
	Status       OperationStatus
	Verification EjectVerification
	Detail       string
}
