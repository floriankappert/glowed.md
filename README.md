# glowed.md

**A Ghostty-first terminal TUI Markdown browser and editor.**

Maintained by **[@floriankappert](https://github.com/floriankappert)** ·
originally created by **[@khw1031](https://github.com/khw1031)**

glowed.md is a fork of **[khw1031/glowed](https://github.com/khw1031/glowed)**.
The original is the base: it scans a project root for Markdown files, searches
and previews them, edits raw Markdown, copies selections with path metadata, and
launches an external LLM CLI with the current document as context.

> **This README only covers what is different here.** For what glowed does, how
> to use it and how to configure it, read the upstream documentation:
> **[khw1031/glowed](https://github.com/khw1031/glowed#readme)**.

Everything below is additional to upstream — no upstream feature was removed.
The fork branched off `v0.2.2` and is versioned `v0.2.2-floriankappert.N`, so a
fork build is never mistaken for an upstream release.

## Install

```bash
brew install floriankappert/glowed.md/glowed
```

Use the fully-qualified name. The upstream tap carries a formula named `glowed`
too, so a bare `brew install glowed` or `brew upgrade glowed` is ambiguous while
both taps are installed:

```bash
brew upgrade floriankappert/glowed.md/glowed
```

Or build from the clone:

```bash
git clone https://github.com/floriankappert/glowed.md.git
cd glowed.md
go build -o ./bin/glowed ./cmd/glowed
./bin/glowed
```

`go install` resolves the module path in `go.mod`, which this fork keeps at the
upstream value — so `go install github.com/khw1031/glowed/cmd/glowed@latest`
installs the **upstream** build, not this one.

The project is named glowed.md. The installed command, the Homebrew formula and
the config paths keep the name `glowed`, so invoking and configuring it is
unchanged.

## What is different here

Upstream's editor is an MVP raw-buffer mode. The work here makes it behave like
an editor and reshapes the surrounding UI.

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

- Edit is the default mode: documents opened from the sidebar open for editing.
  Projects without documents still start in the preview.
- `ctrl+b` toggles the sidebar in every mode, including edit mode where the
  browse bindings are unavailable.
- `shift+tab` moves the focus between the content pane and the sidebar in every
  mode, opening the sidebar when it is hidden.
- Sidebar navigation with `↑` / `↓` and `enter` opens the selected document
  directly in edit mode. The switch is refused while the current buffer has
  unsaved changes.

### Welcome screen

- Launching without a file shows a welcome screen: a lightbulb mark next to the
  version and project root, plus the 7 most recently modified documents under a
  *Recent files* heading, newest first. `↑` / `↓` select, `enter` opens one in
  the editor.
- It stays up until a file is picked, and is skipped when a file is passed on
  the command line — an explicit argument is the selection.
- `ctrl+n` and `ctrl+p` work there too, so an empty project is not a dead end.
  The action menu targets the highlighted recent file and names it in its title.
- *go home* in the action menu returns to it.

### Layout

- The header above the panes carries the lightbulb mark, with the name beside
  it, the mode on the second row and the status message on the third, followed
  by a blank row that separates the block from the panes. The focus name and the
  file name are deliberately absent: the focused pane is already highlighted by
  its caption, and the path sits in the bottom row. Below a terminal height of
  12 rows the header collapses into a single row, so short splits still render
  a frame that fits.
- The sidebar pane is captioned *Files & Folders*.
- The sidebar is visible on launch instead of hidden.
- The search row below the header only appears while the search has focus or a
  query is set, so an idle frame spends that row on content.
- Panes are drawn as fully bordered boxes that share their vertical edges, and
  the pane captions sit further inside the top border.
- The focused pane is highlighted: its border and caption use the accent color.
- The footer hint bar is gone. Its bindings live in the action menu, and the
  bottom row is the document path plus a `ctrl+p actions` pointer.

### File management

- `ctrl+n` creates a new Markdown file. The name is typed into the bottom row,
  the file is created next to the current document, and `.md` is appended when
  the name carries no extension.
- `ctrl+p` opens the action menu: new file, edit filename, delete file, go home,
  followed by the runnable actions of the current mode and a reference list of
  the keys it cannot run. It deliberately does not sit on `ctrl+k`, which
  deletes to the line end in edit mode.
- The menu covers the content pane — the whole screen on the welcome screen —
  with its own dark backdrop, centered, its rows left-aligned with each other.
  On a pane too short for all of it, the reference keys are dropped first and
  the list scrolls to keep the selection visible.
- While the prompt or the action menu is open, the caret sits in that line and
  the buffer stops drawing its own, so the focus is unambiguous.
- Renaming refuses an existing target name and refuses to run while the buffer
  has unsaved changes.
- Deleting asks for confirmation and keeps a `<name>.md.bak` copy, so the
  delete stays recoverable.
- Create and rename are checked against the project root before anything is
  written, the same way opening and saving already were.

### Fixes carried in this fork

- The edit-mode caret no longer pushes the rest of the line one column to the
  right; it covers the cell it sits on instead of being inserted before it.
- Pasted text is no longer dropped in edit mode; multi-line pastes previously
  failed the control-character filter entirely.
- Space can be typed in edit mode; it arrives as its own key type and was
  previously dropped.
- Escape sequences and other control runes no longer leak into the buffer.
- alt-modified keys no longer type their letter into the search field or the
  chat input, and an alt combination with no binding now says so in the status
  line instead of being dropped silently (see the [FAQ](#faq)).
- The documented copy shortcut was corrected to `opt+c`: `cmd+c` cannot reach
  the program, because macOS routes it to Ghostty's *Edit > Copy* menu item.

### Under the hood

- `github.com/alecthomas/chroma/v2` and `github.com/muesli/termenv` are direct
  dependencies, used by the code-block highlighter in
  `internal/render/highlight.go`.
- New files: `internal/render/highlight.go`, `internal/editor/motion.go`,
  `internal/app/editing.go`, `internal/app/splash.go`, `internal/app/files.go`,
  each with tests.
- `docs.Document` carries the file's modification time, which the welcome
  screen orders by, and `docs.GuardNewPath` guards paths that do not exist yet.
- The module path in `go.mod` is unchanged (`github.com/khw1031/glowed`), which
  keeps upstream merges clean.

## Key bindings that changed

On macOS, Ghostty rewrites `cmd` and `opt` combinations into control sequences
before they reach the program, so both spellings below refer to the same
binding.

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

| Keys | Action |
| --- | --- |
| `ctrl+p` | open the action menu |
| `ctrl+n` | create a new Markdown file |
| `ctrl+b` | show/hide the sidebar, in every mode |
| `shift+tab` | move the focus between content pane and sidebar, opening it if hidden |
| `↑` / `↓` | select a row in the sidebar, the welcome screen or the action menu |
| `enter` | open the selected document, run the selected action, or expand a directory |

`cmd+z` / `cmd+shift+z` stay with Ghostty's own undo/redo; use `ctrl+z` and
`ctrl+y` instead.

## FAQ

### I cannot type `[`, `]`, `{`, `}`, `@`, `|` or `~` in Ghostty

This is Ghostty's `macos-option-as-alt` setting, not a glowed.md bug. On layouts
where those characters are Option-composed — German, Nordic and many others —
`macos-option-as-alt = true` turns Option into Alt, so the keystroke arrives as
`alt+5` rather than as the composed `[`. The composed character is never
produced, so no program can see it.

Set the option to one side instead of both:

```
macos-option-as-alt = left
```

Now the right Option composes characters as macOS normally does (`⌥5` → `[`),
while the left Option is still Alt for the word motions (`opt+←→`), `opt+c` and
`opt+a`. Use `right` if you prefer the sides swapped.

On a US layout `macos-option-as-alt = true` is harmless, because nothing you
need is Option-composed. Anything the layout puts behind Option is affected; on
a German Mac layout that is at least `[ ] { } @ | \ ~ €` — which includes the
pipe used for Markdown tables, so it is worth fixing before writing tables.

When an alt combination arrives that glowed.md has no binding for, it says so in
the status line instead of dropping the key silently.

### Why is copy `opt+c` and not `cmd+c`?

macOS routes `cmd+c` to Ghostty's *Edit > Copy* menu item before the terminal
sees it, and a menu shortcut wins over any `keybind` entry — remapping `super+c`
has no effect. The same applies to `cmd+a`. Copy and select-all are therefore
`opt+c` and `opt+a`.

Select-all deliberately does not sit on `ctrl+a`, because Ghostty sends exactly
that for `cmd+←` — the two are indistinguishable to the program.

## Changelog

[`CHANGELOG.md`](CHANGELOG.md) is the source of truth for what shipped in each
release, upstream sections included.

## Development

```bash
go test ./...
go build -o ./bin/glowed ./cmd/glowed
```

Terminal behaviour is environment-sensitive; this fork is developed and tested
on macOS with Ghostty.

## Upstream

Issues that are not specific to this fork belong
[upstream](https://github.com/khw1031/glowed/issues). Upstream's contribution
model asks downstream builds to be published as their own Homebrew tap rather
than as pull requests, which is what this fork does — see upstream's
[CONTRIBUTING.md](https://github.com/khw1031/glowed/blob/main/CONTRIBUTING.md).

The original project is MIT-licensed by
[@khw1031](https://github.com/khw1031); see [`LICENSE`](LICENSE).
