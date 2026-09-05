package main

import "testing"

func TestNewAppExposesFreshServices(t *testing.T) {
	app := NewApp()
	if app.downloads == nil || app.capture == nil {
		t.Fatal("NewApp must initialize the application services")
	}
}
