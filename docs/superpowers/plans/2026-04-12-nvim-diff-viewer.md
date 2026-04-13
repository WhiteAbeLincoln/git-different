# Nvim Diff Viewer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Press `v` to open the rendered diff in nvim for visual selection, yank, search, and motions.

**Architecture:** Write diff content to a temp file, suspend bubbletea with `tea.ExecProcess`, launch nvim/vim with a terminal buffer that renders the ANSI output. Also clean up the line-based selection prototype that this replaces.

**Tech Stack:** bubbletea v2 (`tea.ExecProcess`), `os/exec`, `os.CreateTemp`

---

### Task 1: Clean Up Selection Prototype from diffviewer.go

**Files:**
- Modify: `pkg/ui/panes/diffviewer/diffviewer.go`

- [ ] **Step 1: Remove selection fields from Model struct**

Replace the current Model struct:

```go
type Model struct {
	preamble  string
	vp        viewport.Model
	header    headerData
	selAnchor int // content line where selection started (-1 = none)
	selEnd    int // content line where selection currently ends
	common.Common
}
```

With:

```go
type Model struct {
	preamble string
	vp       viewport.Model
	header   headerData
	common.Common
}
```

- [ ] **Step 2: Revert New() to remove selection init**

Replace:

```go
func New() Model {
	return Model{
		vp:        viewport.Model{},
		selAnchor: -1,
		selEnd:    -1,
	}
}
```

With:

```go
func New() Model {
	return Model{
		vp: viewport.Model{},
	}
}
```

- [ ] **Step 3: Revert View() to remove selection highlight call**

Replace:

```go
func (m Model) View() string {
	vpView := m.vp.View()
	if m.selAnchor >= 0 {
		vpView = m.applySelectionHighlight(vpView)
	}
	scrollbar := common.RenderScrollbar(m.vp.Height(), m.vp.TotalLineCount(), m.vp.YOffset())
	if scrollbar != "" {
		vpView = lipgloss.JoinHorizontal(lipgloss.Top, vpView, " ", scrollbar)
	}
	return lipgloss.JoinVertical(lipgloss.Left, m.headerView(), vpView)
}
```

With:

```go
func (m Model) View() string {
	vpView := m.vp.View()
	scrollbar := common.RenderScrollbar(m.vp.Height(), m.vp.TotalLineCount(), m.vp.YOffset())
	if scrollbar != "" {
		vpView = lipgloss.JoinHorizontal(lipgloss.Top, vpView, " ", scrollbar)
	}
	return lipgloss.JoinVertical(lipgloss.Left, m.headerView(), vpView)
}
```

- [ ] **Step 4: Delete all selection methods and applySelectionHighlight**

Delete the following functions entirely (they appear after `ScrollDown`):
- `StartSelection`
- `ExtendSelection`
- `FinishSelection`
- `ClearSelection`
- `HasSelection`
- `zoneYToContentLine`
- `selRange`
- `applySelectionHighlight`

- [ ] **Step 5: Remove unused `ansi` import**

The `"github.com/charmbracelet/x/ansi"` import is used by both
`applySelectionHighlight` (deleted) and `SetContent` (kept). Check that
`SetContent` still uses `ansi.Truncate` — if so, keep the import. If
`ansi` is only referenced by deleted code, remove it.

`SetContent` uses `ansi.Truncate`, so the import stays.

- [ ] **Step 6: Build and run tests**

Run: `go build ./... && go test ./...`
Expected: All tests pass, no compile errors.

- [ ] **Step 7: Commit**

```bash
git add pkg/ui/panes/diffviewer/diffviewer.go
git commit -m "Remove line-based selection prototype from diffviewer

The nvim-based approach replaces in-app selection entirely."
```

---

### Task 2: Clean Up Selection Prototype from tui.go and keys.go

**Files:**
- Modify: `pkg/ui/tui.go`
- Modify: `pkg/ui/keys.go`

- [ ] **Step 1: Remove ToggleMouse and mouseEnabled from keys.go**

Remove the `ToggleMouse` field from the `KeyMap` struct:

```go
// Delete this field:
ToggleMouse      key.Binding
```

Remove the `ToggleMouse` binding from the `keys` var:

```go
// Delete this block:
ToggleMouse: key.NewBinding(
	key.WithKeys("M"),
	key.WithHelp("M", "toggle mouse"),
),
```

Remove `keys.ToggleMouse` from `KeyGroups()`. Change:

```go
}, {
	keys.ToggleMessage,
	keys.ToggleMouse,
	keys.ToggleHelp,
	keys.Quit,
}}
```

To:

```go
}, {
	keys.ToggleMessage,
	keys.ToggleHelp,
	keys.Quit,
}}
```

- [ ] **Step 2: Remove selectingText and mouseEnabled fields from mainModel**

In the `mainModel` struct, delete these two fields:

```go
selectingText     bool
mouseEnabled      bool
```

- [ ] **Step 3: Remove mouseEnabled init from New()**

In `New()`, delete:

```go
mouseEnabled:      true,
```

- [ ] **Step 4: Remove ToggleMouse key handler from Update()**

Delete this case from the `tea.KeyPressMsg` switch:

```go
case key.Matches(msg, keys.ToggleMouse):
	m.mouseEnabled = !m.mouseEnabled
	return m, tea.Batch(cmds...)
```

- [ ] **Step 5: Remove conditional mouse mode from View()**

In `View()`, replace:

```go
view.AltScreen = true
if m.mouseEnabled {
	view.MouseMode = tea.MouseModeCellMotion
}
```

With:

```go
view.AltScreen = true
view.MouseMode = tea.MouseModeCellMotion
```

- [ ] **Step 6: Remove selection handling from handleMouse()**

In `handleMouse()`, within the `tea.MouseClickMsg` / `MouseLeft` branch,
delete the diff viewer selection block:

```go
// Start text selection in diff viewer.
if zone.Get(zoneDiffViewer).InBounds(msg) {
	_, y := zone.Get(zoneDiffViewer).Pos(msg)
	m.diffViewer.StartSelection(y)
	m.selectingText = true
	return m, nil
}
```

Also delete the selection-clear line after the MouseLeft closing brace:

```go
// Any non-diff-viewer click clears an existing selection.
m.diffViewer.ClearSelection()
```

In the `tea.MouseReleaseMsg` case, remove the selection finalization.
Change:

```go
case tea.MouseReleaseMsg:
	m.draggingSidebar = false
	if m.selectingText {
		m.selectingText = false
		if text := m.diffViewer.FinishSelection(); text != "" {
			return m, tea.SetClipboard(text)
		}
	}
```

To:

```go
case tea.MouseReleaseMsg:
	m.draggingSidebar = false
```

In the `tea.MouseMotionMsg` case, remove the selection drag handling.
Change:

```go
case tea.MouseMotionMsg:
	if m.draggingSidebar {
		return m.handleSidebarDrag(msg)
	}
	if m.selectingText && zone.Get(zoneDiffViewer).InBounds(msg) {
		_, y := zone.Get(zoneDiffViewer).Pos(msg)
		m.diffViewer.ExtendSelection(y)
		return m, nil
	}
```

To:

```go
case tea.MouseMotionMsg:
	if m.draggingSidebar {
		return m.handleSidebarDrag(msg)
	}
```

- [ ] **Step 7: Remove ClearSelection call from setNodeDiff()**

In `setNodeDiff()`, delete:

```go
m.diffViewer.ClearSelection()
```

- [ ] **Step 8: Build and run tests**

Run: `go build ./... && go test ./...`
Expected: All tests pass, no compile errors.

- [ ] **Step 9: Run linters**

Run: `golangci-lint run --fix ./... && golangci-lint fmt ./... && betteralign -apply ./...`
Expected: No issues.

- [ ] **Step 10: Commit**

```bash
git add pkg/ui/tui.go pkg/ui/keys.go
git commit -m "Remove selection prototype state from tui and keys

Clears ToggleMouse binding, selectingText/mouseEnabled fields,
and all mouse-based selection handling. MouseModeCellMotion is
now unconditional."
```

---

### Task 3: Add DiffViewer and DiffViewerClean Config Fields

**Files:**
- Modify: `pkg/config/config.go`
- Test: `pkg/config/config_test.go`

- [ ] **Step 1: Check existing config tests for patterns**

Read `pkg/config/config_test.go` to understand the test style. The config
package uses the standard `testing` package (not Ginkgo).

- [ ] **Step 2: Write test for DiffViewer config loading**

Add a test to `pkg/config/config_test.go`:

```go
func TestLoadDiffViewerConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GIT_DIFFERENT_CONFIG_DIR", dir)
	configPath := filepath.Join(dir, "config.yml")
	err := os.WriteFile(configPath, []byte(`
ui:
  diffViewer: "vim"
  diffViewerClean: false
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	cfg := Load()
	if cfg.UI.DiffViewer != "vim" {
		t.Fatalf("expected DiffViewer=%q, got %q", "vim", cfg.UI.DiffViewer)
	}
	if cfg.UI.DiffViewerClean != false {
		t.Fatal("expected DiffViewerClean=false")
	}
}

func TestDefaultDiffViewerCleanIsTrue(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.UI.DiffViewerClean != true {
		t.Fatal("expected default DiffViewerClean=true")
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./pkg/config/ -run TestLoadDiffViewerConfig -v && go test ./pkg/config/ -run TestDefaultDiffViewerCleanIsTrue -v`
Expected: FAIL — fields don't exist yet.

- [ ] **Step 4: Add fields to UIConfig**

In `pkg/config/config.go`, add to the `UIConfig` struct after the
`ShowDiffStats` field:

```go
DiffViewer      string `yaml:"diffViewer"`
DiffViewerClean bool   `yaml:"diffViewerClean"`
```

- [ ] **Step 5: Set default in DefaultConfig()**

In `DefaultConfig()`, add to the `UIConfig` literal:

```go
DiffViewerClean: true,
```

(`DiffViewer` defaults to `""` which means auto-detect.)

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./pkg/config/ -v`
Expected: All tests pass.

- [ ] **Step 7: Commit**

```bash
git add pkg/config/config.go pkg/config/config_test.go
git commit -m "Add DiffViewer and DiffViewerClean config fields

DiffViewer specifies the command to launch for interactive diff
viewing (default: auto-detect nvim then vim). DiffViewerClean
controls whether --clean is passed (default: true)."
```

---

### Task 4: Add `v` Keybinding

**Files:**
- Modify: `pkg/ui/keys.go`

- [ ] **Step 1: Add ViewInNvim to KeyMap struct**

Add after `OpenInEditor`:

```go
ViewInNvim       key.Binding
```

- [ ] **Step 2: Add binding to keys var**

Add after the `OpenInEditor` binding:

```go
ViewInNvim: key.NewBinding(
	key.WithKeys("v"),
	key.WithHelp("v", "view in nvim"),
),
```

- [ ] **Step 3: Add to KeyGroups()**

Add `keys.ViewInNvim` to the second group (alongside `OpenInEditor`):

```go
}, {
	keys.ToggleFileTree,
	keys.Search,
	keys.Copy,
	keys.OpenInEditor,
	keys.ViewInNvim,
	keys.ToggleCommitView,
	keys.ToggleIconStyle,
}, {
```

- [ ] **Step 4: Build**

Run: `go build ./...`
Expected: Compiles. (The key is defined but not yet handled in Update.)

- [ ] **Step 5: Commit**

```bash
git add pkg/ui/keys.go
git commit -m "Add v keybinding for viewing diff in nvim"
```

---

### Task 5: Implement openInDiffViewer and Wire It Up

**Files:**
- Modify: `pkg/ui/panes/diffviewer/diffviewer.go`
- Modify: `pkg/ui/tui.go`

- [ ] **Step 1: Add GetContent method to diffviewer.Model**

In `pkg/ui/panes/diffviewer/diffviewer.go`, add after `ScrollDown`:

```go
// GetContent returns the full viewport content (ANSI-styled).
func (m Model) GetContent() string {
	return m.vp.GetContent()
}
```

- [ ] **Step 2: Implement openInDiffViewer in tui.go**

In `pkg/ui/tui.go`, add after the `openInEditor` function:

```go
func (m mainModel) openInDiffViewer() tea.Cmd {
	content := m.diffViewer.GetContent()
	if content == "" {
		return nil
	}

	viewer, args := m.resolveDiffViewer()
	if viewer == "" {
		log.Warn("no diff viewer found; install nvim or vim, or set ui.diffViewer in config")
		return nil
	}

	tmpFile, err := os.CreateTemp("", "git-different-*.ansi")
	if err != nil {
		return nil
	}
	if _, err := tmpFile.WriteString(content); err != nil {
		os.Remove(tmpFile.Name())
		return nil
	}
	tmpFile.Close()

	args = append(args, m.diffViewerTerminalArgs(viewer, tmpFile.Name())...)
	c := exec.Command(viewer, args...)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		os.Remove(tmpFile.Name())
		return nil
	})
}

// resolveDiffViewer returns the viewer binary and base args.
// Returns ("", nil) if no viewer is found.
func (m mainModel) resolveDiffViewer() (string, []string) {
	if custom := m.config.UI.DiffViewer; custom != "" {
		parts := strings.Fields(custom)
		return parts[0], parts[1:]
	}

	// Auto-detect: nvim first, then vim.
	if path, err := exec.LookPath("nvim"); err == nil {
		return path, nil
	}
	if path, err := exec.LookPath("vim"); err == nil {
		return path, nil
	}
	return "", nil
}

// diffViewerTerminalArgs returns the -c flags to open a terminal buffer
// for the given viewer and temp file path.
func (m mainModel) diffViewerTerminalArgs(viewer, tmpPath string) []string {
	base := filepath.Base(viewer)
	isNvim := base == "nvim"
	isVim := base == "vim"

	var args []string
	if (isNvim || isVim) && m.config.UI.DiffViewerClean {
		args = append(args, "--clean")
	}
	if isNvim || isVim {
		args = append(args, "-R")
	}

	switch {
	case isNvim:
		args = append(args,
			"-c", "terminal cat "+tmpPath,
			"-c", "stopinsert",
		)
	case isVim:
		args = append(args,
			"-c", "terminal ++curwin cat "+tmpPath,
			"-c", "normal G",
		)
	default:
		// Custom viewer: just append the temp file path.
		args = append(args, tmpPath)
	}
	return args
}
```

- [ ] **Step 3: Add the `path/filepath` import if not already present**

Check the imports in `tui.go` — `path/filepath` is already imported
(used by `openInEditor`). No change needed.

- [ ] **Step 4: Wire the keybinding in Update()**

In the `tea.KeyPressMsg` switch, add after the `OpenInEditor` case:

```go
case key.Matches(msg, keys.ViewInNvim):
	cmd = m.openInDiffViewer()
	if cmd != nil {
		cmds = append(cmds, cmd)
	}
```

- [ ] **Step 5: Build and run tests**

Run: `go build ./... && go test ./...`
Expected: All tests pass, compiles cleanly.

- [ ] **Step 6: Run linters**

Run: `golangci-lint run --fix ./... && golangci-lint fmt ./... && betteralign -apply ./...`
Expected: No issues.

- [ ] **Step 7: Commit**

```bash
git add pkg/ui/panes/diffviewer/diffviewer.go pkg/ui/tui.go
git commit -m "Add v keybinding to open rendered diff in nvim

Writes ANSI diff content to a temp file and launches nvim (or
vim) with a terminal buffer. Supports custom viewer command and
--clean toggle via config."
```

---

### Task 6: Manual Testing

**Files:** None (testing only)

- [ ] **Step 1: Build and run the app**

```bash
go build -o git-different . && ./git-different
```

- [ ] **Step 2: Test nvim launch**

With a file selected in the diff viewer, press `v`. Verify:
- nvim opens with ANSI-colored diff content
- The content renders correctly (colors from delta/bat/difftastic)
- Vim motions work (hjkl, gg, G, /, n, N)
- Visual mode works (v for char, V for line, Ctrl-V for block)
- Yanking works (select text, press y)
- `:q` returns to the file browser
- The file browser state is preserved (same file selected)

- [ ] **Step 3: Test vim fallback**

Temporarily rename nvim or set `diffViewer: "vim"` in config. Press `v`.
Verify vim opens with terminal buffer and ANSI renders.

- [ ] **Step 4: Test custom viewer config**

Set `diffViewer: "less -R"` in config. Press `v`. Verify less opens with
colored diff content.

- [ ] **Step 5: Test mouse still works**

Verify after all changes:
- Scroll wheel works in file tree and diff viewer
- Clicking file tree entries selects them
- Sidebar drag resize works
- Search box clicking works

- [ ] **Step 6: Test that old selection behavior is gone**

Click and drag in the diff viewer. Verify no selection highlight appears
and no text is copied to clipboard.
