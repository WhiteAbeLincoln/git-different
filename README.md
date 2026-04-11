# git-different

A git diff TUI with a file tree sidebar, configurable pager, and commit-segmented view.

> **This is a fork of [`diffnav`](https://github.com/dlvhdr/diffnav) by
> [@dlvhdr](https://github.com/dlvhdr).** All credit for the original design,
> TUI, and file tree implementation goes to the upstream project. This fork
> changes the input model from stdin-piped to direct git invocation, makes the
> pager/external-diff tool configurable, and adds a commit-segmented file tree
> view. If you want a drop-in `git diff` pager with delta-style rendering, the
> upstream project is probably what you want.

## Differences from diffnav

- **Direct git invocation.** Instead of `git diff | diffnav`, invoke as
  `git-different <git-diff-args>` (or `git different <args>` via git's
  subcommand auto-discovery). `git-different` runs git itself, so it knows the
  ref range, the commits in it, and the files in each commit.
- **Configurable pager and external diff tool.** Use anything that consumes a
  unified diff on stdin (`delta`, `bat`, `diff-so-fancy`) or any git external
  diff tool (`difftastic`). Set a default in `config.yml`, override per-run
  with `--pager` / `--external-diff`.
- **Commit-segmented file tree.** When the diff range spans multiple commits,
  press <kbd>c</kbd> to group files under commit headers and view each file's
  per-commit diff in isolation. Great for reviewing a branch commit-by-commit.
- **No stdin.** `git-different` does not read stdin. Arguments are git diff
  arguments — whatever you'd pass to `git diff`, you pass here.

## Installation

### Nix flake

```sh
nix run github:WhiteAbeLincoln/git-different
# or install into a profile
nix profile install github:WhiteAbeLincoln/git-different
```

The flake also exposes a dev shell (`nix develop`) with everything required to
build and lint the project.

### Go install

```sh
go install github.com/WhiteAbeLincoln/git-different@latest
```

> [!NOTE]
> Nerd Font icons require a Nerd Font installed and selected in your
> terminal. See https://www.nerdfonts.com/. If you don't want to install
> a Nerd Font, set `ui.icons: unicode` or `ui.icons: ascii` in your config.

## Usage

Because the binary is named `git-different`, git's subcommand auto-discovery
lets you invoke it as `git different`:

```sh
# working tree vs index (default)
git different

# staged changes
git different --staged

# last 3 commits
git different HEAD~3

# between branches
git different main...feature-branch

# with a specific file or directory
git different HEAD~5 -- src/

# with watch mode (re-runs git diff periodically)
git different --watch --watch-interval 5s HEAD
```

### Flags

| Flag              | Description                                                    |
| ----------------- | -------------------------------------------------------------- |
| `--pager`         | Pager command (overrides config)                               |
| `--external-diff` | External diff tool (overrides config)                          |
| `--watch, -w`     | Watch mode: periodically re-run git diff and refresh           |
| `--watch-interval`| Interval between watch refreshes (default: `2s`)               |

## Configuration

Config file is searched in this order:

1. `$GIT_DIFFERENT_CONFIG_DIR/config.yml` (if env var is set)
2. `$XDG_CONFIG_HOME/git-different/config.yml` (if set, macOS only)
3. `~/.config/git-different/config.yml` (macOS and Linux)
4. OS-specific config directory (e.g., `~/Library/Application Support/git-different/config.yml` on macOS)

Example:

```yaml
ui:
  # Pager command. Any tool that consumes a unified diff on stdin.
  pager: "delta --paging=never --side-by-side"

  # External diff tool. When set, takes precedence over `pager`.
  # git invokes the tool directly with file paths.
  externalDiff: "difft --display=side-by-side"

  # Icon style: "nerd-fonts-status" (default), "nerd-fonts-simple",
  # "nerd-fonts-filetype", "nerd-fonts-full", "unicode", or "ascii"
  icons: nerd-fonts-status

  # Start with the file tree hidden (toggle with 'e')
  showFileTree: true

  # File tree width
  fileTreeWidth: 30

  # Search panel width
  searchTreeWidth: 50

  # Hide the header and footer for more diff space
  hideHeader: false
  hideFooter: false

  # Color filenames by git status
  colorFileNames: true

  # Show diff stats next to each file
  showDiffStats: true
```

| Option               | Type   | Default                 | Description                                   |
| :------------------- | :----- | :---------------------- | :-------------------------------------------- |
| `ui.pager`           | string | `delta --paging=never`  | Pager command (reads unified diff on stdin)   |
| `ui.externalDiff`    | string | `""`                    | External diff tool (takes precedence)         |
| `ui.icons`           | string | `nerd-fonts-status`     | Icon style (see Icon Styles below)            |
| `ui.showFileTree`    | bool   | `true`                  | Show file tree on startup                     |
| `ui.fileTreeWidth`   | int    | `30`                    | Width of the file tree sidebar                |
| `ui.searchTreeWidth` | int    | `50`                    | Width of the search panel                     |
| `ui.hideHeader`      | bool   | `false`                 | Hide the header                               |
| `ui.hideFooter`      | bool   | `false`                 | Hide the footer with keybindings help         |
| `ui.colorFileNames`  | bool   | `true`                  | Color filenames by git status                 |
| `ui.showDiffStats`   | bool   | `true`                  | Show lines added / removed next to each file  |

### Pager vs external diff

These are two different rendering pipelines — pick one:

- **Pager** (`delta`, `bat`, `diff-so-fancy`, …): `git-different` runs
  `git diff -- <path>` and pipes the unified diff into the pager. The pager
  colorizes/renders it. This is the default mode.
- **External diff** (`difftastic`, …): `git-different` runs
  `git -c diff.external=<tool> diff -- <path>`. Git invokes the tool with file
  paths, and the tool computes and renders its own diff. Use this when the
  tool does structural diffing rather than textual.

If `externalDiff` is set (via config or `--external-diff`), it takes precedence
over `pager`.

For difftastic specifically, `git-different` exports `DFT_WIDTH`, `COLUMNS`,
and `DFT_COLOR=always` so colors and wrapping work sensibly in the embedded
viewer.

### Icon styles

| Style                 | Description                                                      |
| :-------------------- | :--------------------------------------------------------------- |
| `nerd-fonts-status`   | Boxed git status icons colored by change type                    |
| `nerd-fonts-simple`   | Generic file icon colored by change type                         |
| `nerd-fonts-filetype` | File-type specific icons (language icons) colored by change type |
| `nerd-fonts-full`     | Both status icon and file-type icon, all colored                 |
| `unicode`             | Unicode symbols                                                  |
| `ascii`               | Plain ASCII characters                                           |

## Keybindings

| Key                         | Description                              |
| :-------------------------- | :--------------------------------------- |
| <kbd>j</kbd> / <kbd>↓</kbd> | Next node                                |
| <kbd>k</kbd> / <kbd>↑</kbd> | Previous node                            |
| <kbd>n</kbd>                | Next file (skip directories)             |
| <kbd>p</kbd> / <kbd>N</kbd> | Previous file (skip directories)         |
| <kbd>h</kbd>                | Collapse node                            |
| <kbd>l</kbd>                | Expand node                              |
| <kbd>Enter</kbd>            | Toggle node                              |
| <kbd>Ctrl-d</kbd>           | Scroll the diff down                     |
| <kbd>Ctrl-u</kbd>           | Scroll the diff up                       |
| <kbd>e</kbd>                | Toggle the file tree                     |
| <kbd>t</kbd>                | Search / go-to file                      |
| <kbd>y</kbd>                | Copy file path                           |
| <kbd>o</kbd>                | Open file in `$EDITOR`                   |
| <kbd>c</kbd>                | Toggle commit-segmented view             |
| <kbd>i</kbd>                | Cycle icon style                         |
| <kbd>m</kbd>                | Show commit info                         |
| <kbd>Tab</kbd>              | Switch focus between panes               |
| <kbd>?</kbd> / <kbd>F1</kbd>| Toggle help                              |
| <kbd>q</kbd>                | Quit                                     |

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) and the [AI Usage Policy](AI_POLICY.md).

## Credits

`git-different` is a fork of [`diffnav`](https://github.com/dlvhdr/diffnav) by
[@dlvhdr](https://github.com/dlvhdr). The file tree, search, icon rendering,
and overall TUI layout come from upstream — this fork adds a git-aware CLI, a
configurable pager/external-diff system, and a commit-segmented view.

Built on:
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI framework
- [`go-gitdiff`](https://github.com/bluekeyes/go-gitdiff) — unified diff parser
