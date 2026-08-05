package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestViewGolden(t *testing.T) {
	testCases := []struct {
		name   string
		model  Model
		golden string
	}{
		{
			name: "80x24",
			model: Model{
				Screen:       ScreenWelcome,
				Width:        80,
				Height:       24,
				Notice:       "Simulator-first bootstrap is ready.",
				StatusLine:   "Press q to quit.",
				CurrentFocus: "start",
			},
			golden: "welcome_80x24.golden",
		},
		{
			name: "60x18",
			model: Model{
				Screen:       ScreenWelcome,
				Width:        60,
				Height:       18,
				Notice:       "Simulator-first bootstrap is ready.",
				StatusLine:   "Press q to quit.",
				CurrentFocus: "start",
			},
			golden: "welcome_60x18.golden",
		},
		{
			name: "40x12",
			model: Model{
				Screen:       ScreenWelcome,
				Width:        40,
				Height:       12,
				Notice:       "Simulator-first bootstrap is ready.",
				StatusLine:   "Press q to quit.",
				CurrentFocus: "start",
			},
			golden: "welcome_40x12.golden",
		},
		{
			name: "below-minimum",
			model: Model{
				Screen:       ScreenWelcome,
				Width:        39,
				Height:       11,
				Notice:       "Simulator-first bootstrap is ready.",
				StatusLine:   "Press q to quit.",
				CurrentFocus: "start",
			},
			golden: "too_small_39x11.golden",
		},
		{
			name: "monochrome",
			model: Model{
				Screen:       ScreenWelcome,
				Width:        80,
				Height:       24,
				Notice:       "Simulator-first bootstrap is ready.",
				StatusLine:   "Press q to quit.",
				CurrentFocus: "start",
				Monochrome:   true,
			},
			golden: "welcome_monochrome_80x24.golden",
		},
		{
			name: "long-path-wrap",
			model: Model{
				Screen:       ScreenWelcome,
				Width:        40,
				Height:       18,
				Notice:       "Output path D:/Projects/kerbymart/DiscRescue/archive/very/long/path/that/must/wrap/discrescue-image.iso is ready.",
				StatusLine:   "Press q to quit.",
				CurrentFocus: "start",
			},
			golden: "welcome_long_path_40x18.golden",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got := normalizeGolden(testCase.model.View().Content)
			want := normalizeGolden(readGolden(t, testCase.golden))
			if got != want {
				t.Fatalf("golden mismatch\n--- got ---\n%s--- want ---\n%s", got, want)
			}
		})
	}
}

func readGolden(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("testdata", name)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", name, err)
	}
	return string(content)
}

func normalizeGolden(content string) string {
	return strings.TrimSuffix(content, "\n")
}
