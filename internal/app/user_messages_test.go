package app

import (
	"errors"
	"strings"
	"testing"

	"discrescue/internal/device"
)

func TestErrorNoticeTranslatesStableDeviceCodeAndPreservesCause(t *testing.T) {
	err := &device.OperationError{
		Code:   device.ErrorBusy,
		Op:     "native macOS optical eject",
		Detail: "resource busy: exit status 1",
	}

	notice := errorNotice(contextForce, err)
	if notice.Code != MessageForceEjectFailed {
		t.Fatalf("code = %q, want %q", notice.Code, MessageForceEjectFailed)
	}
	if notice.Severity != SeverityError {
		t.Fatalf("severity = %q, want %q", notice.Severity, SeverityError)
	}
	if notice.Text != "The optical drive is busy." {
		t.Fatalf("text = %q", notice.Text)
	}
	if notice.TechnicalDetail != err.Error() {
		t.Fatalf("technical detail = %q, want %q", notice.TechnicalDetail, err.Error())
	}
	if strings.Contains(notice.String(), "device_failure") || strings.Contains(notice.String(), "exit status") {
		t.Fatalf("technical terminology leaked into primary message: %q", notice.String())
	}
}

func TestErrorNoticeUsesFriendlyFallbackForUnknownError(t *testing.T) {
	err := errors.New("native driver exploded with errno 999")
	notice := errorNotice(contextMedia, err)
	if notice.Text != "Disc inspection failed." {
		t.Fatalf("text = %q", notice.Text)
	}
	if notice.TechnicalDetail != err.Error() {
		t.Fatalf("technical detail = %q", notice.TechnicalDetail)
	}
	if strings.Contains(notice.String(), "errno 999") {
		t.Fatal("unknown technical error leaked into primary message")
	}
}

func TestSetErrorNoticeMakesTechnicalDetailsExplicit(t *testing.T) {
	model := NewModel()
	err := errors.New("open /dev/rdisk4: permission denied")
	model.setErrorNotice(contextDiscovery, err)

	if model.Notice == nil || model.Notice.TechnicalDetail != err.Error() {
		t.Fatalf("notice = %+v", model.Notice)
	}
	if !strings.Contains(strings.Join(model.Details.Lines, "\n"), err.Error()) {
		t.Fatalf("details do not preserve original error: %v", model.Details.Lines)
	}
}

func TestNoDriveViewDoesNotRepeatEquivalentStateMessages(t *testing.T) {
	model := NewModel()
	model.Page = PageNoDrives
	model.Width = 80
	model.Height = 24
	view := model.View().Content
	if strings.Count(strings.ToLower(view), "no optical drives found") > 1 {
		t.Fatalf("repeated no-drive state: %q", view)
	}
	if strings.Contains(view, "No usable optical drives found.") && strings.Contains(view, "No optical drives found") {
		t.Fatal("no-drive state is repeated in the primary body and status region")
	}
}
