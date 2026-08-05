package device

import "testing"

func TestSupervisorAcquireAndReleaseSingleDrive(t *testing.T) {
	supervisor := Supervisor{}

	next, ok := supervisor.AcquireDrive("/dev/sr0")
	if !ok {
		t.Fatal("expected first drive acquire to succeed")
	}
	if !next.CanOwnDrive("/dev/sr0") {
		t.Fatal("expected supervisor to own the acquired drive")
	}

	if _, ok := next.AcquireDrive("/dev/sr1"); ok {
		t.Fatal("expected second distinct drive acquire to fail")
	}

	released := next.ReleaseDrive("/dev/sr0")
	if !released.CanOwnDrive("/dev/sr1") {
		t.Fatal("expected released supervisor to allow a different drive")
	}
}
