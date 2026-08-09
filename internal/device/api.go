package device

import (
	"errors"
	"fmt"
	"time"
)

// DeviceID identifies a physical drive independently of its current path.
type DeviceID string

type DeviceRef struct {
	ID   DeviceID
	Path string
}

type IdentityStability string

const (
	IdentityStableHardware IdentityStability = "stable_hardware"
	IdentityStableSession  IdentityStability = "stable_session"
	IdentityPathBased      IdentityStability = "path_based"
)

type DeviceIdentity struct {
	Value     string
	Stability IdentityStability
}

type DevicePresence string

const (
	DevicePresent     DevicePresence = "present"
	DeviceUnavailable DevicePresence = "unavailable"
	DeviceRemoved     DevicePresence = "removed"
)

type MediaPresence string

const (
	MediaUnknown  MediaPresence = "unknown"
	MediaEmpty    MediaPresence = "empty"
	MediaPresent  MediaPresence = "present"
	MediaChanging MediaPresence = "changing"
)

type SupportStatus string

const (
	SupportUnknown     SupportStatus = "unknown"
	SupportSupported   SupportStatus = "supported"
	SupportUnsupported SupportStatus = "unsupported"
)

type Capability struct {
	Status            SupportStatus
	RequiresPrivilege bool
	Detail            string
}

type DriveCapabilities struct {
	MediaProbe     Capability
	RecoveryRead   Capability
	RawRead        Capability
	QueryReadSpeed Capability
	SetReadSpeed   Capability
	NormalEject    Capability
	ForceEject     Capability
	DeviceEvents   Capability
	MediaEvents    Capability
}

type DriveDescriptor struct {
	Ref          DeviceRef
	Identity     DeviceIdentity
	DisplayName  string
	Presence     DevicePresence
	Media        MediaPresence
	Capabilities DriveCapabilities
}

type DiscoverySnapshot struct {
	Generation uint64
	ObservedAt time.Time
	Drives     []DriveDescriptor
}

type MediaToken struct {
	Value      string
	Confidence string
}

type MediaObservation struct {
	Device            DeviceRef
	Presence          MediaPresence
	Token             MediaToken
	ObservedAt        time.Time
	LogicalSectorSize uint32
	CapacitySectors   uint64
}

type ErrorCode string

const (
	ErrorUnsupported      ErrorCode = "unsupported"
	ErrorPermissionDenied ErrorCode = "permission_denied"
	ErrorBusy             ErrorCode = "busy"
	ErrorNoMedia          ErrorCode = "no_media"
	ErrorDeviceRemoved    ErrorCode = "device_removed"
	ErrorMediaChanged     ErrorCode = "media_changed"
	ErrorTimeout          ErrorCode = "timeout"
	ErrorInvalidRequest   ErrorCode = "invalid_request"
	ErrorTransport        ErrorCode = "transport"
	ErrorProtocol         ErrorCode = "protocol"
	ErrorDeviceFailure    ErrorCode = "device_failure"
)

type OperationError struct {
	Code   ErrorCode
	Op     string
	Device DeviceRef
	Detail string
	Cause  error
}

func (e *OperationError) Error() string {
	if e == nil {
		return ""
	}
	if e.Detail != "" {
		return fmt.Sprintf("%s %s: %s", e.Op, e.Code, e.Detail)
	}
	return fmt.Sprintf("%s: %s", e.Op, e.Code)
}

func (e *OperationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func IsCode(err error, code ErrorCode) bool {
	var opErr *OperationError
	return errors.As(err, &opErr) && opErr.Code == code
}

type OperationStatus string

const (
	OperationCompleted OperationStatus = "completed"
	OperationAccepted  OperationStatus = "accepted"
	OperationPending   OperationStatus = "pending"
)

type DriveOwner string

const (
	DriveUnowned       DriveOwner = "unowned"
	DriveRecoveryOwned DriveOwner = "recovery"
	DriveCommandOwned  DriveOwner = "command"
)

// Validate ensures an observation cannot accidentally be used for another drive.
func (o MediaObservation) Validate() error {
	if o.Device.ID == "" || o.Device.Path == "" {
		return fmt.Errorf("validate media observation: device identity and path are required")
	}
	if o.Presence == MediaPresent && o.LogicalSectorSize == 0 {
		return fmt.Errorf("validate media observation: present media requires sector size")
	}
	return nil
}
