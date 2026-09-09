package config

import (
	"os"
	"testing"
)

// TestMain keeps the package away from a real ~/.config/glowed/config.json,
// which Load reads and SaveDefaults writes.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "glowed-config-home-")
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
