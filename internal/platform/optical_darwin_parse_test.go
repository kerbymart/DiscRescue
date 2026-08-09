package platform

import "testing"

func TestNormalizeDarwinOpticalDevice(t *testing.T) {
	tests := []struct {
		path string
		want string
		ok   bool
	}{
		{"/dev/disk4", "/dev/rdisk4", true},
		{"/dev/rdisk4", "/dev/rdisk4", true},
		{"/dev/disk4s1", "", false},
		{"/dev/sr0", "", false},
	}
	for _, test := range tests {
		got, err := normalizeDarwinOpticalDevice(test.path)
		if test.ok && (err != nil || got != test.want) {
			t.Errorf("normalize %q: got %q, %v; want %q", test.path, got, err, test.want)
		}
		if !test.ok && err == nil {
			t.Errorf("normalize %q: expected error", test.path)
		}
	}
}
