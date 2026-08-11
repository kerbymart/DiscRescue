package app

import (
	"errors"
	"strings"

	"discrescue/internal/device"
	"discrescue/internal/platform"
)

// UserMessageCode is the stable, presentation-facing classification for a
// notice. It intentionally does not expose native error strings or Go error
// types to the TUI.
type UserMessageCode string

const (
	MessageDiscoveryFailed    UserMessageCode = "discovery_failed"
	MessageMediaInspection    UserMessageCode = "media_inspection_failed"
	MessageHistoryUnavailable UserMessageCode = "history_unavailable"
	MessageTargetUnavailable  UserMessageCode = "target_unavailable"
	MessageRecoveryFailed     UserMessageCode = "recovery_failed"
	MessageEjectFailed        UserMessageCode = "eject_failed"
	MessageForceEjectFailed   UserMessageCode = "force_eject_failed"
	MessageOperationFailed    UserMessageCode = "operation_failed"
)

type messageContext string

const (
	contextDiscovery messageContext = "discovery"
	contextMedia     messageContext = "media"
	contextHistory   messageContext = "history"
	contextTarget    messageContext = "target"
	contextRecovery  messageContext = "recovery"
	contextEject     messageContext = "eject"
	contextForce     messageContext = "force_eject"
)

// errorNotice translates a technical failure once, at the boundary where it
// becomes user-facing. The complete original error remains available through
// TechnicalDetail and LastError for diagnostics.
func errorNotice(context messageContext, err error) NoticeModel {
	notice := NoticeModel{
		Code:            messageCode(context),
		Severity:        SeverityError,
		Text:            "Operation failed.",
		Explanation:     "DiscRescue could not complete the requested operation.",
		Action:          "Try again, or choose another available action.",
		TechnicalDetail: technicalErrorDetail(err),
	}

	if errors.Is(err, platform.ErrUnsupportedEnvironment) {
		notice.Code = MessageOperationFailed
		notice.Severity = SeverityWarning
		notice.Text = "This operation is unavailable here."
		notice.Explanation = "The current platform does not provide the required optical-drive support."
		notice.Action = "Use a supported Linux, macOS, or Windows optical environment."
		return notice
	}
	applyContextMessage(&notice, context)

	var operationErr *device.OperationError
	if errors.As(err, &operationErr) {
		notice.applyDeviceError(context, operationErr.Code)
		return notice
	}

	return notice
}

func applyContextMessage(notice *NoticeModel, context messageContext) {
	switch context {
	case contextDiscovery:
		notice.Text = "Drive discovery failed."
		notice.Explanation = "DiscRescue could not determine which optical drives are available."
		notice.Action = "Check the drive connection, then retry discovery."
	case contextMedia:
		notice.Text = "Disc inspection failed."
		notice.Explanation = "DiscRescue could not read enough information to identify this media."
		notice.Action = "Check that the disc is inserted and try again."
	case contextHistory:
		notice.Code = MessageHistoryUnavailable
		notice.Severity = SeverityWarning
		notice.Text = "Saved recovery history is unavailable."
		notice.Explanation = "The current recovery can still be configured without local history."
		notice.Action = "Continue, or choose a different output folder."
	case contextTarget:
		notice.Code = MessageTargetUnavailable
		notice.Text = "Output target cannot be used."
		notice.Explanation = "DiscRescue could not validate the selected image and recovery-map location."
		notice.Action = "Choose a writable output path and try again."
	case contextRecovery:
		notice.Code = MessageRecoveryFailed
		notice.Text = "Recovery stopped with an error."
		notice.Explanation = "The image and recovery map may contain the last durable checkpoint."
		notice.Action = "Open details, then resume or retry the unresolved sectors when safe."
	case contextEject:
		notice.Code = MessageEjectFailed
		notice.Text = "Normal eject failed."
		notice.Explanation = "The disc could not be ejected normally."
		notice.Action = "Close apps using the drive and try again, or review force eject."
	case contextForce:
		notice.Code = MessageForceEjectFailed
		notice.Text = "Force eject failed."
		notice.Explanation = "The operating system could not eject the disc."
		notice.Action = "Close apps using the drive, then retry normal eject."
	}
}

func messageCode(context messageContext) UserMessageCode {
	if context == contextForce {
		return MessageForceEjectFailed
	}
	if context == contextEject {
		return MessageEjectFailed
	}
	switch context {
	case contextDiscovery:
		return MessageDiscoveryFailed
	case contextMedia:
		return MessageMediaInspection
	case contextHistory:
		return MessageHistoryUnavailable
	case contextTarget:
		return MessageTargetUnavailable
	case contextRecovery:
		return MessageRecoveryFailed
	default:
		return MessageOperationFailed
	}
}

func (n *NoticeModel) applyDeviceError(context messageContext, code device.ErrorCode) {
	if context == contextForce {
		n.Code = MessageForceEjectFailed
		n.Text = "Force eject failed."
		n.Explanation = "The operating system could not eject the disc."
		n.Action = "Close apps using the drive, then retry normal eject."
	} else if context == contextEject {
		n.Code = MessageEjectFailed
		n.Text = "Normal eject failed."
		n.Explanation = "The disc could not be ejected normally."
		n.Action = "Close apps using the drive and try again, or review force eject."
	}

	switch code {
	case device.ErrorPermissionDenied:
		n.Text = "Access to the drive was denied."
		n.Explanation = "DiscRescue could not open or control the optical drive."
		n.Action = "Check permissions and close applications using the drive."
	case device.ErrorBusy:
		n.Text = "The optical drive is busy."
		n.Explanation = "Another application or an active recovery still has the drive in use."
		n.Action = "Stop recovery or close the other application, then try again."
	case device.ErrorNoMedia:
		n.Text = "No disc was detected."
		n.Explanation = "The selected optical drive does not currently contain readable media."
		n.Action = "Insert a CD or DVD, then retry."
	case device.ErrorDeviceRemoved:
		n.Text = "The optical drive is unavailable."
		n.Explanation = "The drive was removed or disconnected before the operation completed."
		n.Action = "Reconnect the drive and retry discovery."
	case device.ErrorMediaChanged:
		n.Text = "The disc changed during the operation."
		n.Explanation = "The selected media no longer matches the operation in progress."
		n.Action = "Insert the original disc and inspect it again."
	case device.ErrorTimeout:
		n.Text = "The drive request timed out."
		n.Explanation = "The optical drive did not respond within the allowed time."
		n.Action = "Wait for the drive to settle, then retry or review details."
	case device.ErrorUnsupported:
		n.Text = "This operation is not supported."
		n.Explanation = "The current adapter does not provide the requested optical operation."
		n.Action = "Use a supported operation or platform."
	case device.ErrorInvalidRequest:
		n.Text = "The request could not be accepted."
		n.Explanation = "The selected operation is not valid for the current recovery state."
		n.Action = "Return to the previous step and try again."
	}
}

func technicalErrorDetail(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (m *Model) setErrorNotice(context messageContext, err error) {
	m.LastError = err
	notice := errorNotice(context, err)
	m.Notice = &notice
	if notice.TechnicalDetail != "" {
		m.Details.Lines = []string{
			"Technical details",
			"",
			notice.TechnicalDetail,
		}
	}
}

func (m Model) noticeHasTechnicalDetail() bool {
	return m.Notice != nil && strings.TrimSpace(m.Notice.TechnicalDetail) != ""
}

func (m NoticeModel) String() string {
	parts := []string{m.Text}
	if m.Explanation != "" {
		parts = append(parts, m.Explanation)
	}
	if m.Action != "" {
		parts = append(parts, "Next: "+m.Action)
	}
	if m.TechnicalDetail != "" {
		parts = append(parts, "Press d for technical details.")
	}
	return strings.Join(parts, " ")
}
