package clipboard

import (
	"encoding/base64"
	"fmt"
	"os/exec"
	"runtime"
)

func Copy(text string) error {
	if runtime.GOOS == "darwin" {
		cmd := exec.Command("pbcopy")
		w, err := cmd.StdinPipe()
		if err != nil {
			return err
		}
		if err := cmd.Start(); err != nil {
			return err
		}
		_, _ = w.Write([]byte(text))
		_ = w.Close()
		return cmd.Wait()
	}
	return fmt.Errorf("clipboard copy is not configured for %s; OSC52 fallback should be emitted by the app", runtime.GOOS)
}

func Paste() (string, error) {
	if runtime.GOOS == "darwin" {
		out, err := exec.Command("pbpaste").Output()
		return string(out), err
	}
	return "", fmt.Errorf("clipboard paste is not configured for %s", runtime.GOOS)
}

func OSC52(text string) string {
	return "\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte(text)) + "\x07"
}
