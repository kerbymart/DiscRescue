package device

import (
	"bytes"
	"testing"
)

func TestReadSpeedDefaultsToAuto(t *testing.T) {
	got := DefaultReadSpeedRequest()
	if got.Mode != ReadSpeedAuto {
		t.Fatalf("got %+v", got)
	}
	encoded, err := MarshalSetSpeedRequest(got)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded, make([]byte, 8)) {
		t.Fatalf("auto payload must be zeroed: %x", encoded)
	}
	decoded, err := UnmarshalSetSpeedRequest(encoded)
	if err != nil || decoded.Mode != ReadSpeedAuto {
		t.Fatalf("decoded %+v, err %v", decoded, err)
	}
}

func TestExplicitReadSpeedRequiresBoundedValue(t *testing.T) {
	if _, err := MarshalSetSpeedRequest(ReadSpeedRequest{Mode: ReadSpeedExplicit}); err == nil {
		t.Fatal("expected explicit speed validation")
	}
	want := ReadSpeedRequest{Mode: ReadSpeedExplicit, Speed: ReadSpeed{KilobytesPerSecond: 1764}}
	encoded, err := MarshalSetSpeedRequest(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := UnmarshalSetSpeedRequest(encoded)
	if err != nil || got != want {
		t.Fatalf("got %+v, err %v", got, err)
	}
}
