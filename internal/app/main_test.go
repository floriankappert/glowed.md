package app

import (
	"os"
	"testing"
)

// TestMain points HOME at a throwaway directory for the whole package. Model
// construction loads the global config, so without this a developer's own
// ~/.config/glowed/config.json would change what the tests see — and a test
// that saves a default would write into it.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "glowed-app-home-")
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
