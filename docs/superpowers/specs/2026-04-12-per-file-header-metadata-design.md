# Per-File Header Metadata

## Problem

The header bar always shows metadata from the overall diff range preamble, regardless of which file is selected. When viewing a multi-commit range like `HEAD~3..HEAD`, every file shows the same commit info even though different files were last modified in different commits.

## Design

Update the header to reflect the most recent commit that edited the currently viewed file, within the diff range. In commit view, use the commit section containing the file.

### New git helper

Add `FileLogPreamble(repoDir, args, filepath)` to `pkg/git/git.go`. Runs `git log -1 --format=fuller --decorate <range-args> -- <filepath>` using the existing `diffArgsToLogArgs` conversion. Returns the preamble for the most recent commit in the range that touched the file.

### New message type and cache

In `tui.go`:
- `fileCommitInfoMsg` message type with fields: `preamble`, `branch`, `filePath`.
- `fileCommitCache map[string]string` on `mainModel` — maps file path to preamble text for non-commit view.
- Separate from existing `commitPreambles` (keyed by commit hash for commit view).
- Both caches cleared in the `fileTreeMsg` handler on file tree refresh (watch mode reload).

### Modified `setNodeDiff` for FileNode

When handling a `*filenode.FileNode`:
- **Non-commit view**: Check `fileCommitCache[fname]`. Cache hit → parse meta, update `cachedMeta`/`commitBranch` inline. Cache miss → fire `tea.Cmd` calling `FileLogPreamble`, returning `fileCommitInfoMsg`. Batched with existing `renderDiff` cmd.
- **Commit view**: Check `commitPreambles[ancestorHash]`. Cache hit → inline update. Cache miss → fire `tea.Cmd` calling `CommitPreamble`, returning `fileCommitInfoMsg`. Batched with existing `renderDiff` cmd.

### Modified `setNodeDiff` for DirNode/CommitNode/root

- **DirNode/root (non-commit view)**: Reset `cachedMeta`/`commitBranch` to range-level preamble values.
- **CommitNode (commit view)**: Update from that commit's preamble (same fetch/cache pattern).
- **DirNode under a commit (commit view)**: Use ancestor commit's info.

### New handler in Update

Handle `fileCommitInfoMsg`:
- Store preamble in appropriate cache (`fileCommitCache` or `commitPreambles`).
- Only update `cachedMeta`/`commitBranch` if current file still matches `msg.filePath` (guard against stale responses from navigation).
- Parse meta from the returned preamble.

### `parseCommitMeta` refactor

Refactor `parseCommitMeta()` → `parseCommitMeta(preamble string)` so it can accept any preamble text. Same for `commitSubject()` → `commitSubject(preamble string)`. `resolveBranch` already takes a string parameter.

## Behavior by node type and view mode

| Node type | Non-commit view | Commit view |
|-----------|----------------|-------------|
| FileNode | Most recent commit touching file in range (async fetch + cache) | Ancestor commit section's info |
| DirNode/root | Range-level preamble (current behavior) | Ancestor commit's info |
| CommitNode | N/A | That commit's info |
