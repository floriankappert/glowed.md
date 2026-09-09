# Changelog

All notable changes to this project will be documented in this file.

## Unreleased

## v0.2.2-floriankappert.12 - 2026-09-09

### Changed

- The action-menu overlay covers the whole frame now, sidebar included, and the frame is drawn as one box captioned *actions* while it is open. A click can no longer reach a pane behind it.
- *configuration* moved to the very bottom of the menu, below *delete file* and behind a blank row. Both it and the destructive entry stay pinned when the menu has to scroll.
- A submenu no longer shows the filter row, and typing in one does nothing instead of filtering invisibly. Its status line says so rather than claiming you can type.

### Fixed

- The welcome screen's action menu ignored the filter entirely, so neither actions nor documents could be searched from it, and it was missing the configuration entry. Both work there now.
- Tests no longer read or write the developer's own `~/.config/glowed/config.json`: both packages that touch it point `HOME` at a throwaway directory for the whole test run. Without that, a local config changed what the tests saw, and a test that saves a default wrote into it.


## v0.2.2-floriankappert.11 - 2026-09-09

### Added

- Added a multi-level *configuration* entry to the action menu. *configuration → defaults* offers *edit mode as default* and *sidebar visible as default*, each showing `on` or `off` and flipping on `enter`. `esc` walks back up one level before closing the menu, and the menu title shows the current level.
- Added `config.SaveDefaults`, which writes those defaults to `~/.config/glowed/config.json`. It merges the file as raw JSON, so settings this build does not know about survive and the other defaults are not frozen into the file, and it writes through a temp file so a failed write cannot truncate an existing config.
- Edit mode and sidebar visibility on launch are now read from `defaults.editMode` and `defaults.sidebarVisible` instead of being hardcoded. Both default to on, which is the behaviour they replace.

### Changed

- A submenu shows only its own entries: no mode actions, no delete, no document matches, and the filter applies to that level.


## v0.2.2-floriankappert.10 - 2026-09-09

### Added

- The action-menu filter now finds documents, not just actions: it matches on path and title, lists the hits under a *files* heading, and `enter` opens the highlighted one in the editor. Typing `todo` offers `todo.md`. At most 7 matches are shown at once, with a note saying how many were left out.

### Changed

- The lightbulb mark is two-tone: its core is drawn in a pale yellow inside the yellow rim, so it reads as glowing rather than flat.


## v0.2.2-floriankappert.9 - 2026-09-09

### Added

- The action menu opens with a filter row that has the keyboard, so `ctrl+p` and typing is enough: entries are filtered by label and key, `↑` / `↓` move through the matches, `enter` runs the highlighted one, and `esc` clears the filter before it closes the menu. Because typing goes into the filter, `j` / `k` are no longer navigation keys there.

### Changed

- The welcome screen's action menu offers only what makes sense there: open the highlighted file, new file, quit. Renaming or deleting a document that is not open, and the mode actions, are gone from it — they remain in the main window.

### Removed

- Dropped the code that kept you on the welcome screen after a rename or delete, along with the welcome-screen branch of the action target: neither is reachable now that those actions are gone from that screen.


## v0.2.2-floriankappert.8 - 2026-09-09

### Added

- The action menu's reference list is back in browse mode, which had no non-runnable hints of its own and therefore showed no "keys" section at all: `↑↓` select, `enter` open, `tab` cycle focus, `shift+tab` sidebar focus.
- `ctrl+t` toggles the sidebar. `ctrl+b` stays bound, but Ghostty claims it on macOS, where it never reaches the program.

### Changed

- The search input now shares the header's third row, next to the lightbulb, with the result counter beside it. It no longer costs a row of its own, and the status message takes that row while the search is idle. The compact header keeps a separate search row.
- Menu labels and keys sit in their own columns: the label column is as wide as the longest label, so the keys line up instead of drifting to the far edge.
- The toggle entries read `<> sidebar` and `<> edit/preview`, and a blank row separates them from *go home*.

### Fixed

- *delete file* could scroll out of sight on a short pane, which is the one entry that must always be visible. The menu now pins the title at the top and the destructive entry at the bottom, and scrolls only the entries between them.


## v0.2.2-floriankappert.7 - 2026-09-09

### Added

- Added *toggle sidebar* and *toggle edit/preview* to the action menu. The mode toggle refuses to leave a buffer with unsaved changes, unlike `esc`, which discards them.

### Changed

- *delete file* moved to the bottom of the action menu, set apart by a blank row and rendered in red, with a red highlight when selected. It is the only destructive entry, so it no longer sits between the harmless ones.
- The sidebar hint dropped out of the menu's reference list, where it now repeated the runnable *toggle sidebar* entry.


## v0.2.2-floriankappert.6 - 2026-09-09

### Changed

- The header no longer repeats itself: row 1 is the name, row 2 the mode (with the unsaved marker), row 3 the status message. The focus name is gone, because the focused pane is already marked by its caption colour, and the current file is gone, because the status line and the bottom row already carry it.
- A blank row now separates the header block from the panes.
- The sidebar pane caption is *Files & Folders* instead of *files*.
- The action menu box is wider than its measured content, so a binding like `cmd+⌫` can no longer be squeezed against its label — some glyphs are drawn wider than their reported width.


## v0.2.2-floriankappert.5 - 2026-09-09

### Added

- Added a "go home" entry to the action menu (`ctrl+p`), which returns to the welcome screen. It refuses to run while the buffer has unsaved changes.
- The action menu now works on the welcome screen, where it covers the whole screen, targets the highlighted recent file and names it in its title. `ctrl+n` works there too, so an empty project is no longer a dead end.
- The third header row, next to the lightbulb, now carries the status message in its status colour.

### Changed

- The footer hint bar is gone. Its bindings moved into the action menu: the runnable ones as selectable entries with their keys, the rest as a reference list. The bottom row is now the document path plus a `ctrl+p actions` pointer, and the layout gains a content row.
- The action menu backdrop is much darker, so the overlay reads as dark as the editor instead of as a light grey slab.
- On a pane too short for the whole menu, the reference keys are dropped first and the entry list scrolls to keep the selection visible, instead of silently clipping rows.
- The README was cut down to what this fork actually changes: it now links the original project prominently, credits both authors at the top, and points at upstream for everything the fork does not alter. The inherited sections that only restated upstream's documentation — features, usage, search, configuration, limitations, screenshots and distribution boilerplate — were removed.

### Fixed

- Rename and delete no longer drop you into the editor when they were started from the welcome screen; the recent-file list refreshes and you stay home.


## v0.2.2-floriankappert.4 - 2026-09-09

### Added

- Added a "go home" entry to the action menu (`ctrl+p`), which returns to the welcome screen. It refuses to run while the buffer has unsaved changes.

### Changed

- The project is now published as **glowed.md** at `floriankappert/glowed.md`, and the welcome screen, header, `--help` and `--version` carry that name. The installed command, the Homebrew formula and the config paths keep the name `glowed`, so nothing about invoking or configuring it changes.
- The Homebrew tap is now `floriankappert/glowed.md`, so installing and upgrading use `brew install floriankappert/glowed.md/glowed`. Homebrew derives the repository name from the tap name, so the repository is `homebrew-glowed.md`; the previous name relied on a GitHub rename redirect to be reachable at all.
- Note that the bare `brew upgrade glowed` is ambiguous while the upstream `khw1031/tap` is installed too, because both taps carry a formula named `glowed`. Use the fully-qualified name.

### Fixed

- An alt combination that has no binding now says so in the status line instead of being dropped silently. On layouts where brackets are Option-composed, Ghostty's `macos-option-as-alt = true` turns `[` into `alt+5`, which made the key look broken; the new README FAQ explains the `macos-option-as-alt = left` fix.

### Documentation

- Added a FAQ to the README covering the Option-composed characters (`[ ] { } @ | \ ~ €`), why copy is `opt+c`, and which Ghostty setting to change.


## v0.2.2-floriankappert.3 - 2026-09-09

### Added

- The header above the panes now carries the yellow lightbulb mark, with the name and mode beside it and the current file underneath. On terminals shorter than 12 rows it collapses back into a single row so the frame still fits.
- Added a welcome screen on launch: a yellow lightbulb mark next to the version and project root, plus the 7 most recently modified documents under a "Recent files" heading. `↑` / `↓` select, `enter` opens the file in the editor. It stays up until a file is picked, and is skipped when a file is passed on the command line.
- Added `ctrl+n` to create a new Markdown file. The filename is entered in the toolbar row, the file is created next to the current document, and `.md` is appended when the name has no extension.
- Added `ctrl+p` to open a file action menu: new file, edit filename, delete file. `ctrl+k` was not used for it because it already deletes to the line end in edit mode. The menu covers the content pane with its own backdrop, and its entries sit centered in the pane while staying left-aligned with each other.
- Added renaming the current document from the action menu, refusing an existing target name and refusing to run while the buffer has unsaved changes.
- Added deleting the current document from the action menu. It asks for confirmation (`y`) and keeps a `<name>.md.bak` copy, so the delete stays recoverable.
- Added `docs.GuardNewPath` so create and rename are checked against the project root before touching the filesystem, the way opening and saving already were.
- The Markdown scan now records each file's modification time, which is what the welcome screen orders by.

### Changed

- The sidebar is now visible on launch instead of hidden.
- The search row below the header is only rendered while the search has focus or a query is set. An idle frame spends that row on content instead of a placeholder.
- The README now documents how this fork differs from the upstream project, and points its install instructions at this fork and the `floriankappert/tap` Homebrew tap.

### Fixed

- The caret now follows the focus: while the filename prompt or the action menu is open, the buffer stops drawing its own caret, so only the line that owns the keyboard shows one.
- The edit-mode caret no longer pushes the rest of the line one column to the right: it now covers the cell it sits on instead of being inserted before it. Wide runes stay intact.
- Corrected the documented copy shortcut: `cmd+c` cannot reach the program because macOS routes it to Ghostty's Edit menu, so the footer and the README now name `opt+c`. The same applies to `cmd+a`; remapping them in the Ghostty config has no effect.


## v0.2.2-floriankappert.2 - 2026-09-09

### Added

- Added word-wise and line-wise caret motion in edit mode (`opt+←→`, `cmd+←→`/`ctrl+a`/`ctrl+e`).
- Added word-wise and line-wise deletion in edit mode (`opt+⌫`, `opt+⌦`, `cmd+⌫`/`ctrl+u`, `ctrl+k`).
- Added keyboard text selection in edit mode with `shift` and `opt+shift` motion, plus `opt+a` to select the whole buffer; typing or deleting replaces an active selection.
- Added syntax highlighting for fenced code blocks in edit and source mode, using the colors of the configured `preview.style`.
- Added `ctrl+b` to toggle the sidebar in every mode, including edit mode where the browse bindings are unavailable.
- Added `shift+tab` to move the focus between the content pane and the sidebar in every mode, opening the sidebar when it is hidden.
- Added clipboard copy (`cmd+c` / `opt+c`) for the editor selection, copied as plain text.
- Added paste at the caret (`cmd+v` / `opt+v`), replacing an active selection as a single undoable edit; pasted text may span multiple lines.
- Added sidebar navigation and `enter` to open the selected document directly in edit mode, refusing the switch while the current buffer has unsaved changes.

### Fixed

- Pasted text is no longer dropped in edit mode; multi-line pastes previously failed the control-character filter entirely.
- alt-modified keys no longer type their letter into the search field or the chat input.
- Space can now be typed in edit mode; it arrives as its own key type and was previously dropped.
- Escape sequences and other control runes no longer leak into the buffer in edit mode.

### Changed

- Panes are now drawn as fully bordered boxes that share their vertical edges, and the pane captions sit further inside the top border.
- The focused pane is highlighted: its border and caption use the accent color.
- The document path moved out of the pane body and the header into its own toolbar row directly above the footer, which gives every pane one more content row.
- The footer bar now shows edit-mode bindings while editing instead of the browse bindings, and drops optional hints when the terminal is too narrow.
- `opt+←` / `opt+→` now move by word instead of jumping to line start/end; use `cmd+←` / `cmd+→`, `home` / `end`, or `ctrl+a` / `ctrl+e` for line start/end.
- Edit is now the default mode: glowed starts in the editor and documents opened from the sidebar open for editing; projects without documents still start in the preview.
- Saving with `ctrl+s` now keeps the buffer open instead of switching to the preview.
- `esc` in edit mode now clears an active selection first and only leaves edit mode when nothing is selected.


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

