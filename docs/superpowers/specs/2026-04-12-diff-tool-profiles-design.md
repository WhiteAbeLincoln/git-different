# Diff Tool Profiles

## Summary

Replace the top-level `pager` and `externalDiff` config fields with named
**profiles** — bundles of pager + external-diff settings that can be selected
from the config file or the command line. The current PATH-based auto-detection
logic becomes a set of built-in profiles, and the auto-detect priority order
changes to prefer delta over difftastic.

## Config file format

```yaml
ui:
  profile: builtin-delta    # optional — selects the active profile by name
  profiles:                  # optional — user-defined profiles
    my-fancy:
      pager: "delta --side-by-side --paging=never --true-color=always"
    structural:
      externalDiff: "difft --display=inline"
  # all other UI fields (icons, fileTreeWidth, etc.) are unchanged
```

- The `pager` and `externalDiff` fields are removed from the top level of
  `UIConfig`. They now exist only inside profile definitions.
- `UIConfig` retains `Pager` and `ExternalDiff` as runtime-only fields
  (populated by resolution, no longer deserialized from YAML).

## Profile struct

```go
type Profile struct {
    Pager        string `yaml:"pager"`
    ExternalDiff string `yaml:"externalDiff"`
}
```

Each profile sets at most one of these fields. Setting both is allowed but
`ExternalDiff` takes precedence at render time (existing behavior).

## Built-in profiles

Hardcoded in Go, not stored in the config file:

| Name                  | Field          | Command                                              |
|-----------------------|----------------|------------------------------------------------------|
| `builtin-delta`       | Pager          | `delta --paging=never --true-color=always`           |
| `builtin-difftastic`  | ExternalDiff   | `difft`                                              |
| `builtin-bat`         | Pager          | `bat --color=always --language=Diff --style=-header` |

## Resolution order

From highest to lowest priority:

1. **CLI `--pager` / `--external-diff` flags** — override everything.
2. **`--profile` CLI flag** — selects a profile by name.
3. **Config file `ui.profile` field** — selects a profile by name.
4. **Auto-detect from PATH** — delta > difftastic > bat > raw git diff.

When a profile is selected (steps 2 or 3), it is looked up first in the
user-defined `profiles` map, then in the built-in profiles. The first match
wins.

## CLI changes

New flag: `--profile <name>`

Accepts any user-defined profile name or built-in name (e.g. `builtin-delta`).

## Validation

- User-defined profile names starting with `builtin-` are rejected at config
  load time. `Load()` returns the default config (matching the current behavior
  for YAML parse errors).
- Selecting a nonexistent profile name (via `--profile` or `ui.profile`) is a
  fatal error at startup.

## Implementation changes

### `pkg/config/config.go`

- Add `Profile` struct (pager + externalDiff).
- Add `Profiles map[string]Profile` and `ProfileName string` to `UIConfig`
  (YAML-deserialized).
- Change `Pager` and `ExternalDiff` on `UIConfig` to use `yaml:"-"` so they
  are runtime-only.
- Add a `builtinProfiles` map variable holding the three built-in profiles.
- Rename `ResolveDiffTool` to `ResolveProfile`. New logic:
  1. If CLI flags set pager or externalDiff, populate those fields and return.
  2. If a profile name is set, look it up (user-defined then built-in),
     populate pager/externalDiff, and return.
  3. Otherwise, run PATH auto-detection: delta > difftastic > bat > empty.
- `Load()` validates that no user-defined profile name starts with `builtin-`.

### `cmd/root.go`

- Add `--profile` string flag.
- Pass the flag value into `cfg.UI.ProfileName` before calling
  `ResolveProfile`.
- Remove the comment referencing the old priority order; the function name
  and doc comment are now self-documenting.

### Tests (`pkg/config/config_test.go`)

- Rename/update existing `ResolveDiffTool` tests to `ResolveProfile`.
- Update auto-detect tests: delta now takes priority over difftastic.
- Add tests for profile selection (user-defined, built-in, CLI override).
- Add tests for validation (reject `builtin-` prefix in user profiles,
  error on nonexistent profile).

### No changes needed

- `pkg/git/git.go` — `PipeToPager` and `ExternalDiff` are untouched.
- `pkg/ui/tui.go` — reads `config.UI.Pager` / `config.UI.ExternalDiff` at
  render time, which are still populated by the resolution step.
