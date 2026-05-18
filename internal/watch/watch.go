package watch

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"

	"github.com/khw1031/glowed/internal/docs"
)

// Event describes a filesystem change that may affect glowed's scanned note set.
type Event struct {
	Path          string
	Rel           string
	Reason        string
	IgnoreChanged bool
}

// Watcher recursively watches note-relevant paths under a project root.
type Watcher struct {
	root string
	// matcher is a snapshot loaded at watcher construction. Callers must restart
	// the watcher when .glowedignore changes.
	matcher docs.IgnoreMatcher

	watcher *fsnotify.Watcher
	events  chan Event
	errors  chan error
	done    chan struct{}
	once    sync.Once

	mu          sync.Mutex
	watchedDirs map[string]bool
}

// New starts an fsnotify-backed recursive watcher for root.
func New(root string) (*Watcher, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	absRoot = filepath.Clean(absRoot)
	info, err := os.Stat(absRoot)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("watch root is not a directory: %s", absRoot)
	}

	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	w := &Watcher{
		root:        absRoot,
		matcher:     docs.LoadIgnoreMatcher(absRoot),
		watcher:     fw,
		events:      make(chan Event, 64),
		errors:      make(chan error, 8),
		done:        make(chan struct{}),
		watchedDirs: map[string]bool{},
	}
	if err := w.addRecursive(absRoot); err != nil {
		_ = fw.Close()
		return nil, err
	}
	go w.run()
	return w, nil
}

func (w *Watcher) Events() <-chan Event { return w.events }

func (w *Watcher) Errors() <-chan error { return w.errors }

func (w *Watcher) Close() error {
	var err error
	w.once.Do(func() {
		close(w.done)
		err = w.watcher.Close()
	})
	return err
}

func (w *Watcher) run() {
	defer close(w.events)
	defer close(w.errors)
	for {
		select {
		case <-w.done:
			return
		case ev, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			w.handleEvent(ev)
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			w.sendError(err)
		}
	}
}

func (w *Watcher) handleEvent(ev fsnotify.Event) {
	clean := filepath.Clean(ev.Name)
	if ev.Op&fsnotify.Create != 0 {
		w.addCreatedDirectory(clean)
	}
	if event := w.noteEvent(ev); event != nil {
		w.sendEvent(*event)
	}
	if ev.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
		w.removeKnownDir(clean)
	}
}

func (w *Watcher) noteEvent(ev fsnotify.Event) *Event {
	if ev.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) == 0 {
		return nil
	}
	clean := filepath.Clean(ev.Name)
	rel, ok := w.rel(clean)
	if !ok {
		return nil
	}
	if rel == ".glowedignore" {
		return &Event{Path: clean, Rel: rel, Reason: ev.Op.String(), IgnoreChanged: true}
	}
	if w.matcher.Ignored(rel, false) {
		return nil
	}
	if strings.EqualFold(filepath.Ext(clean), ".md") {
		return &Event{Path: clean, Rel: rel, Reason: ev.Op.String()}
	}
	if w.isKnownDir(clean) && ev.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
		return &Event{Path: clean, Rel: rel, Reason: ev.Op.String()}
	}
	if ev.Op&fsnotify.Create != 0 && isDir(clean) && !w.matcher.Ignored(rel, true) {
		return &Event{Path: clean, Rel: rel, Reason: ev.Op.String()}
	}
	return nil
}

func (w *Watcher) addCreatedDirectory(path string) {
	if !isDir(path) {
		return
	}
	rel, ok := w.rel(path)
	if !ok || w.matcher.Ignored(rel, true) {
		return
	}
	if err := w.addRecursive(path); err != nil {
		w.sendError(err)
	}
}

func (w *Watcher) addRecursive(root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !entry.IsDir() {
			return nil
		}
		rel, ok := w.rel(path)
		if !ok {
			return filepath.SkipDir
		}
		if rel != "." && w.matcher.Ignored(rel, true) {
			return filepath.SkipDir
		}
		return w.addDir(path)
	})
}

func (w *Watcher) addDir(path string) error {
	path = filepath.Clean(path)
	w.mu.Lock()
	if w.watchedDirs[path] {
		w.mu.Unlock()
		return nil
	}
	w.mu.Unlock()

	if err := w.watcher.Add(path); err != nil {
		return err
	}
	w.mu.Lock()
	w.watchedDirs[path] = true
	w.mu.Unlock()
	return nil
}

func (w *Watcher) removeKnownDir(path string) {
	path = filepath.Clean(path)
	w.mu.Lock()
	defer w.mu.Unlock()
	for watched := range w.watchedDirs {
		if watched == path || isSubpath(path, watched) {
			delete(w.watchedDirs, watched)
		}
	}
}

func (w *Watcher) isKnownDir(path string) bool {
	path = filepath.Clean(path)
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.watchedDirs[path]
}

func (w *Watcher) rel(path string) (string, bool) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	abs = filepath.Clean(abs)
	rel, err := filepath.Rel(w.root, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", false
	}
	if rel == "" {
		rel = "."
	}
	return filepath.ToSlash(rel), true
}

func (w *Watcher) sendEvent(ev Event) {
	select {
	case w.events <- ev:
	case <-w.done:
	}
}

func (w *Watcher) sendError(err error) {
	select {
	case w.errors <- err:
	case <-w.done:
	}
}

// IgnoreFingerprint returns a fingerprint for .glowedignore itself.
func IgnoreFingerprint(root string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	path := filepath.Join(filepath.Clean(absRoot), ".glowedignore")
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return "missing", nil
	}
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "dir", nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return fmt.Sprintf("file\x00%d\x00%d\x00%s", info.Size(), info.ModTime().UnixNano(), hex.EncodeToString(h[:])), nil
}

// Fingerprint returns a stable fingerprint of note-relevant filesystem state.
// It is used by polling fallback when fsnotify cannot be started.
func Fingerprint(root string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	absRoot = filepath.Clean(absRoot)
	matcher := docs.LoadIgnoreMatcher(absRoot)
	entries := []string{}

	addStat := func(path string, rel string) {
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			return
		}
		entries = append(entries, fmt.Sprintf("%s\x00%d\x00%d\x00%s", filepath.ToSlash(rel), info.Size(), info.ModTime().UnixNano(), filePrefixHash(path)))
	}
	addStat(filepath.Join(absRoot, ".glowedignore"), ".glowedignore")

	err = filepath.WalkDir(absRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, relErr := filepath.Rel(absRoot, path)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if rel != "." && matcher.Ignored(rel, true) {
				return filepath.SkipDir
			}
			return nil
		}
		if matcher.Ignored(rel, false) || !entry.Type().IsRegular() || !strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
			return nil
		}
		addStat(path, rel)
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(entries)
	h := sha256.New()
	for _, entry := range entries {
		_, _ = h.Write([]byte(entry))
		_, _ = h.Write([]byte("\n"))
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func filePrefixHash(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return "open-error"
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	_, err = io.CopyN(h, f, 4096)
	if err != nil && err != io.EOF {
		return "read-error"
	}
	return hex.EncodeToString(h.Sum(nil))
}

func isSubpath(parent string, child string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
