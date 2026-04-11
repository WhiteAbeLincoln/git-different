# git-different: Design Spec

Fork of [diffnav](https://github.com/dlvhdr/diffnav) — a bubbletea TUI for navigating git diffs with a file tree sidebar and per-file diff rendering. This fork replaces the stdin-based pager model with a git-aware CLI, adds a configurable pager/external-diff system, and introduces a commit-segmented file tree view.

## CLI Interface

`git-different` accepts git diff arguments directly and invokes git internally:

```
git-different [flags] [git-diff-args...]
git different main...feature-branch     # via git auto-discovery
git different HEAD~3
git different --staged
git different                           # working tree vs index
```

**Flags:**
- `--pager <cmd>` — override configured pager
- `--external-diff <cmd>` — override configured external diff tool
- `--watch` / `--watch-interval` — re-run git diff periodically

No stdin support. The binary name `git-different` enables `git different` subcommand auto-discovery.

## Architecture — Input Model

### Current diffnav flow

```
git diff | diffnav → parse stdin → file tree + hardcoded delta rendering
```

### New git-different flow

```
git-different <args>
  → git diff --name-status <args>   → file list + statuses → file tree
  → git diff <args>                 → unified diff → preamble parsing
  → on file select (pager mode):      git diff <args> -- <file> | <pager>
  → on file select (ext-diff mode):   git -c diff.external=<tool> diff <args> -- <file>
  → on dir select:                    same, with directory pathspec
```

### Key changes to cmd/root.go

- Drop all stdin reading logic
- Store git diff args on the model
- Initial file tree: `git diff --name-status <args>` for file list, `git diff <args>` for preamble/parsing
- On-demand rendering: invoke git per-file when a node is selected instead of slicing pre-parsed patches

### Commit enumeration (for segmented view)

When the diff range contains multiple commits:
- `git log --format='%H %s' <args>` to enumerate commits
- `git diff-tree --no-commit-id -r <hash>` per commit for that commit's file list
- On file select: `git diff <parent>..<hash> -- <file>` piped to pager/ext-diff

## Pager Configuration

Two mutually exclusive rendering modes:

```yaml
# ~/.config/git-different/config.yml
ui:
  pager: "delta --paging=never --side-by-side"
  externalDiff: "difft --display=inline"
```

### Resolution order (highest to lowest priority)

1. CLI flags (`--pager`, `--external-diff`)
2. Config file
3. Default: `pager: "delta --paging=never"`

### Behavior

- If `externalDiff` is set (via config or CLI flag), it takes precedence — pager is ignored
- Pager mode: `git diff <args> -- <file> | <pager>`
- External diff mode: `git -c diff.external=<tool> diff <args> -- <file>`
- The pager/externalDiff value is the full command string including flags — the user is responsible for passing the right flags to their tool

### Pager vs External Diff

These are fundamentally different pipelines:
- **Pager** (delta, bat, less): receives unified diff on stdin, colorizes/renders it
- **External diff** (difftastic): git invokes the tool with file paths, the tool computes and renders its own diff

## File Tree — Commit Segmentation

Two view modes, toggled with `c`:

### Unified view (default)

All changed files across the entire diff range in one tree, deduplicated:

```
▸ src/
    M auth.go
    M server.go
    A auth_test.go
  D old_config.go
```

### Commit-segmented view

Files grouped under commit headers. A file changed in multiple commits appears under each:

```
▸ abc1234 Fix auth bug
    M src/auth.go
    A src/auth_test.go
▸ def5678 Add logging
    M src/server.go
    M src/auth.go
```

### Behavior

- Commits in chronological order (oldest first)
- Selecting a file in segmented mode shows the diff for that file within that specific commit (not the cumulative diff)
- Toggle only available when the diff range contains multiple commits — for `git different --staged` or `git different` (working tree), the toggle is hidden/no-op
- Navigation keys work identically in both modes

## Keybinding Changes

### Removed
- `s` — toggle side-by-side (user controls via pager flags)

### Added
- `c` — toggle unified/commit-segmented file tree view

### Unchanged
- `j`/`k`/`↑`/`↓` — navigate tree
- `n`/`p` — next/prev file (skip dirs)
- `h`/`l` — collapse/expand node
- `enter` — toggle fold
- `tab`/`shift+tab` — switch pane
- `ctrl+d`/`ctrl+u` — scroll diff
- `e` — toggle file tree visibility
- `t` — search files
- `y` — copy file path
- `o` — open in editor
- `i` — cycle icon style
- `?` — help
- `m` — commit info overlay
- `q` — quit

## Changes to Codebase

| Area | File(s) | Change |
|---|---|---|
| Binary rename | `main.go`, `flake.nix`, config paths | `diffnav` → `git-different`, config dir `~/.config/git-different/` |
| Input model | `cmd/root.go` | Drop stdin, accept git diff args, invoke git directly |
| Git integration | New `pkg/git/` package | Wraps git commands: `diff`, `diff-tree`, `log`. Handles ref parsing, pathspec filtering |
| Pager rendering | `pkg/ui/panes/diffviewer/diffviewer.go` | Replace hardcoded delta with configurable pager/external-diff invocation |
| Config | `pkg/config/config.go` | Add `pager` and `externalDiff` fields, drop `sideBySide`, rename config dir |
| Commit-segmented tree | `pkg/ui/panes/filetree/filetree.go`, `pkg/ui/tui.go` | New view mode grouping files by commit, `c` toggle |
| Keybindings | `pkg/ui/keys.go` | Remove `s`, add `c` |
| Watch mode | `pkg/watch/watch.go`, `cmd/root.go` | Re-run `git diff <args>` instead of custom command |
| CLI flags | `cmd/root.go` | Drop `--side-by-side`/`--unified`, add `--pager`/`--external-diff` |

### Not changing
File tree core structure (dirnode/filenode), search, mouse handling, help overlay, icon cycling, editor integration.
