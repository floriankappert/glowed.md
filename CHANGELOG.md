# Changelog

All notable changes to this project will be documented in this file.

## Unreleased

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

