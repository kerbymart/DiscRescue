package app

import "discrescue/internal/platform"

type RecoveryTargetInspectedMsg struct {
	RequestID int
	Status    platform.RecoveryTargetStatus
	Err       error
}
