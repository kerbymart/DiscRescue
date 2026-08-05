package device

type Supervisor struct {
	ActiveRequestID uint64
}

func (s Supervisor) CanDispatch() bool {
	return s.ActiveRequestID == 0
}
