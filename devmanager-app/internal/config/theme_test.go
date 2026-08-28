package config

import "testing"

func TestValidThemeAcceptsSystemPreference(t *testing.T) {
	if !validTheme("system") {
		t.Fatal("system theme preference must be accepted")
	}
}

func TestValidStyle(t *testing.T) {
	if !validStyle("standard") || !validStyle("brutalist") || validStyle("rounded") {
		t.Fatal("style validation mismatch")
	}
}
