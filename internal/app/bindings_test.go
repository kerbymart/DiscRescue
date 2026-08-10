package app

import "testing"

func TestDefaultKeysAreDerivedFromAuthoritativeBindings(t *testing.T) {
	k := NewKeyMapV2()
	legacy := DefaultKeys()
	checks := []struct {
		name string
		got  []string
		want []string
	}{
		{"up", legacy.Up, k.Up.Keys()}, {"down", legacy.Down, k.Down.Keys()},
		{"select", legacy.Select, k.Select.Keys()}, {"back", legacy.Back, k.Back.Keys()},
		{"quit", legacy.Quit, k.Quit.Keys()}, {"details", legacy.Details, k.Details.Keys()},
		{"advanced", legacy.Advanced, k.Advanced.Keys()}, {"pause", legacy.Pause, k.Pause.Keys()},
		{"force", legacy.Force, k.Force.Keys()}, {"refresh", legacy.Refresh, k.Refresh.Keys()},
		{"eject", legacy.Eject, k.Eject.Keys()}, {"force eject", legacy.ForceEject, k.ForceEject.Keys()},
	}
	for _, check := range checks {
		if len(check.got) != len(check.want) {
			t.Errorf("%s keys: got %v want %v", check.name, check.got, check.want)
			continue
		}
		for i := range check.got {
			if check.got[i] != check.want[i] {
				t.Errorf("%s keys: got %v want %v", check.name, check.got, check.want)
				break
			}
		}
	}
}

func TestChooseDriveHelpMatchesImplementedActions(t *testing.T) {
	short := pageHelp(PageChooseDrive).ShortHelp()
	wanted := map[string]bool{"refresh drives": false, "eject": false, "force eject": false}
	for _, binding := range short {
		if _, ok := wanted[binding.Help().Desc]; ok {
			wanted[binding.Help().Desc] = true
		}
	}
	for description, found := range wanted {
		if !found {
			t.Errorf("choose-drive help does not expose %q", description)
		}
	}
}
