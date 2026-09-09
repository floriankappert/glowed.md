package main

import (
	"os"
	"testing"
)

// TestMain points HOME at a throwaway directory. These tests spawn the program
// with `go run`, and a subprocess inherits HOME — so without this a CLI test
// could read or write the developer's own ~/.config/glowed/config.json.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "glowed-cli-home-")
	if err != nil {
		panic(err)
	}
	if err := os.Setenv("HOME", home); err != nil {
		panic(err)
	}
	code := m.Run()
	_ = os.RemoveAll(home)
	os.Exit(code)
}
