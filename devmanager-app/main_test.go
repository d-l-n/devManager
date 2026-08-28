package main

import (
	"testing"

	"github.com/wailsapp/wails/v2/pkg/options"
)

func TestSingleInstanceLockConfiguration(t *testing.T) {
	app := &App{}
	lock := singleInstanceLock(app)

	if lock == nil || lock.UniqueId == "" || lock.OnSecondInstanceLaunch == nil {
		t.Fatal("single-instance lock must define an identifier and second-launch handler")
	}

	lock.OnSecondInstanceLaunch(options.SecondInstanceData{})
	if app.windowHidden {
		t.Fatal("second launch must focus the existing instance")
	}
}
