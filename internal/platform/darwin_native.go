//go:build darwin

package platform

import (
	"time"
)

const (
	// DKIOCGETBLOCKSIZE and DKIOCGETBLOCKCOUNT are Darwin disk ioctls.
	dkioGetBlockSize  = uintptr(0x40046418)
	dkioGetBlockCount = uintptr(0x40086419)
	// DKIOCEJECT is the public Darwin ioctl for ejecting removable media.
	dkioEject = uintptr(0x20006415)

	darwinForceEjectTimeout  = 10 * time.Second
	darwinEjectDiagnosticMax = 4 << 10
)

type darwinNativeDrive struct {
	Path              string
	DisplayName       string
	LogicalSectorSize uint32
	CapacityBytes     uint64
	RegistryID        uint64
	Media             bool
	State             MediaProbeState
}
