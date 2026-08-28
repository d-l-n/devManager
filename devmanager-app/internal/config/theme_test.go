package config

import "testing"

func TestValidThemeAcceptsSystemPreference(t *testing.T) {
	if !validTheme("system") {
		t.Fatal("system theme preference must be accepted")
	}
}
