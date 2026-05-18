package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/khw1031/glowed/internal/docs"
	filewatch "github.com/khw1031/glowed/internal/watch"
)

const (
	watchDebounceDelay = 750 * time.Millisecond
	watchPollInterval  = 5 * time.Second
)

type watchDebouncedMsg struct {
	Generation int
}

type pollTickMsg struct {
	Fingerprint       string
	IgnoreFingerprint string
	Err               error
}

func pollTickCmd(root string) tea.Cmd {
	return tea.Tick(watchPollInterval, func(time.Time) tea.Msg {
		fingerprint, err := filewatch.Fingerprint(root)
		ignoreFingerprint, ignoreErr := filewatch.IgnoreFingerprint(root)
		if err == nil {
			err = ignoreErr
		}
		return pollTickMsg{Fingerprint: fingerprint, IgnoreFingerprint: ignoreFingerprint, Err: err}
	})
}

func (m *Model) queueWatchRescan(event filewatch.Event) tea.Cmd {
	m.WatchDebounceGen++
	m.WatchLastEvent = event
	if m.Mode == ModeEdit && m.editorFileAffected(event) {
		m.Editor.ExternalChanged = true
		m.setStatus("external change detected for editing file; editor buffer was not reloaded", "warn")
	}
	if m.WatchDebouncePending {
		return nil
	}
	return m.startWatchDebounceTimer()
}

func (m *Model) startWatchDebounceTimer() tea.Cmd {
	m.WatchDebouncePending = true
	generation := m.WatchDebounceGen
	return tea.Tick(watchDebounceDelay, func(time.Time) tea.Msg {
		return watchDebouncedMsg{Generation: generation}
	})
}

func (m Model) handleWatchDebounced(msg watchDebouncedMsg) (Model, tea.Cmd) {
	m.WatchDebouncePending = false
	if msg.Generation != m.WatchDebounceGen {
		// A newer polling change arrived while this timer was pending. Start one
		// replacement debounce window from now so bursts coalesce into one rescan.
		return m, m.startWatchDebounceTimer()
	}
	m.rescanAfterExternalChange(m.WatchLastEvent)
	return m, nil
}

func (m *Model) initializePollingRefresh() {
	fingerprint, err := filewatch.Fingerprint(m.Root)
	ignoreFingerprint, ignoreErr := filewatch.IgnoreFingerprint(m.Root)
	if err == nil {
		err = ignoreErr
	}
	if err != nil {
		m.setStatus("polling refresh baseline failed: "+err.Error(), "warn")
		return
	}
	m.WatchFingerprint = fingerprint
	m.WatchIgnoreFingerprint = ignoreFingerprint
	notice := fmt.Sprintf("polling refresh every %s", watchPollInterval)
	if m.Status == "" {
		m.setStatus(notice, "info")
	} else if m.StatusKind != "error" && !strings.Contains(m.Status, notice) {
		m.setStatus(fmt.Sprintf("%s; %s", m.Status, notice), m.StatusKind)
	}
}

func (m Model) handlePollTick(msg pollTickMsg) (Model, tea.Cmd) {
	nextCmd := pollTickCmd(m.Root)
	if msg.Err != nil {
		m.setStatus("polling refresh failed: "+msg.Err.Error(), "warn")
		return m, nextCmd
	}
	if m.WatchFingerprint == "" {
		m.WatchFingerprint = msg.Fingerprint
		m.WatchIgnoreFingerprint = msg.IgnoreFingerprint
		return m, nextCmd
	}
	editorChanged, editorFingerprint := m.pollingEditorFileChanged()
	if msg.Fingerprint != m.WatchFingerprint || msg.IgnoreFingerprint != m.WatchIgnoreFingerprint || editorChanged {
		ignoreChanged := msg.IgnoreFingerprint != m.WatchIgnoreFingerprint
		event := filewatch.Event{Reason: "poll", IgnoreChanged: ignoreChanged}
		if editorChanged {
			event.Path = m.Editor.File
			m.Editor.FileFingerprint = editorFingerprint
			if rel, err := filepath.Rel(m.Root, m.Editor.File); err == nil {
				event.Rel = filepath.ToSlash(rel)
			}
		}
		m.WatchFingerprint = msg.Fingerprint
		m.WatchIgnoreFingerprint = msg.IgnoreFingerprint
		cmd := m.queueWatchRescan(event)
		if cmd == nil {
			return m, nextCmd
		}
		return m, tea.Batch(cmd, nextCmd)
	}
	return m, nextCmd
}

func (m Model) pollingEditorFileChanged() (bool, string) {
	if (m.Mode != ModeEdit && m.Mode != ModeSource) || m.Editor.File == "" || m.Editor.FileFingerprint == "" {
		return false, ""
	}
	fingerprint, err := filewatch.ContentFingerprint(m.Editor.File)
	if err != nil {
		return true, ""
	}
	return fingerprint != m.Editor.FileFingerprint, fingerprint
}

func (m *Model) rescanAfterExternalChange(event filewatch.Event) {
	beforeAbs := ""
	beforeRel := ""
	if doc := m.currentDoc(); doc != nil {
		beforeAbs = doc.Abs
		beforeRel = doc.Rel
	}
	editorAffected := m.editorFileAffected(event)
	if m.Mode == ModeEdit && editorAffected {
		m.Editor.ExternalChanged = true
	}

	if err := m.scanAndApply(true); err != nil {
		m.setStatus("auto refresh failed: "+err.Error(), "error")
		return
	}

	status := "notes updated: " + m.scanStatus()
	kind := "success"
	if m.Mode == ModeSource && editorAffected {
		if err := m.reloadSourceBuffer(); err != nil {
			m.Mode = ModePreview
			m.Focus = FocusPreview
			status = "source file changed but raw buffer reload failed; returned to preview: " + err.Error()
			kind = "warn"
		} else {
			status = "source file reloaded after external change"
		}
	}
	if m.Mode == ModeEdit && editorAffected {
		status = "external change detected for editing file; editor buffer was not reloaded"
		kind = "warn"
	} else if beforeAbs != "" && !m.resultContainsAbs(beforeAbs) {
		selected := "no document"
		if doc := m.currentDoc(); doc != nil {
			selected = doc.Rel
		}
		status = fmt.Sprintf("document removed: %s; selected %s", beforeRel, selected)
		kind = "warn"
	} else if event.IgnoreChanged {
		status = ".glowedignore changed: " + m.scanStatus()
	}
	m.setStatus(status, kind)
}

func (m *Model) reloadSourceBuffer() error {
	if m.Editor.File == "" {
		return nil
	}
	path, err := docs.GuardExistingPath(m.Root, m.Editor.File)
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	m.Editor.Lines = splitEditorLines(string(raw))
	if len(m.Editor.Lines) == 0 {
		m.Editor.Lines = []string{""}
	}
	m.Editor.File = path
	m.Editor.FileFingerprint, _ = filewatch.ContentFingerprint(path)
	m.Editor.CY = clamp(m.Editor.CY, 0, len(m.Editor.Lines)-1)
	m.Editor.CX = clamp(m.Editor.CX, 0, lineLen(m.Editor.Lines[m.Editor.CY]))
	m.Editor.ScrollY = clamp(m.Editor.ScrollY, 0, max(0, len(m.Editor.Lines)-m.editorTextHeight()))
	m.Editor.ScrollX = max(0, m.Editor.ScrollX)
	return nil
}

func (m Model) editorFileAffected(event filewatch.Event) bool {
	if m.Editor.File == "" || event.Path == "" {
		return false
	}
	for _, eventRel := range rootRelativePaths(m.Root, event.Path) {
		for _, editorRel := range rootRelativePaths(m.Root, m.Editor.File) {
			if eventRel == editorRel || isAncestorRel(eventRel, editorRel) {
				return true
			}
		}
	}
	eventPath, err := comparablePath(event.Path)
	if err != nil {
		return false
	}
	editorPath, err := comparablePath(m.Editor.File)
	if err != nil {
		return false
	}
	return eventPath == editorPath || isAncestorPath(eventPath, editorPath)
}

func rootRelativePaths(root string, path string) []string {
	roots := comparablePathCandidates(root)
	paths := comparablePathCandidates(path)
	out := []string{}
	seen := map[string]bool{}
	for _, root := range roots {
		for _, path := range paths {
			rel, err := filepath.Rel(root, path)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
				continue
			}
			rel = filepath.ToSlash(filepath.Clean(rel))
			if !seen[rel] {
				out = append(out, rel)
				seen[rel] = true
			}
		}
	}
	return out
}

func comparablePathCandidates(path string) []string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil
	}
	out := []string{filepath.Clean(abs)}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		real = filepath.Clean(real)
		if real != out[0] {
			out = append(out, real)
		}
	}
	return out
}

func comparablePath(path string) (string, error) {
	candidates := comparablePathCandidates(path)
	if len(candidates) == 0 {
		return "", fmt.Errorf("invalid path: %s", path)
	}
	return candidates[len(candidates)-1], nil
}

func isAncestorRel(parent string, child string) bool {
	parent = filepath.ToSlash(filepath.Clean(parent))
	child = filepath.ToSlash(filepath.Clean(child))
	return parent != "." && parent != child && strings.HasPrefix(child, parent+"/")
}

func isAncestorPath(parent string, child string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func (m Model) resultContainsAbs(abs string) bool {
	for _, doc := range m.Results {
		if doc.Abs == abs {
			return true
		}
	}
	return false
}

func (m Model) Shutdown() {
	m.shutdown()
}

func (m *Model) shutdown() {}
