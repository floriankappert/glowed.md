# Changelog

All notable changes to this project will be documented in this file.

## Unreleased

## v0.2.2 - 2026-05-18

### Added

- Added built-in default Markdown scan ignores for common VCS, dependency, cache, and generated-output paths.
- Added `.glowedignore` negation overrides so projects can re-include paths hidden by built-in defaults.
- Added `glowed --init-ignore [project-root]` to create a starter `.glowedignore` template without overwriting existing files.

### Changed

- Switched automatic live refresh to a consistent lightweight polling model with a 5 second interval.
- Removed the default native watcher/fsnotify runtime path; polling snapshot refresh is now the single automatic refresh model.
- Polling fingerprints now use Markdown path/size/modtime plus `.glowedignore` fingerprints to avoid reading file contents on every tick.
- Active edit/source buffers now use content fingerprints for external-change detection while project-wide polling remains lightweight.
- Search now includes Markdown body text and ranks general matches by title, body, frontmatter, path/filename, then tag.

### Fixed

- Search focus now shows an explicit cursor so typed query text is easier to distinguish.
- Search result snippets now show their match source, such as `title:`, `body:`, `frontmatter:`, `path:`, or `tag:foo`.
- Title extraction now skips fenced and indented code blocks when choosing the first Markdown H1.
- Filesystem refresh debounce is coalesced so repeated polling changes do not create unbounded debounce timers.

### Breaking Changes

- Markdown files under built-in ignored paths such as `.git/`, `node_modules/`, `vendor/`, root `/build/`, and root `/dist/` are now hidden by default. Add `!pattern` rules to `.glowedignore` to re-include paths that should be visible.
- Automatic refresh now reflects changes after the polling interval rather than through native filesystem events. Use `r` for immediate manual refresh when needed.

### Documentation

- Updated English, Korean, Japanese, and Chinese READMEs for polling refresh, body search, built-in ignore defaults, and `--init-ignore`.

### Internal

- Removed the `fsnotify` dependency.
- Removed the old `Document.Haystack` search cache and switched search tests to source-aware fields.


## v0.2.1 - 2026-05-18

### Added

- Added live Markdown file watching so files created, modified, deleted, or renamed while glowed is running are reflected automatically.
- Added polling fallback for environments where native file watching cannot be started, with periodic retry of native watching.
- Added project agent guidance files (`AGENTS.md` and `CLAUDE.md`) for repository-level coding and release conventions.

### Fixed

- Search input now accepts spaces, so multi-token AND searches such as `foo bar` can be typed directly.
- Special keys and escape-like control input are filtered out of the search query instead of appearing as stray character codes.
- Search now supports word deletion with `ctrl+w` and `alt+backspace`.
- Editing buffers are protected from external file changes; glowed warns instead of overwriting dirty editor contents.

### Changed

- `.glowedignore` changes now trigger watcher rebuilds and automatic rescans while glowed is running.
- Search help text now clarifies that whitespace-separated tokens are matched with AND semantics and that `tag:foo` searches tags.
- Source selection mode reloads the raw buffer after external changes when possible, or returns safely to preview if reload fails.

### Documentation

- Documented live file watching, polling fallback, manual refresh behavior, and search syntax in the English and Korean READMEs.
- Clarified that frontmatter `tag` / `tags` fields are indexed as metadata, while the query operator is `tag:foo`.

### Internal

- Added an `internal/watch` package backed by `fsnotify`, debounce handling in the Bubble Tea update loop, and tests for file watching, polling fallback, `.glowedignore` changes, and search input behavior.


## v0.2.0 - 2026-05-18

### Added

- Sidebar now shows Markdown files in an expandable directory tree when no search is active.
- Added project-local `.glowedignore` support for Markdown scan exclusions using gitignore-style patterns.
- Scan status now reports when Markdown files or directories are hidden by `.glowedignore` or `maxFileBytes`.

### Changed

- `tab` and `enter` now expand or collapse a selected sidebar directory; `left` and `right` also collapse or expand directories.
- Search results continue to show as a flat document list so filtering remains direct.

### Removed

- Removed `scan.excludeDirs` from `.glowed.json`, `.glowed.example.json`, and `glowed.schema.json`.
- glowed no longer reads root `.gitignore` for Markdown scan exclusions.

### Breaking Changes

- Markdown scan exclusions now come only from `<project-root>/.glowedignore`.
- Migration: move any `scan.excludeDirs` entries from glowed config into `.glowedignore`, one pattern per line. For example, use `/build/` to hide only the root `build` directory, or `build/` to hide every directory named `build`.
- Migration: if you relied on `.gitignore` to hide Markdown files from glowed, copy the relevant rules into `.glowedignore`.

### Documentation

- Updated English, Korean, Japanese, and Chinese README files for the sidebar tree, `.glowedignore`, and distribution policy changes.
- Clarified that custom taps/builds are welcome and that distribution registration is for discovery, not approval.
- Added changelog and release-note workflow documentation.

### Internal

- Added release changelog tooling: `draft-changelog.sh`, `update-changelog.sh`, and `extract-release-notes.sh`.
- Added scan-report coverage and tests for `.glowedignore`, ignored paths, root-only patterns, and sidebar expand/collapse behavior.
- Updated the release workflow to treat `CHANGELOG.md` as the public source of truth for release notes.


## v0.1.0 - 2026-05-17

### Added

- Initial Ghostty-oriented terminal TUI Markdown browser/editor.
- Project Markdown scanning with common ignored directories, basic root `.gitignore` support, and configurable file size limits.
- Search across filenames, frontmatter, and `tag:` / `tags:` metadata.
- Sidebar document list, rendered Markdown preview via Glamour, and per-document preview scroll restoration.
- Raw Markdown edit mode with save, undo/redo, and backup-based atomic writes.
- Mouse click, wheel, and drag support for app-managed selection.
- Source selection mode for copying exact original Markdown with glowed metadata.
- External LLM session launcher that opens a configured CLI, copies current Markdown context, and targets Ghostty split workflows by default.
- Configurable key bindings, prefix bindings, footer actions, preview style, scan settings, and LLM launch settings.
- CLI support for opening a project root, opening a specific Markdown file, `--help`, and `--version`.
- JSON schema and example configuration for editor validation/autocomplete.

### Breaking Changes

- None. This is the first public release.
- Migration: no existing glowed configuration or file format migration is required.

### Documentation

- Added English, Korean, Japanese, and Chinese README files.
- Added screenshots for preview and Ghostty split workflows.
- Added contribution guidance, custom distribution guidance, and a distribution registry document.
- Added a GitHub issue template for registering public custom distributions.

### Internal

- Added the initial Go module and test suite.
- Added rendering, scanning, search, editor save, selection, clipboard, path guard, configuration, and LLM integration packages.
- Added benchmark coverage for Markdown scanning and preview rendering.
- Added a release workflow skill and release prompt for publishing GitHub/Homebrew releases.

