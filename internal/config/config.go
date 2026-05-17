package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Prefix     string            `json:"prefix"`
	Keys       map[string]string `json:"keys"`
	PrefixKeys map[string]string `json:"prefixKeys"`
	Preview    PreviewConfig     `json:"preview"`
	Footer     FooterConfig      `json:"footer"`
	Scan       ScanConfig        `json:"scan"`
	Mouse      MouseConfig       `json:"mouse"`
	LLM        LLMConfig         `json:"llm"`
}

type PreviewConfig struct {
	Style            string `json:"style"`
	PreserveNewLines bool   `json:"preserveNewLines"`
}

type FooterConfig struct {
	Actions []string `json:"actions"`
}

type ScanConfig struct {
	ExcludeDirs  []string `json:"excludeDirs"`
	MaxFileBytes int64    `json:"maxFileBytes"`
}

type MouseConfig struct {
	Enabled bool `json:"enabled"`
}

type LLMConfig struct {
	Enabled            bool   `json:"enabled"`
	Command            string `json:"command"`
	TerminalCommand    string `json:"terminalCommand"`
	TerminalApp        string `json:"terminalApp"`
	OpenMode           string `json:"openMode"`
	MaxContextBytes    int    `json:"maxContextBytes"`
	IncludeCurrentFile bool   `json:"includeCurrentFile"`
	IncludeSelection   bool   `json:"includeSelection"`
}

func Default() Config {
	return Config{
		Prefix: "ctrl+g",
		Keys: map[string]string{
			"quit":          "q",
			"search":        "/",
			"edit":          "e",
			"sourceSelect":  "v",
			"openLLM":       "c",
			"toggleSidebar": "b",
			"save":          "ctrl+s",
			"undo":          "ctrl+z",
			"redo":          "ctrl+y",
			"refresh":       "r",
			"nextFocus":     "tab",
			"open":          "enter",
		},
		PrefixKeys: map[string]string{
			"toggleSidebar": "b",
			"openLLM":       "l",
			"refresh":       "r",
			"quit":          "q",
		},
		Preview: PreviewConfig{Style: "dark", PreserveNewLines: true},
		Footer:  FooterConfig{Actions: []string{"search", "edit", "sourceSelect", "openLLM", "toggleSidebar", "refresh", "quit"}},
		Scan: ScanConfig{
			ExcludeDirs:  []string{".git", "node_modules", "dist", "build", ".next", "target", "vendor"},
			MaxFileBytes: 1024 * 1024,
		},
		Mouse: MouseConfig{Enabled: true},
		LLM: LLMConfig{
			Enabled:            true,
			Command:            "claude",
			TerminalCommand:    "ghostty",
			TerminalApp:        "Ghostty",
			OpenMode:           "split-right",
			MaxContextBytes:    20 * 1024,
			IncludeCurrentFile: false,
			IncludeSelection:   true,
		},
	}
}

func Load(root string) (Config, []error) {
	cfg := Default()
	var errs []error

	paths := []string{
		filepath.Join(os.Getenv("HOME"), ".config", "glowed", "config.json"),
		filepath.Join(root, ".glowed.json"),
	}

	for _, p := range paths {
		if p == "" {
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			if !os.IsNotExist(err) {
				errs = append(errs, fmt.Errorf("read %s: %w", p, err))
			}
			continue
		}
		if err := mergeJSON(&cfg, b); err != nil {
			errs = append(errs, fmt.Errorf("parse %s: %w", p, err))
		}
	}

	normalize(&cfg)
	return cfg, errs
}

func mergeJSON(cfg *Config, b []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}

	if v, ok := raw["prefix"]; ok {
		_ = json.Unmarshal(v, &cfg.Prefix)
	}
	if v, ok := raw["keys"]; ok {
		var m map[string]string
		if err := json.Unmarshal(v, &m); err != nil {
			return err
		}
		for k, val := range m {
			cfg.Keys[k] = val
		}
	}
	if v, ok := raw["prefixKeys"]; ok {
		var m map[string]string
		if err := json.Unmarshal(v, &m); err != nil {
			return err
		}
		for k, val := range m {
			cfg.PrefixKeys[k] = val
		}
	}
	if v, ok := raw["preview"]; ok {
		var p PreviewConfig
		if err := json.Unmarshal(v, &p); err != nil {
			return err
		}
		if p.Style != "" {
			cfg.Preview.Style = p.Style
		}
		cfg.Preview.PreserveNewLines = p.PreserveNewLines || cfg.Preview.PreserveNewLines
	}
	if v, ok := raw["footer"]; ok {
		var f FooterConfig
		if err := json.Unmarshal(v, &f); err != nil {
			return err
		}
		if len(f.Actions) > 0 {
			cfg.Footer.Actions = f.Actions
		}
	}
	if v, ok := raw["scan"]; ok {
		var s ScanConfig
		if err := json.Unmarshal(v, &s); err != nil {
			return err
		}
		if len(s.ExcludeDirs) > 0 {
			cfg.Scan.ExcludeDirs = s.ExcludeDirs
		}
		if s.MaxFileBytes > 0 {
			cfg.Scan.MaxFileBytes = s.MaxFileBytes
		}
	}
	if v, ok := raw["mouse"]; ok {
		var m MouseConfig
		if err := json.Unmarshal(v, &m); err != nil {
			return err
		}
		cfg.Mouse.Enabled = m.Enabled
	}
	if v, ok := raw["llm"]; ok {
		var l map[string]json.RawMessage
		if err := json.Unmarshal(v, &l); err != nil {
			return err
		}
		if rawEnabled, ok := l["enabled"]; ok {
			_ = json.Unmarshal(rawEnabled, &cfg.LLM.Enabled)
		}
		if rawCommand, ok := l["command"]; ok {
			_ = json.Unmarshal(rawCommand, &cfg.LLM.Command)
		}
		if rawTerminal, ok := l["terminalCommand"]; ok {
			_ = json.Unmarshal(rawTerminal, &cfg.LLM.TerminalCommand)
		}
		if rawTerminalApp, ok := l["terminalApp"]; ok {
			_ = json.Unmarshal(rawTerminalApp, &cfg.LLM.TerminalApp)
		}
		if rawOpenMode, ok := l["openMode"]; ok {
			_ = json.Unmarshal(rawOpenMode, &cfg.LLM.OpenMode)
		}
		if rawMax, ok := l["maxContextBytes"]; ok {
			_ = json.Unmarshal(rawMax, &cfg.LLM.MaxContextBytes)
		}
		if rawInclude, ok := l["includeCurrentFile"]; ok {
			_ = json.Unmarshal(rawInclude, &cfg.LLM.IncludeCurrentFile)
		}
		if rawInclude, ok := l["includeSelection"]; ok {
			_ = json.Unmarshal(rawInclude, &cfg.LLM.IncludeSelection)
		}
	}
	return nil
}

func normalize(cfg *Config) {
	if cfg.Prefix == "" {
		cfg.Prefix = Default().Prefix
	}
	if cfg.Preview.Style == "" {
		cfg.Preview.Style = "dark"
	}
	if cfg.Scan.MaxFileBytes <= 0 {
		cfg.Scan.MaxFileBytes = 1024 * 1024
	}
	if len(cfg.Scan.ExcludeDirs) == 0 {
		cfg.Scan.ExcludeDirs = Default().Scan.ExcludeDirs
	}
	if len(cfg.Footer.Actions) == 0 {
		cfg.Footer.Actions = Default().Footer.Actions
	}
	if cfg.LLM.Command == "" {
		cfg.LLM.Command = "claude"
	}
	if cfg.LLM.TerminalCommand == "" {
		cfg.LLM.TerminalCommand = "ghostty"
	}
	if cfg.LLM.TerminalApp == "" {
		cfg.LLM.TerminalApp = "Ghostty"
	}
	if cfg.LLM.OpenMode == "" {
		cfg.LLM.OpenMode = "split-right"
	}
	if cfg.LLM.MaxContextBytes <= 0 {
		cfg.LLM.MaxContextBytes = 20 * 1024
	}
}
