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
	watchDebounceDelay  = 350 * time.Millisecond
	watchPollInterval   = 2 * time.Second
	watchPollRetryTicks = 30
)

type noteWatcher interface {
	Events() <-chan filewatch.Event
	Errors() <-chan error
	Close() error
}

type watchStartedMsg struct {
	Watcher           noteWatcher
	Fingerprint       string
	IgnoreFingerprint string
}

type watchStartFailedMsg struct {
	Err               error
	Fingerprint       string
	IgnoreFingerprint string
}

type watchFileChangedMsg struct {
	Watcher noteWatcher
	Event   filewatch.Event
}

type watchDebouncedMsg struct {
	Generation int
}

type watchErrorMsg struct {
	Watcher noteWatcher
	Err     error
}

type watchClosedMsg struct {
	Watcher noteWatcher
}

type pollTickMsg struct {
	Fingerprint       string
	IgnoreFingerprint string
	Err               error
}

func startWatcherCmd(root string) tea.Cmd {
	return func() tea.Msg {
		fingerprint, _ := filewatch.Fingerprint(root)
		ignoreFingerprint, _ := filewatch.IgnoreFingerprint(root)
		watcher, err := filewatch.New(root)
		if err != nil {
			return watchStartFailedMsg{Err: err, Fingerprint: fingerprint, IgnoreFingerprint: ignoreFingerprint}
		}
		return watchStartedMsg{Watcher: watcher, Fingerprint: fingerprint, IgnoreFingerprint: ignoreFingerprint}
	}
}

func waitWatcherCmd(watcher noteWatcher) tea.Cmd {
	return func() tea.Msg {
		select {
		case event, ok := <-watcher.Events():
			if !ok {
				return watchClosedMsg{Watcher: watcher}
			}
			return watchFileChangedMsg{Watcher: watcher, Event: event}
		case err, ok := <-watcher.Errors():
			if !ok {
				return watchClosedMsg{Watcher: watcher}
			}
			return watchErrorMsg{Watcher: watcher, Err: err}
		}
	}
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

func (m Model) handleWatchStarted(msg watchStartedMsg) (Model, tea.Cmd) {
	if m.Watcher != nil && m.Watcher != msg.Watcher {
		_ = m.Watcher.Close()
	}
	m.Watcher = msg.Watcher
	m.WatchPolling = false
	m.WatchPollTicks = 0
	m.WatchFingerprint = msg.Fingerprint
	m.WatchIgnoreFingerprint = msg.IgnoreFingerprint
	if m.WatchRescanAfterStart {
		m.WatchRescanAfterStart = false
		m.rescanAfterExternalChange(filewatch.Event{Rel: ".glowedignore", Reason: "watch-restart", IgnoreChanged: true})
	}
	return m, waitWatcherCmd(msg.Watcher)
}

func (m Model) handleWatchStartFailed(msg watchStartFailedMsg) (Model, tea.Cmd) {
	if m.Watcher != nil {
		_ = m.Watcher.Close()
	}
	m.Watcher = nil
	m.WatchPolling = true
	m.WatchPollTicks = 0
	m.WatchFingerprint = msg.Fingerprint
	m.WatchIgnoreFingerprint = msg.IgnoreFingerprint
	m.WatchRescanAfterStart = false
	m.setStatus("file watcher unavailable; using polling fallback: "+msg.Err.Error(), "warn")
	return m, pollTickCmd(m.Root)
}

func (m Model) handleWatchFileChanged(msg watchFileChangedMsg) (Model, tea.Cmd) {
	if msg.Watcher != m.Watcher {
		return m, nil
	}
	return m, tea.Batch(waitWatcherCmd(msg.Watcher), m.queueWatchRescan(msg.Event))
}

func (m *Model) queueWatchRescan(event filewatch.Event) tea.Cmd {
	m.WatchDebounceGen++
	m.WatchLastEvent = event
	if event.IgnoreChanged {
		m.WatchRestartPending = true
	}
	if m.Mode == ModeEdit && m.editorFileAffected(event) {
		m.Editor.ExternalChanged = true
		m.setStatus("external change detected for editing file; editor buffer was not reloaded", "warn")
	}
	generation := m.WatchDebounceGen
	return tea.Tick(watchDebounceDelay, func(time.Time) tea.Msg {
		return watchDebouncedMsg{Generation: generation}
	})
}

func (m Model) handleWatchDebounced(msg watchDebouncedMsg) (Model, tea.Cmd) {
	if msg.Generation != m.WatchDebounceGen {
		return m, nil
	}
	restart := m.WatchRestartPending
	m.WatchRestartPending = false
	m.rescanAfterExternalChange(m.WatchLastEvent)
	if restart && !m.WatchPolling {
		if m.Watcher != nil {
			_ = m.Watcher.Close()
			m.Watcher = nil
		}
		m.WatchRescanAfterStart = true
		return m, startWatcherCmd(m.Root)
	}
	return m, nil
}

func (m Model) handleWatchError(msg watchErrorMsg) (Model, tea.Cmd) {
	if msg.Watcher != m.Watcher {
		return m, nil
	}
	m.setStatus("file watcher warning: "+msg.Err.Error(), "warn")
	return m, waitWatcherCmd(msg.Watcher)
}

func (m Model) handleWatchClosed(msg watchClosedMsg) (Model, tea.Cmd) {
	if msg.Watcher != m.Watcher {
		return m, nil
	}
	m.Watcher = nil
	m.WatchPolling = true
	m.WatchPollTicks = 0
	m.setStatus("file watcher stopped; using polling fallback", "warn")
	return m, pollTickCmd(m.Root)
}

func (m Model) handlePollTick(msg pollTickMsg) (Model, tea.Cmd) {
	if !m.WatchPolling {
		return m, nil
	}
	m.WatchPollTicks++
	retryWatcher := m.WatchPollTicks >= watchPollRetryTicks
	if retryWatcher {
		m.WatchPollTicks = 0
	}
	nextCmd := pollTickCmd(m.Root)
	if retryWatcher {
		nextCmd = startWatcherCmd(m.Root)
	}
	if msg.Err != nil {
		m.setStatus("polling failed: "+msg.Err.Error(), "warn")
		return m, nextCmd
	}
	if m.WatchFingerprint == "" {
		m.WatchFingerprint = msg.Fingerprint
		m.WatchIgnoreFingerprint = msg.IgnoreFingerprint
		return m, nextCmd
	}
	if msg.Fingerprint != m.WatchFingerprint || msg.IgnoreFingerprint != m.WatchIgnoreFingerprint {
		ignoreChanged := msg.IgnoreFingerprint != m.WatchIgnoreFingerprint
		m.WatchFingerprint = msg.Fingerprint
		m.WatchIgnoreFingerprint = msg.IgnoreFingerprint
		cmd := m.queueWatchRescan(filewatch.Event{Reason: "poll", IgnoreChanged: ignoreChanged})
		return m, tea.Batch(cmd, nextCmd)
	}
	return m, nextCmd
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

func (m *Model) shutdown() {
	if m.Watcher != nil {
		_ = m.Watcher.Close()
		m.Watcher = nil
	}
}
