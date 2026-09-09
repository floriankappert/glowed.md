# glowed.md

**glowed.md** is a Ghostty-oriented terminal TUI Markdown browser/editor. The
command it installs is `glowed`.

It treats the directory where it is launched as a project root, scans Markdown files, lets you search and preview them, edit raw Markdown, copy app-managed selections with path metadata, and open an external LLM CLI session with the current document context.

> This repository is a fork of [khw1031/glowed](https://github.com/khw1031/glowed) that turns the MVP's raw edit mode into a usable editor. It is published as **glowed.md**; the command, the Homebrew formula and the config paths keep the name `glowed`. See
> [Differences from the upstream project](#differences-from-the-upstream-project) for the complete list.

Language versions: [한국어](README.ko.md) · [日本語](README.jp.md) · [中文](README.zh.md)

## Screenshots

![glowed Markdown preview in Ghostty](assets/glowed-preview.png)

![glowed with sidebar and pi agent split in Ghostty](assets/glowed-ghostty-split.png)

## Project status

This project is in an early MVP stage.

The current implementation is a Go TUI built with:

- Bubble Tea
- Lipgloss
- Glamour
- Go standard tooling

glowed is currently implemented as a Go terminal application.

The current implementation was produced with Codex GPT-5.5, a local `TODO.md` planning file, and the pi agent coding harness.

## Differences from the upstream project

This fork branched off upstream `v0.2.2` and is versioned as
`v0.2.2-floriankappert.N`. Upstream's editor is an MVP raw-buffer mode; the work
here makes it behave like an editor and reshapes the surrounding UI. Everything
below is additional to upstream — no upstream feature was removed.

### Editing

- Word- and line-wise caret motion: `opt+←` / `opt+→`, and `cmd+←` / `cmd+→`
  (also `ctrl+a` / `ctrl+e`, `home` / `end`).
- Word- and line-wise deletion: `opt+⌫`, `opt+⌦`, `cmd+⌫` (`ctrl+u`), `ctrl+k`.
- Keyboard text selection with `shift` and `opt+shift` motion, plus `opt+a` to
  select the whole buffer. Typing or deleting replaces an active selection.
- Clipboard copy of the editor selection as plain text (`opt+c`).
- Paste at the caret (`cmd+v` / `opt+v`), replacing an active selection as a
  single undoable edit; multi-line pastes are supported.
- Syntax highlighting for fenced code blocks in edit and source mode, using the
  colors of the configured `preview.style`.
- `opt+←` / `opt+→` were remapped from line start/end to word motion; line
  start/end moved to `cmd+←` / `cmd+→`, `home` / `end`, or `ctrl+a` / `ctrl+e`.
- `esc` clears an active selection first and only leaves edit mode when nothing
  is selected.
- `ctrl+s` keeps the buffer open instead of switching back to the preview.

### Modes and navigation

- Edit is the default mode: glowed starts in the editor, and documents opened
  from the sidebar open for editing. Projects without documents still start in
  the preview.
- `ctrl+b` toggles the sidebar in every mode, including edit mode where the
  browse bindings are unavailable.
- `shift+tab` moves the focus between the content pane and the sidebar in every
  mode, opening the sidebar when it is hidden.
- Sidebar navigation with `↑` / `↓` and `enter` opens the selected document
  directly in edit mode. The switch is refused while the current buffer has
  unsaved changes.

### Layout

- A welcome screen is shown on launch — a lightbulb mark next to the version and
  the project root, plus the 7 most recently modified documents under a *Recent
  files* heading. `↑` / `↓` select and `enter` opens one in the editor. It stays
  up until a file is picked, and is skipped when a file is passed on the command
  line.
- The header above the panes carries the same lightbulb mark, with the name
  (`glowed.md`) and mode beside it and the current file underneath. Below a
  terminal height of 12 rows it collapses into the single row it used to be, so
  short splits still render a frame that fits.
- The sidebar is visible on launch instead of hidden.
- The search row below the header only appears while the search has focus or a
  query is set, so an idle frame spends that row on content.
- Panes are drawn as fully bordered boxes that share their vertical edges, and
  the pane captions sit further inside the top border.
- The focused pane is highlighted: its border and caption use the accent color.
- The document path moved out of the pane body and the header into its own
  toolbar row directly above the footer, which gives every pane one more
  content row.
- The footer bar shows edit-mode bindings while editing instead of the browse
  bindings, and drops optional hints when the terminal is too narrow.

### File management

- `ctrl+n` creates a new Markdown file. The name is typed into the toolbar row,
  the file is created next to the current document, and `.md` is appended when
  the name carries no extension.
- `ctrl+p` opens a file action menu: new file, edit filename, delete file. It
  deliberately does not sit on `ctrl+k`, which deletes to the line end in edit
  mode. The menu covers the content pane with its own backdrop and centers its
  entries in it, left-aligned with each other.
- While the prompt or the action menu is open, the caret sits in that line and
  the buffer stops drawing its own, so the focus is unambiguous.
- Renaming refuses an existing target name and refuses to run while the buffer
  has unsaved changes.
- Deleting asks for confirmation and keeps a `<name>.md.bak` copy, so the delete
  stays recoverable.
- Create and rename are checked against the project root before anything is
  written, the same way opening and saving already were.

### Fixes carried in this fork

- Pasted text is no longer dropped in edit mode; multi-line pastes previously
  failed the control-character filter entirely.
- alt-modified keys no longer type their letter into the search field or the
  chat input.
- Space can be typed in edit mode; it arrives as its own key type and was
  previously dropped.
- Escape sequences and other control runes no longer leak into the buffer.
- The documented copy shortcut was corrected to `opt+c`: `cmd+c` cannot reach
  the program, because macOS routes it to Ghostty's *Edit > Copy* menu item.
- The edit-mode caret no longer pushes the rest of the line one column to the
  right; it covers the cell it sits on instead of being inserted before it.

### Under the hood

- `github.com/alecthomas/chroma/v2` and `github.com/muesli/termenv` are now
  direct dependencies, used by the code-block highlighter in
  `internal/render/highlight.go`.
- New packages/files: `internal/render/highlight.go`,
  `internal/editor/motion.go`, `internal/app/editing.go`,
  `internal/app/splash.go`, `internal/app/files.go`, each with tests.
- `docs.Document` carries the file's modification time, which the welcome screen
  orders by.
- The module path in `go.mod` is unchanged (`github.com/khw1031/glowed`), which
  keeps upstream merges clean but means `go install` cannot fetch this fork.

### Distribution

- Published as **glowed.md** at
  [floriankappert/glowed.md](https://github.com/floriankappert/glowed.md). The
  installed command stays `glowed`, so nothing about invoking it changes.
- Released through the `floriankappert/tap` Homebrew tap
  ([floriankappert/homebrew-tap](https://github.com/floriankappert/homebrew-tap)),
  with the upstream tap left untouched.
- Version numbers carry the `-floriankappert.N` suffix so a fork build is never
  mistaken for an upstream release.

## Features

- Scan `.md` files under the project root
- Automatically refresh by polling for Markdown files created, modified, deleted, or renamed while glowed is running
- Apply built-in scan ignores for common generated paths, with project-local `.glowedignore` overrides
- Search by title, Markdown body, filename/path, frontmatter, and `tag:` / `tags:` metadata
- Sidebar directory tree with expandable/collapsible folders
- Glamour-based Markdown preview
- Raw Markdown edit mode with word- and line-wise motion, deletion, and keyboard selection
- Syntax highlighting for fenced code blocks in edit and source mode
- Atomic save with backup
- Undo/redo in edit mode
- Welcome screen with the most recently modified documents, selectable on launch
- Create, rename, and delete Markdown files from inside the TUI
- Mouse click, wheel, and drag-based app-managed selection
- Source selection mode for copying exact original Markdown with metadata
- Footer action bar with clickable actions
- Configurable keymap and footer actions
- External LLM session launcher for any configured CLI command

## Installation

### From source

```bash
git clone https://github.com/floriankappert/glowed.md.git
cd glowed.md
go build -o ./bin/glowed ./cmd/glowed
./bin/glowed
```

Or install the binary somewhere on your `PATH`:

```bash
go build -o glowed ./cmd/glowed
install -m 0755 glowed ~/.local/bin/glowed
```

### With `go install`

`go install` resolves the module path declared in `go.mod`, which this fork
keeps at the upstream value. The command below therefore installs the
**upstream** build, not this fork:

```bash
go install github.com/khw1031/glowed/cmd/glowed@latest
```

To get this fork, use the Homebrew tap below or build from the clone above.

### Homebrew tap

The distribution model is a custom Homebrew tap first, not Homebrew core.

This fork is distributed through its own tap:

```bash
brew install floriankappert/tap/glowed
```

The upstream build lives in the maintainer's tap:

```bash
brew install khw1031/tap/glowed
```

Both formulae are named `glowed`, so install with the full tap path to avoid
ambiguity.

## Usage

Open the current directory as the project root:

```bash
glowed
```

Open a specific project root:

```bash
glowed /path/to/project
```

Open a specific Markdown file:

```bash
glowed /path/to/project/notes/file.md
```

Show help:

```bash
glowed --help
```

Create a project `.glowedignore` template for custom scan rules:

```bash
glowed --init-ignore /path/to/project
```

## Search

Press `/` to focus search. Search tokens are split on whitespace and combined with AND semantics: `foo bar` matches documents that contain both `foo` and `bar`.

Search covers:

- document title, from the first `# Heading` or frontmatter `title`
- Markdown body text, excluding the leading frontmatter block
- relative path and filename
- raw frontmatter text
- tags collected from frontmatter `tag` / `tags` fields and inline `tag:foo` markers

General query matches are ranked by source: title, body, frontmatter, path/filename, then tag. The sidebar snippet shows the match source, such as `title:`, `body:`, `frontmatter:`, `path:`, or `tag:foo`.

Use `tag:foo` to search tags specifically. The query syntax is `tag:foo`; `tags:foo` is not a query operator. For example, `notes tag:ai draft` matches documents whose title/body/path/frontmatter includes `notes` and `draft`, and whose tags include `ai`.

## Default key bindings

- `q`: quit
- `/`: focus search
- `tab`: cycle focus; when a sidebar directory is selected, expand/collapse it
- `enter`: open selected document / focus preview; when a sidebar directory is selected, expand/collapse it
- `e`: edit current document
- `v`: source selection mode
- `c`: open external LLM session
- `ctrl+s`: save in edit mode
- `ctrl+z`: undo in edit mode
- `ctrl+y`: redo in edit mode
- `esc`: cancel search/edit/source mode depending on context
- `r`: rescan project root manually; automatic polling refresh also reflects Markdown changes while glowed is running
- `ctrl+n`: create a new Markdown file, entering the name in the toolbar row
- `ctrl+p`: open the file action menu (new file, edit filename, delete file)
- `ctrl+g b`: toggle sidebar
- `ctrl+g l`: open external LLM session
- `ctrl+g r`: rescan
- `ctrl+g q`: quit

### Edit mode

Edit is the default mode: glowed opens the initial document — or the first one
it finds in the project — ready for editing, and documents opened from the
sidebar land in the editor as well. `esc` leaves the editor for the preview.

Editing supports word- and line-wise motion, deletion, and selection. On macOS,
Ghostty rewrites `cmd` and `opt` combinations into control sequences before they
reach the program, so both spellings below refer to the same binding.

| Keys | Action |
| --- | --- |
| `opt+←` / `opt+→` | move one word left/right |
| `cmd+←` / `cmd+→` (`ctrl+a` / `ctrl+e`, `home` / `end`) | move to line start/end |
| `opt+⌫` | delete the word before the caret |
| `opt+⌦` | delete the word after the caret |
| `cmd+⌫` (`ctrl+u`) | delete to line start |
| `ctrl+k` | delete to line end |
| `shift+←→↑↓` | extend the selection character- and line-wise |
| `opt+shift+←` / `opt+shift+→` | extend the selection word-wise |
| `shift+home` / `shift+end` | extend the selection to line start/end |
| `opt+a` | select the whole buffer |
| `opt+c` | copy the selection as plain text |
| `cmd+v` / `opt+v` | paste at the caret, replacing the selection |
| `esc` | clear the selection, or leave edit mode when nothing is selected |

Typing or deleting with an active selection replaces or removes it. A plain
arrow key collapses the selection to its start or end.

The sidebar is reachable without leaving the editor. `ctrl+b` and `shift+tab`
work in every mode, so they behave the same while editing and while browsing:

| Keys | Action |
| --- | --- |
| `ctrl+b` | show/hide the sidebar |
| `shift+tab` | move the focus between the content pane and the sidebar, opening the sidebar if hidden |
| `↑` / `↓` | select a row while the sidebar has focus |
| `enter` | open the selected document for editing, or expand/collapse a directory |

Switching to another document is refused while the current buffer has unsaved
changes; save with `ctrl+s` or discard with `esc` first. Saving keeps the buffer
open instead of returning to the preview.

`cmd+v` works out of the box: Ghostty pastes into the terminal and glowed
receives it as a bracketed paste, newlines included.

`cmd+c` and `cmd+a` cannot be forwarded at all. macOS routes them to Ghostty's
*Edit > Copy* and *Edit > Select All* menu items before the terminal sees them,
and a menu shortcut wins over any `keybind` entry — remapping `super+c` or
`super+a` in the Ghostty config has no effect. Copy and select-all are therefore
`opt+c` and `opt+a`, which need no configuration when `macos-option-as-alt` is
set.

Select-all deliberately does not sit on `ctrl+a`, because Ghostty sends exactly
that for `cmd+←` — the two are indistinguishable to the program.

If `macos-option-as-alt = true` costs you the bracket keys on a non-US layout,
see [the FAQ](#faq).



`cmd+z` / `cmd+shift+z` stay with Ghostty's own undo/redo; use `ctrl+z` and
`ctrl+y` instead.

Fenced code blocks that name a language are syntax highlighted in edit and
source mode, using the same colors as the preview. Set `preview.style` to change
the theme for both.

## Configuration

Configuration is loaded from:

1. `~/.config/glowed/config.json`
2. `<project-root>/.glowed.json`

Project-local config overrides global config.

Markdown scan ignore rules combine built-in defaults with optional project-local overrides from `<project-root>/.glowedignore`. Built-in defaults hide common VCS, dependency, cache, and root generated-output paths such as `.git/`, `node_modules/`, `vendor/`, `.cache/`, `/build/`, and `/dist/`. The syntax is gitignore-style; use `/build/` for the root build directory only, or `build/` for every directory named `build`. Rules in `.glowedignore` are applied after built-in defaults, so `!pattern` can re-include paths hidden by defaults. `.gitignore` is intentionally not read.

Use `glowed --init-ignore [project-root]` to create a starter `.glowedignore` template. It never overwrites an existing file.

While glowed is running, it uses a lightweight polling snapshot to detect note-relevant filesystem changes. The default polling interval is 5 seconds; when the Markdown path/size/modtime snapshot or `.glowedignore` fingerprint changes, glowed rescans the project. The manual refresh key (`r`) remains available for immediate refresh.

See:

- [`glowed.schema.json`](glowed.schema.json)
- [`.glowed.example.json`](.glowed.example.json)
- [`.glowedignore`](.glowedignore)

## Release changelog drafts

Before a release, generate LLM-drafted notes from the git log and diff, then publish the reviewed contents in `CHANGELOG.md`:

```bash
scripts/draft-changelog.sh vX.Y.Z --llm-cmd "codex exec --sandbox read-only -"
scripts/update-changelog.sh vX.Y.Z .release/notes-vX.Y.Z.md
scripts/extract-release-notes.sh vX.Y.Z /tmp/glowed-release-notes-vX.Y.Z.md
```

The generated `.release/` files are local draft artifacts. The public source of truth for each release is the version section in `CHANGELOG.md`; GitHub Release notes should be extracted from that section.

## External LLM sessions

`glowed` does not handle OAuth, passwords, or API keys directly.

Instead, it opens the external CLI command configured in `llm.command` and passes the current Markdown context through a temporary context file and clipboard prompt. The user is expected to have already installed and logged in to that CLI in their terminal environment.

`claude` and `codex` are examples, not the only supported commands. You can point `llm.command` to any interactive CLI on your `PATH`, including another LLM CLI or your own wrapper script.

Supported intent:

- Launch the CLI configured in `llm.command`
- Examples: `claude`, `codex`, `aider`, or a custom wrapper script
- Apply small command-specific launch defaults for known commands such as Claude/Codex when useful
- Open a Ghostty split when possible
- Include current file path, relative path, mode, optional selection, and optional raw Markdown context

## FAQ

### I cannot type `[`, `]`, `{`, `}`, `@`, `|` or `~` in Ghostty

This is Ghostty's `macos-option-as-alt` setting, not a glowed bug. On layouts
where those characters are Option-composed — German, Nordic and many others —
`macos-option-as-alt = true` turns Option into Alt, so the keystroke arrives as
`alt+5` rather than as the composed `[`. The composed character is never
produced, so no program can see it.

Set the option to one side instead of both:

```
macos-option-as-alt = left
```

Now the right Option composes characters as macOS normally does (`⌥5` → `[`),
while the left Option is still Alt for glowed's word motions (`opt+←→`),
`opt+c` and `opt+a`. Use `right` if you prefer the sides swapped.

On a US layout `macos-option-as-alt = true` is harmless, because nothing you
need is Option-composed.

When an alt combination arrives that glowed has no binding for, it says so in
the status line instead of dropping the key silently.

### Which characters are affected?

Anything the layout puts behind Option. On a German Mac layout that is at least
`[ ] { } @ | \ ~ €` — which includes the pipe used for Markdown tables, so it
is worth fixing before writing tables.

### Why is copy `opt+c` and not `cmd+c`?

macOS routes `cmd+c` to Ghostty's *Edit > Copy* menu item before the terminal
sees it, and a menu shortcut wins over any `keybind` entry. The same applies to
`cmd+a`. See [Edit mode](#edit-mode).

## Important limitations

Please read this section before using or distributing the project.

### Ghostty-first, not terminal-universal

`glowed` is currently designed around Ghostty. It may run in other terminals, but other terminal environments are not first-class targets yet.

Known risk areas:

- Mouse tracking
- Drag selection
- Wheel events
- `Cmd+Left` / `Cmd+Right` and other platform-specific key sequences
- Cursor shape behavior
- Alternate screen behavior
- Clipboard fallback through OSC52
- Terminal split launching for external LLM sessions

### Limited environment testing

The project has not yet been tested across a broad set of environments.

In particular, these are not guaranteed yet:

- iTerm2
- Terminal.app
- Alacritty
- Kitty
- WezTerm
- VS Code integrated terminal
- SSH sessions
- tmux/screen
- Linux desktop terminals
- Windows Terminal / WSL

The strongest expected path today is macOS + Ghostty.

### External LLM launch is environment-sensitive

The external LLM launcher assumes that the selected CLI command is already installed and logged in.

Ghostty split support is especially tailored to Ghostty. Other terminals may need different commands or may only open a separate window.

### Preview selection is not source mapping

Preview selection copies rendered preview text with metadata. It does not accurately map rendered terminal coordinates back to exact Markdown source ranges.

For exact original Markdown copying, use:

- edit/raw mode selection
- `sourceSelect` mode (`v`)

### Early editor warning

The editor performs backup + atomic save, but this is still early software. Use version control for important documents.

## Custom distributions

Homebrew tap namespaces are the recommended way to distinguish modified builds.

- The same formula name, `glowed`, can exist in different taps.
- For example, `khw1031/tap/glowed` and `floriankappert/tap/glowed` can both be distributed.
- Users should install with the full tap path, such as `brew install someone/tap/glowed`, to avoid ambiguity.
- You are encouraged to maintain and use your own tap/build freely for your workflow.
- A modified build may install the binary as `glowed` for drop-in use, or as `glowed-<name>` if it should coexist with other builds.
- This repository does not use external pull requests as the default contribution path.
- If you want to share your version back, please open a **Distribution registration** issue. The maintainer may review it and, if desired, prepare and merge changes here independently.
- If your build was customized with an AI agent or coding harness, it is recommended to state which agent/model/method you used.
- Known distributions may be listed in [`DISTRIBUTIONS.md`](DISTRIBUTIONS.md).

See [`CONTRIBUTING.md`](CONTRIBUTING.md) for contribution, distribution registration, and Homebrew tap guidance.

## Development

Run tests:

```bash
go test ./...
```

Run benchmarks:

```bash
go test -bench=. -benchmem ./internal/docs ./internal/render
```

Run locally:

```bash
go run ./cmd/glowed
```
