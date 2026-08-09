package device

import "fmt"

// Ownership serializes recovery and control operations for one drive.
// Discovery and observation do not acquire ownership; media-affecting commands do.
type Ownership struct {
	owner DriveOwner
}

func (o *Ownership) Owner() DriveOwner {
	if o == nil || o.owner == "" {
		return DriveUnowned
	}
	return o.owner
}

func (o *Ownership) Acquire(owner DriveOwner) error {
	if owner != DriveRecoveryOwned && owner != DriveCommandOwned {
		return fmt.Errorf("acquire drive ownership: invalid owner %q", owner)
	}
	if current := o.Owner(); current != DriveUnowned {
		return &OperationError{Code: ErrorBusy, Op: "acquire drive ownership", Detail: string(current)}
	}
	o.owner = owner
	return nil
}

func (o *Ownership) Release(owner DriveOwner) {
	if o != nil && o.owner == owner {
		o.owner = DriveUnowned
	}
}

func (o *Ownership) ValidateEject(request EjectRequest) error {
	if err := request.Validate(); err != nil {
		return err
	}
	switch request.Mode {
	case EjectNormal:
		if o.Owner() != DriveUnowned {
			return &OperationError{Code: ErrorBusy, Op: "normal eject", Detail: "drive is owned"}
		}
	case EjectForce:
		if o.Owner() == DriveRecoveryOwned {
			return &OperationError{Code: ErrorBusy, Op: "force eject", Detail: "recovery must be stopped first"}
		}
	}
	return nil
}
