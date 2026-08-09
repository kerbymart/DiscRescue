package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"discrescue/internal/device"
	"discrescue/internal/platform"
)

type ejectingOpticalStub struct {
	result device.EjectResult
	err    error
}

func (s ejectingOpticalStub) DiscoverOpticalDrives() ([]platform.OpticalDrive, error) {
	return nil, nil
}
func (s ejectingOpticalStub) IdentifyOpticalMedia(string) (platform.OpticalMedia, error) {
	return platform.OpticalMedia{}, platform.ErrUnsupportedEnvironment
}
func (s ejectingOpticalStub) EjectOpticalDrive(string, device.EjectRequest) (device.EjectResult, error) {
	return s.result, s.err
}
func (s ejectingOpticalStub) EjectCapability(string) device.Capability {
	return device.Capability{Status: device.SupportSupported}
}

func TestForceEjectRequiresUserConfirmation(t *testing.T) {
	model := NewModel()
	model.Page = PageChooseDrive
	model.Devices = []DeviceSummary{{ID: "drive-a", Path: "/dev/sr0", DisplayName: "Drive"}}
	model.SelectedDrive = model.Devices[0]
	next, cmd := model.Update(tea.KeyPressMsg{Code: 'f', Text: "f"})
	updated := next.(Model)
	if cmd != nil || updated.Page != PageEjectConfirm || updated.PendingEject.Mode != device.EjectForce {
		t.Fatalf("updated page=%v pending=%+v cmd=%v", updated.Page, updated.PendingEject, cmd)
	}
}

func TestNormalEjectFailureOffersForceEject(t *testing.T) {
	model := NewModel()
	model.Page = PageChooseDrive
	model.SelectedDrive = DeviceSummary{ID: "drive-a", Path: "/dev/sr0", DisplayName: "Drive"}
	next, _ := model.Update(EjectCompletedMsg{Request: device.EjectRequest{Mode: device.EjectNormal}, Err: &device.OperationError{Code: device.ErrorBusy, Op: "eject"}})
	updated := next.(Model)
	if updated.Page != PageEjectConfirm || updated.PendingEject.Mode != device.EjectForce {
		t.Fatalf("page=%v pending=%+v", updated.Page, updated.PendingEject)
	}
}

func TestAcceptedEjectRefreshesDevices(t *testing.T) {
	model := NewModel()
	model.Page = PageChooseDrive
	model.SelectedDrive = DeviceSummary{ID: "drive-a", Path: "/dev/sr0"}
	next, cmd := model.Update(EjectCompletedMsg{
		Request: device.EjectRequest{Mode: device.EjectForce, ExplicitConfirm: true},
		Result:  device.EjectResult{Status: device.OperationAccepted, Verification: device.EjectAcceptedUnverified, Detail: "accepted"},
	})
	updated := next.(Model)
	if updated.Page != PageDiscover || cmd == nil {
		t.Fatalf("page=%v cmd=%v", updated.Page, cmd)
	}
	msg := cmd().(EffectRequestedMsg)
	if msg.Kind != EffectDiscoverDevices {
		t.Fatalf("effect=%+v", msg)
	}
}

func TestRuntimeEjectUsesPlatformAdapter(t *testing.T) {
	runtime := platform.Runtime{Optical: ejectingOpticalStub{result: device.EjectResult{Status: device.OperationAccepted}}}
	program := NewProgramModel(runtime)
	msg := program.runEffect(EffectRequestedMsg{Kind: EffectEject, DevicePath: "/dev/sr0", Eject: device.EjectRequest{Mode: device.EjectNormal}})
	result, ok := msg.(EjectCompletedMsg)
	if !ok || result.Err != nil || result.Result.Status != device.OperationAccepted {
		t.Fatalf("message=%#v", msg)
	}
}
