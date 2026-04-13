# Nvim Diff Viewer

Open the rendered diff in nvim for full vim interaction — visual selection,
yank, `/` search, and motions.

## Problem

The bubbletea viewport displays pre-rendered ANSI output from pagers
(delta, difftastic, bat) but provides no way to select or copy text.
Terminal mouse capture (`MouseModeCellMotion`) intercepts mouse events,
preventing native terminal selection. A line-based selection prototype
was attempted but proved too coarse, and reimplementing vim's visual mode
and motion grammar in bubbletea is not viable.

## Design

### User interaction

Press `v` while the diff viewer panel is active. Bubbletea suspends and
nvim opens with the rendered diff content displayed in a terminal buffer.
The user has full vim: visual mode (`v`/`V`/`<C-v>`), yank (`y`), `/`
search, and all standard motions. Quit nvim (`:q`) to return to the file
browser.

This is the same lifecycle pattern as the existing `o` (open in editor)
feature, using `tea.ExecProcess`.

### Nvim invocation

1. Write the current ANSI diff content (the string from
   `diffRenderedMsg` stored in the viewport) to a temp file.
2. Launch nvim:
   ```
   nvim --clean -R \
     -c 'terminal cat <tmpfile>' \
     -c 'stopinsert'
   ```
   - `--clean` — skip user config/plugins for fast, predictable startup.
   - `-R` — signal readonly intent.
   - `terminal cat <tmpfile>` — open a terminal buffer that renders the
     ANSI content faithfully.
   - `stopinsert` — enter terminal-normal mode immediately so vim motions
     work without pressing `<C-\><C-n>`.
3. On return, delete the temp file.

### Editor fallback

If `nvim` is not in `$PATH`, fall back to `vim`. If neither is found,
show an error in the message overlay.

`vim` supports `:terminal` as of version 8.1. The invocation differs:

```
vim -R --clean \
  -c 'terminal ++curwin cat <tmpfile>' \
  -c 'normal G'
```

`++curwin` prevents vim from opening the terminal in a split. Vim's
terminal mode does not have an equivalent to nvim's `stopinsert` via
`-c`, so the user may need to press `<C-\><C-n>` to enter normal mode.
This is an acceptable trade-off for a fallback path.

### Configuration

Two new fields in `UIConfig`:

```yaml
ui:
  diffViewer: "nvim"       # command to launch (default: auto-detect nvim, then vim)
  diffViewerClean: true    # pass --clean to skip user config (default: true)
```

When `diffViewer` is set to a custom command string, it is used as-is
with the temp file path appended. The user is responsible for flags.
`diffViewerClean` is only applied when using the default nvim/vim
detection.

### Keybinding

`v` — open diff in nvim. Mnemonic: visual mode. Not currently bound.

Added to the `KeyMap` struct and the help key groups.

## Cleanup

Remove the line-based selection prototype added in the current branch:

- `diffviewer.Model`: remove `selAnchor`, `selEnd`, and all selection
  methods (`StartSelection`, `ExtendSelection`, `FinishSelection`,
  `ClearSelection`, `HasSelection`, `zoneYToContentLine`, `selRange`,
  `applySelectionHighlight`).
- `mainModel`: remove `selectingText`, `mouseEnabled` fields.
- `keys.go`: remove `ToggleMouse` binding.
- `tui.go handleMouse`: remove click/drag/release selection handling and
  the mouse toggle key handler.

### Kept changes

- `MouseModeCellMotion` (was `MouseModeAllMotion`) — the app has no
  hover handlers; `CellMotion` covers clicks, scroll, and button-held
  drag (sidebar resize) with less noise.

## Scope

This spec covers only the `v` keybinding to launch nvim with rendered
diff content. It does not cover:

- In-app character-level selection (replaced by nvim)
- Changes to the pager/external diff pipeline
- Nvim plugin integration or filetype detection
