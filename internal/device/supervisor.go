package device

type Supervisor struct {
	ActiveRequestID uint64
	OwnedDrive      string
}

func (s Supervisor) CanDispatch() bool {
	return s.ActiveRequestID == 0
}

func (s Supervisor) CanOwnDrive(drive string) bool {
	return s.OwnedDrive == "" || s.OwnedDrive == drive
}

func (s Supervisor) AcquireDrive(drive string) (Supervisor, bool) {
	if drive == "" || !s.CanOwnDrive(drive) {
		return s, false
	}
	next := s
	next.OwnedDrive = drive
	return next, true
}

func (s Supervisor) ReleaseDrive(drive string) Supervisor {
	if s.OwnedDrive != drive {
		return s
	}
	next := s
	next.OwnedDrive = ""
	return next
}
