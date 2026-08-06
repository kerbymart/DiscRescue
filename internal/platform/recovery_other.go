//go:build !windows

package platform

import "fmt"

type OSRecovery struct{}

func (OSRecovery) StartImageRecovery(input RecoveryInput) (RecoveryJob, error) {
	return nil, fmt.Errorf("real image recovery is not implemented for this platform")
}
