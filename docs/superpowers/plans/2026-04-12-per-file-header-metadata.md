# Per-File Header Metadata Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Update the header bar to show commit metadata for the most recent commit that edited the currently viewed file, rather than always showing range-level info.

**Architecture:** Add `FileLogPreamble` git helper for per-file commit lookups. Refactor `parseCommitMeta`/`commitSubject` to standalone functions. Add header-specific state fields (`headerPreamble`, `headerBranch`) separate from range-level fields. Fire async `tea.Cmd` on file navigation (cache miss) that returns a `fileCommitInfoMsg`; cache results so revisiting a file is instant.

**Tech Stack:** Go, bubbletea, git CLI

---

### Task 1: Add `FileLogPreamble` to git package

**Files:**
- Modify: `pkg/git/git.go:131-142` (add after `LogPreamble`)
- Test: `pkg/git/git_test.go`

- [ ] **Step 1: Write the failing test**

Add to `pkg/git/git_test.go` after the `LogPreamble` describe block (after line 215):

```go
var _ = Describe("FileLogPreamble", func() {
	It("returns preamble for the most recent commit touching the file in range", func() {
		repo := initRepo()
		writeFile(repo, "a.txt", "a\n")
		run(repo, "git", "add", ".")
		run(repo, "git", "commit", "-m", "add a")
		writeFile(repo, "b.txt", "b\n")
		run(repo, "git", "add", ".")
		run(repo, "git", "commit", "-m", "add b")

		out, err := git.FileLogPreamble(repo, []string{"HEAD~2..HEAD"}, "a.txt")
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(ContainSubstring("Author:"))
		Expect(out).To(ContainSubstring("add a"))
		Expect(out).NotTo(ContainSubstring("add b"))
	})

	It("returns empty string when no commit in range touches the file", func() {
		repo := initRepo()
		writeFile(repo, "a.txt", "a\n")
		run(repo, "git", "add", ".")
		run(repo, "git", "commit", "-m", "add a")
		writeFile(repo, "b.txt", "b\n")
		run(repo, "git", "add", ".")
		run(repo, "git", "commit", "-m", "add b")

		out, err := git.FileLogPreamble(repo, []string{"HEAD~1..HEAD"}, "a.txt")
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(BeEmpty())
	})

	It("converts single ref to range", func() {
		repo := initRepo()
		writeFile(repo, "a.txt", "a\n")
		run(repo, "git", "add", ".")
		run(repo, "git", "commit", "-m", "add a")

		out, err := git.FileLogPreamble(repo, []string{"HEAD~1"}, "a.txt")
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(ContainSubstring("add a"))
	})
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/git/... --ginkgo.focus="FileLogPreamble" -v`
Expected: compilation error — `git.FileLogPreamble` undefined.

- [ ] **Step 3: Write the implementation**

Add to `pkg/git/git.go` after the `LogPreamble` function (after line 142):

```go
// FileLogPreamble runs "git log -1 --format=fuller --decorate <range-args> -- <filepath>"
// and returns the preamble for the most recent commit in the range that touched
// the given file. Returns empty string if no commit in the range modified the file.
func FileLogPreamble(repoDir string, args []string, filepath string) (string, error) {
	logArgs := diffArgsToLogArgs(args)
	cmdArgs := slices.Concat(
		[]string{"log", "-1", "--format=fuller", "--decorate"},
		logArgs,
		[]string{"--", filepath},
	)
	out, err := runGit(repoDir, cmdArgs)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/git/... --ginkgo.focus="FileLogPreamble" -v`
Expected: all 3 specs pass.

- [ ] **Step 5: Run linters**

Run: `golangci-lint run --fix ./pkg/git/... && golangci-lint fmt ./pkg/git/...`

- [ ] **Step 6: Commit**

```bash
git add pkg/git/git.go pkg/git/git_test.go
git commit -m "Add FileLogPreamble for per-file commit lookups

Used by the TUI to find the most recent commit that edited a specific
file within a diff range, enabling per-file header metadata."
```

---

### Task 2: Refactor `parseCommitMeta` and `commitSubject` to standalone functions

**Files:**
- Modify: `pkg/ui/tui.go:662-736` (function signatures)
- Modify: `pkg/ui/tui.go:355` (call site in fileTreeMsg handler)
- Modify: `pkg/ui/tui.go:788` (call site in viewHeader)

This is a pure refactor — no behavior change.

- [ ] **Step 1: Run existing tests to establish baseline**

Run: `go test ./pkg/ui/... -v`
Expected: all tests pass.

- [ ] **Step 2: Refactor `parseCommitMeta`**

In `pkg/ui/tui.go`, change the function signature from a method to a standalone function.

Replace lines 662-712:

```go
func (m mainModel) parseCommitMeta() commitMeta {
	var meta commitMeta
	if m.preamble == "" {
		return meta
	}
	for line := range strings.SplitSeq(m.preamble, "\n") {
```

With:

```go
func parseCommitMeta(preamble string) commitMeta {
	var meta commitMeta
	if preamble == "" {
		return meta
	}
	for line := range strings.SplitSeq(preamble, "\n") {
```

- [ ] **Step 3: Refactor `commitSubject`**

In `pkg/ui/tui.go`, change the function signature from a method to a standalone function.

Replace lines 714-736:

```go
func (m mainModel) commitSubject() string {
	if m.preamble == "" {
		return ""
	}
	for line := range strings.SplitSeq(m.preamble, "\n") {
```

With:

```go
func commitSubject(preamble string) string {
	if preamble == "" {
		return ""
	}
	for line := range strings.SplitSeq(preamble, "\n") {
```

- [ ] **Step 4: Update call sites**

In `pkg/ui/tui.go`:

Line 355 — change:
```go
m.cachedMeta = m.parseCommitMeta()
```
To:
```go
m.cachedMeta = parseCommitMeta(m.preamble)
```

Line 788 — change:
```go
subject := m.commitSubject()
```
To:
```go
subject := commitSubject(m.preamble)
```

- [ ] **Step 5: Run tests to verify no regression**

Run: `go test ./pkg/ui/... -v`
Expected: all tests pass — behavior unchanged.

- [ ] **Step 6: Run linters**

Run: `golangci-lint run --fix ./pkg/ui/... && golangci-lint fmt ./pkg/ui/... && betteralign -apply ./pkg/ui/...`

- [ ] **Step 7: Commit**

```bash
git add pkg/ui/tui.go
git commit -m "Refactor parseCommitMeta and commitSubject to standalone functions

Accepts a preamble string parameter instead of reading from
m.preamble, so they can be called with any preamble source."
```

---

### Task 3: Add header state fields and wire into viewHeader

**Files:**
- Modify: `pkg/ui/tui.go:67-100` (add fields to mainModel)
- Modify: `pkg/ui/tui.go:346-356` (fileTreeMsg handler — initialize fields, clear cache)
- Modify: `pkg/ui/tui.go:738-801` (viewHeader — use new fields)

This is a pure refactor — the header shows the same info as before since the new fields are initialized from range-level values.

- [ ] **Step 1: Run existing tests to establish baseline**

Run: `go test ./pkg/ui/... -v`
Expected: all tests pass.

- [ ] **Step 2: Add new fields to mainModel**

In `pkg/ui/tui.go`, add three fields to the `mainModel` struct. Add after the `commitBranch` field (line 74):

```go
	headerPreamble  string
	headerBranch    string
```

Add after the `commitPreambles` field (line 68):

```go
	fileCommitCache   map[string]string
```

- [ ] **Step 3: Initialize header fields in fileTreeMsg handler**

In `pkg/ui/tui.go`, in the `fileTreeMsg` case (around line 353-355), after:

```go
	m.cachedMeta = parseCommitMeta(m.preamble)
```

Add:

```go
	m.headerPreamble = m.preamble
	m.headerBranch = m.commitBranch
	m.fileCommitCache = nil
	m.commitPreambles = nil
```

- [ ] **Step 4: Update viewHeader to use new fields**

In `pkg/ui/tui.go` in `viewHeader()`:

Change `m.commitBranch` (line 779) to `m.headerBranch`:

```go
		if m.headerBranch != "" {
			branchLabel := "[" + m.headerBranch + "]"
			if m.iconStyle != filenode.IconsASCII && m.iconStyle != filenode.IconsUnicode {
				branchLabel = string(md.SourceBranch) + " " + m.headerBranch
			}
```

Change `commitSubject(m.preamble)` (line 788) to use `m.headerPreamble`:

```go
		subject := commitSubject(m.headerPreamble)
```

- [ ] **Step 5: Run tests to verify no regression**

Run: `go test ./pkg/ui/... -v`
Expected: all tests pass — header displays same info as before.

- [ ] **Step 6: Run linters**

Run: `golangci-lint run --fix ./pkg/ui/... && golangci-lint fmt ./pkg/ui/... && betteralign -apply ./pkg/ui/...`

- [ ] **Step 7: Commit**

```bash
git add pkg/ui/tui.go
git commit -m "Add header state fields separate from range-level values

headerPreamble and headerBranch drive the header display and can be
updated per-file. fileCommitCache stores per-file preamble lookups.
No behavior change yet — fields initialized from range-level values."
```

---

### Task 4: Implement per-file header updates

**Files:**
- Modify: `pkg/ui/tui.go` — add message type, modify `setNodeDiff`, add Update handler, add fetch commands

- [ ] **Step 1: Add the `fileCommitInfoMsg` type**

In `pkg/ui/tui.go`, add near the other message types (after `diffRenderedMsg` around line 583):

```go
type fileCommitInfoMsg struct {
	key        string // file path (non-commit view) or commit hash (commit view)
	preamble   string
	branch     string
	commitView bool
}
```

- [ ] **Step 2: Add fetch command functions**

In `pkg/ui/tui.go`, add after `diffArgsForFile` (after line 1503):

```go
// fetchFileCommitInfo returns a command that looks up the most recent commit
// in the diff range that touched the given file, for header display.
func (m mainModel) fetchFileCommitInfo(fname string) tea.Cmd {
	return func() tea.Msg {
		preamble, err := gitpkg.FileLogPreamble(m.repoRoot, m.gitArgs, fname)
		if err != nil || preamble == "" {
			return nil
		}
		return fileCommitInfoMsg{
			key:      fname,
			preamble: preamble,
			branch:   resolveBranch(preamble),
		}
	}
}

// fetchCommitHeaderInfo returns a command that looks up commit metadata
// for header display in commit view.
func (m mainModel) fetchCommitHeaderInfo(hash string) tea.Cmd {
	return func() tea.Msg {
		preamble, err := gitpkg.CommitPreamble(m.repoRoot, hash)
		if err != nil {
			return nil
		}
		preamble = strings.TrimSpace(preamble)
		return fileCommitInfoMsg{
			key:        hash,
			preamble:   preamble,
			branch:     resolveBranch(preamble),
			commitView: true,
		}
	}
}
```

- [ ] **Step 3: Add a helper to update header state from a preamble**

In `pkg/ui/tui.go`, add near the header-related functions:

```go
// setHeaderFromPreamble updates the cached header state from a preamble string.
func (m *mainModel) setHeaderFromPreamble(preamble, branch string) {
	m.headerPreamble = preamble
	m.headerBranch = branch
	m.cachedMeta = parseCommitMeta(preamble)
}
```

- [ ] **Step 4: Modify `setNodeDiff` FileNode case**

In `pkg/ui/tui.go`, replace the `*filenode.FileNode` case in `setNodeDiff` (lines 1438-1446):

```go
	case *filenode.FileNode:
		fname := filenode.GetFileName(val.File)
		additions, deletions := filenode.DiffStats(val.File)
		m.diffViewer.SetFileHeader(fname, additions, deletions)

		args := m.diffArgsForFile(fname)

		// Update header to reflect the commit that last touched this file.
		var headerCmd tea.Cmd
		if m.commitView {
			if hash := m.fileTree.AncestorCommitHash(); hash != "" {
				if cached, ok := m.commitPreambles[hash]; ok {
					m.setHeaderFromPreamble(cached, resolveBranch(cached))
				} else {
					headerCmd = m.fetchCommitHeaderInfo(hash)
				}
			}
		} else if gitpkg.HasRefs(m.gitArgs) {
			if cached, ok := m.fileCommitCache[fname]; ok {
				m.setHeaderFromPreamble(cached, resolveBranch(cached))
			} else {
				headerCmd = m.fetchFileCommitInfo(fname)
			}
		}

		return m, tea.Batch(m.renderDiff(args), headerCmd)
```

Note the `HasRefs` guard: when diffing against the working tree (no refs, e.g. unstaged changes), there are no commits to look up — the range-level preamble is correct.

- [ ] **Step 5: Modify `setNodeDiff` CommitNode case**

In `pkg/ui/tui.go`, in the `*dirnode.CommitNode` case (lines 1448-1460), add header update logic after the existing code:

```go
	case *dirnode.CommitNode:
		files := m.fileTree.GetCurrNodeDesendantDiffs()
		var added, deleted int64
		for _, file := range files {
			na, nd := filenode.DiffStats(file)
			added += na
			deleted += nd
		}
		header := val.Hash[:7] + " " + val.Subject
		m.diffViewer.SetDirHeader(header, added, deleted)
		args := []string{val.Hash + "~1.." + val.Hash}

		// Update header to reflect this commit.
		var headerCmd tea.Cmd
		if cached, ok := m.commitPreambles[val.Hash]; ok {
			m.setHeaderFromPreamble(cached, resolveBranch(cached))
		} else {
			headerCmd = m.fetchCommitHeaderInfo(val.Hash)
		}

		return m, tea.Batch(m.renderDiff(args), headerCmd)
```

- [ ] **Step 6: Modify `setNodeDiff` DirNode/root case**

In `pkg/ui/tui.go`, in the `string, *dirnode.DirNode` case (lines 1462-1489), add header logic. In commit view, use ancestor commit info. In non-commit view, reset to range-level preamble:

```go
	case string, *dirnode.DirNode:
		files := m.fileTree.GetCurrNodeDesendantDiffs()
		fullPath := "/"
		if dir, ok := node.GivenValue().(*dirnode.DirNode); ok {
			fullPath = dir.FullPath
		}
		var added, deleted int64
		for _, file := range files {
			na, nd := filenode.DiffStats(file)
			added += na
			deleted += nd
		}
		m.diffViewer.SetDirHeader(fullPath, added, deleted)

		// In commit-segmented mode, scope to the ancestor commit
		var baseArgs []string
		var headerCmd tea.Cmd
		if m.commitView {
			if hash := m.fileTree.AncestorCommitHash(); hash != "" {
				baseArgs = []string{hash + "~1.." + hash}
				if cached, ok := m.commitPreambles[hash]; ok {
					m.setHeaderFromPreamble(cached, resolveBranch(cached))
				} else {
					headerCmd = m.fetchCommitHeaderInfo(hash)
				}
			}
		} else {
			// Reset header to range-level info for directories
			m.setHeaderFromPreamble(m.preamble, m.commitBranch)
		}
		if baseArgs == nil {
			baseArgs = append(baseArgs, m.gitArgs...)
		}
		if fullPath != "/" {
			baseArgs = append(baseArgs, "--", fullPath+"/")
		}
		return m, tea.Batch(m.renderDiff(baseArgs), headerCmd)
```

- [ ] **Step 7: Add `fileCommitInfoMsg` handler in Update**

In `pkg/ui/tui.go`, in the `Update` method's type switch, add a case after `diffRenderedMsg` (after line 369):

```go
	case fileCommitInfoMsg:
		// Cache the result.
		if msg.commitView {
			if m.commitPreambles == nil {
				m.commitPreambles = make(map[string]string)
			}
			m.commitPreambles[msg.key] = msg.preamble
		} else {
			if m.fileCommitCache == nil {
				m.fileCommitCache = make(map[string]string)
			}
			m.fileCommitCache[msg.key] = msg.preamble
		}

		// Only update header if the user is still viewing the relevant node.
		updateHeader := false
		if msg.commitView {
			if m.commitView {
				// Check if still under the same commit section.
				if hash := m.fileTree.AncestorCommitHash(); hash == msg.key {
					updateHeader = true
				}
				// Also match if a CommitNode itself is selected.
				if cn, ok := m.fileTree.GetCurrNode().GivenValue().(*dirnode.CommitNode); ok {
					if cn.Hash == msg.key {
						updateHeader = true
					}
				}
			}
		} else {
			if !m.commitView {
				if fn, ok := m.fileTree.GetCurrNode().GivenValue().(*filenode.FileNode); ok {
					if filenode.GetFileName(fn.File) == msg.key {
						updateHeader = true
					}
				}
			}
		}
		if updateHeader {
			m.setHeaderFromPreamble(msg.preamble, msg.branch)
		}
```

- [ ] **Step 8: Verify it compiles**

Run: `go build ./...`
Expected: clean build.

- [ ] **Step 9: Run all tests**

Run: `go test ./... -v`
Expected: all tests pass.

- [ ] **Step 10: Run linters**

Run: `golangci-lint run --fix ./... && golangci-lint fmt ./... && betteralign -apply ./...`

- [ ] **Step 11: Manual test**

Run the tool against a multi-commit range in this repo:

```bash
go run . HEAD~5..HEAD
```

Verify:
1. Navigate between files — header updates to show each file's most recent commit
2. Navigate to a directory — header resets to range-level info
3. Toggle commit view (`c`) — header reflects the commit section
4. Navigate back to a previously viewed file — header updates instantly (cached)
5. Press `m` — message overlay still shows correct info via `activePreamble()`

- [ ] **Step 12: Commit**

```bash
git add pkg/ui/tui.go
git commit -m "Update header to show per-file commit metadata

When navigating to a file, the header now reflects the most recent
commit that edited that file within the diff range. In commit view,
it reflects the commit section. Results are cached so revisiting a
file is instant. Directory nodes reset to range-level info."
```
