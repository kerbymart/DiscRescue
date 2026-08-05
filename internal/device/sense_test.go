package device

import "testing"

func TestParseFixedFormatSense(t *testing.T) {
	data := []byte{
		0x70, 0x00, 0x03, 0x00, 0x00, 0x00, 0x00,
		0x0a, 0x00, 0x00, 0x00, 0x00, 0x11, 0x00,
	}
	sense, err := ParseFixedFormatSense(data)
	if err != nil {
		t.Fatalf("parse fixed format sense: %v", err)
	}
	if sense.Key != 0x03 || sense.ASC != 0x11 || sense.ASCQ != 0x00 {
		t.Fatalf("unexpected sense tuple: %+v", sense)
	}
}

func TestClassifySenseRetryable(t *testing.T) {
	if got := ClassifySense(SenseTuple{Key: 0x02}); got != SenseRetryable {
		t.Fatalf("unexpected sense class: %s", got)
	}
}

func TestClassifySenseFatal(t *testing.T) {
	if got := ClassifySense(SenseTuple{Key: 0x03, ASC: 0x11, ASCQ: 0x00}); got != SenseFatal {
		t.Fatalf("unexpected sense class: %s", got)
	}
}
