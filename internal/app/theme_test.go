package app

import (
	"image/color"
	"reflect"
	"testing"

	lipgloss "charm.land/lipgloss/v2"
)

func TestWarningUsesCohesiveVioletRole(t *testing.T) {
	tests := []struct {
		name string
		dark bool
		want color.Color
	}{
		{name: "dark", dark: true, want: lipgloss.Color("#C4B5FD")},
		{name: "light", dark: false, want: lipgloss.Color("#7E22CE")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newTheme(false, tt.dark).Warning.GetForeground()
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("warning foreground = %#v, want %q", newTheme(false, tt.dark).Warning.GetForeground(), tt.want)
			}
		})
	}
}
