package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/khw1031/glowed/internal/app"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "-h" || os.Args[1] == "--help") {
		fmt.Println("glowed - Ghostty terminal Markdown browser/editor")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  glowed [project-root]")
		fmt.Println("  glowed [project-root] [initial-markdown-file]")
		fmt.Println("  glowed [initial-markdown-file]")
		return
	}

	root, initial, err := resolveArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	m := app.NewWithInitial(root, initial)
	opts := []tea.ProgramOption{tea.WithAltScreen(), tea.WithMouseCellMotion()}
	program := tea.NewProgram(m, opts...)
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Print("\x1b[0 q")
}

func resolveArgs(args []string) (root string, initial string, err error) {
	root = "."
	if len(args) == 0 {
		root, err = absDir(root)
		return root, "", err
	}
	if len(args) == 1 {
		abs, err := filepath.Abs(args[0])
		if err != nil {
			return "", "", err
		}
		info, statErr := os.Stat(abs)
		if statErr == nil && info.IsDir() {
			root, err = absDir(abs)
			return root, "", err
		}
		if statErr == nil && !info.IsDir() && strings.EqualFold(filepath.Ext(abs), ".md") {
			root, err = absDir(filepath.Dir(abs))
			return root, abs, err
		}
		return "", "", fmt.Errorf("glowed: path is neither a directory nor markdown file: %s", abs)
	}
	root, err = absDir(args[0])
	if err != nil {
		return "", "", err
	}
	initial, err = filepath.Abs(args[1])
	if err != nil {
		return "", "", err
	}
	return root, initial, nil
}

func absDir(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("glowed: root is not a directory: %s", abs)
	}
	return abs, nil
}
