# Changelog

All notable changes to this project will be documented in this file.

## Unreleased

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

