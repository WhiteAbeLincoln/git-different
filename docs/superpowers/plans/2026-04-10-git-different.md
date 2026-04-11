# git-different Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Transform the forked diffnav into `git-different` — a git-aware TUI that invokes git directly, supports configurable pager/external-diff tools, and offers commit-segmented file tree views.

**Architecture:** Replace stdin input with direct git invocation. The TUI stores git diff args and shells out to git on-demand: `git diff --name-status` for the file tree, `git diff -- <file> | <pager>` or `git -c diff.external=<tool> diff -- <file>` for rendering. A new `pkg/git` package encapsulates all git commands. The file tree gains a commit-segmented mode that groups files under commit headers.

**Tech Stack:** Go, Bubble Tea v2, Cobra, go-gitdiff, Lipgloss v2

---

## File Structure

| File | Responsibility |
|------|---------------|
| `pkg/git/git.go` | **Create.** Wraps all git CLI invocations: diff, diff-tree, log, name-status. Single place for building and exec'ing git commands. |
| `pkg/git/git_test.go` | **Create.** Tests for the git package. |
| `pkg/config/config.go` | **Modify.** Add `Pager` and `ExternalDiff` fields, drop `SideBySide`, rename config dir from `diffnav` to `git-different`, rename env var. |
| `cmd/root.go` | **Modify.** Drop stdin reading, accept git diff args as positional args, add `--pager`/`--external-diff` flags, pass git args + pager config to TUI. |
| `pkg/ui/tui.go` | **Modify.** Replace `input string` with `gitArgs []string` + pager config. Change `fetchFileTree` to invoke git. Change `setNodeDiff` to invoke git per-file with pager. Add commit-segmented tree toggle. |
| `pkg/ui/panes/diffviewer/diffviewer.go` | **Modify.** Replace hardcoded delta with configurable pager/external-diff invocation. Drop `sideBySide` field. |
| `pkg/ui/keys.go` | **Modify.** Remove `ToggleDiffView` (`s`), add `ToggleCommitView` (`c`). |
| `pkg/ui/panes/filetree/filetree.go` | **Modify.** Add commit-segmented tree building mode. |
| `main.go` | **Modify.** Rename import path (if module renamed). |
| `cmd/logo-diff-part.txt`, `cmd/logo-nav-part.txt` | **Modify.** Update branding. |
| `pkg/watch/watch.go` | **Modify.** Change to run `git diff <args>` instead of arbitrary shell command. |

---

### Task 1: Create `pkg/git` Package — Git Command Wrappers

**Files:**
- Create: `pkg/git/git.go`
- Create: `pkg/git/git_test.go`

This package wraps all git CLI interactions. Every function returns structured data parsed from git output.

- [ ] **Step 1: Write failing test for `NameStatus`**

`pkg/git/git_test.go`:
```go
package git_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/dlvhdr/diffnav/pkg/git"
)

func TestGit(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Git Suite")
}

var _ = Describe("NameStatus", func() {
	var repoDir string

	BeforeEach(func() {
		var err error
		repoDir, err = os.MkdirTemp("", "git-different-test-*")
		Expect(err).NotTo(HaveOccurred())

		run := func(args ...string) {
			cmd := exec.Command("git", args...)
			cmd.Dir = repoDir
			cmd.Env = append(os.Environ(),
				"GIT_AUTHOR_NAME=Test",
				"GIT_AUTHOR_EMAIL=test@test.com",
				"GIT_COMMITTER_NAME=Test",
				"GIT_COMMITTER_EMAIL=test@test.com",
			)
			out, err := cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), string(out))
		}

		run("init")
		run("commit", "--allow-empty", "-m", "initial")

		Expect(os.WriteFile(filepath.Join(repoDir, "added.txt"), []byte("new"), 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(repoDir, "modified.txt"), []byte("v1"), 0o644)).To(Succeed())
		run("add", ".")
		run("commit", "-m", "first")

		Expect(os.WriteFile(filepath.Join(repoDir, "modified.txt"), []byte("v2"), 0o644)).To(Succeed())
		run("add", ".")
		run("commit", "-m", "second")
	})

	AfterEach(func() {
		os.RemoveAll(repoDir)
	})

	It("returns file statuses for a diff range", func() {
		entries, err := git.NameStatus(repoDir, []string{"HEAD~2..HEAD"})
		Expect(err).NotTo(HaveOccurred())
		Expect(entries).To(ConsistOf(
			git.FileStatus{Status: "A", Path: "added.txt"},
			git.FileStatus{Status: "A", Path: "modified.txt"},
		))
	})

	It("returns file statuses for a single commit", func() {
		entries, err := git.NameStatus(repoDir, []string{"HEAD~1..HEAD"})
		Expect(err).NotTo(HaveOccurred())
		Expect(entries).To(ConsistOf(
			git.FileStatus{Status: "M", Path: "modified.txt"},
		))
	})
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/abe/Projects/git-different && go test ./pkg/git/... -v`
Expected: Compilation failure — `pkg/git` doesn't exist yet.

- [ ] **Step 3: Implement `pkg/git/git.go` with `NameStatus`**

`pkg/git/git.go`:
```go
package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// FileStatus represents a file's change status from git diff --name-status.
type FileStatus struct {
	Status string // "A", "M", "D", "R", etc.
	Path   string
}

// NameStatus runs `git diff --name-status <args>` and returns parsed file statuses.
func NameStatus(repoDir string, args []string) ([]FileStatus, error) {
	cmdArgs := append([]string{"diff", "--name-status"}, args...)
	cmd := exec.Command("git", cmdArgs...)
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff --name-status: %w", err)
	}
	return parseNameStatus(string(out))
}

func parseNameStatus(output string) ([]FileStatus, error) {
	var entries []FileStatus
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) < 2 {
			continue
		}
		entries = append(entries, FileStatus{
			Status: parts[0],
			Path:   parts[1],
		})
	}
	return entries, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/abe/Projects/git-different && go test ./pkg/git/... -v`
Expected: PASS

- [ ] **Step 5: Write failing test for `DiffUnified`**

Append to `pkg/git/git_test.go`:
```go
var _ = Describe("DiffUnified", func() {
	var repoDir string

	BeforeEach(func() {
		var err error
		repoDir, err = os.MkdirTemp("", "git-different-test-*")
		Expect(err).NotTo(HaveOccurred())

		run := func(args ...string) {
			cmd := exec.Command("git", args...)
			cmd.Dir = repoDir
			cmd.Env = append(os.Environ(),
				"GIT_AUTHOR_NAME=Test",
				"GIT_AUTHOR_EMAIL=test@test.com",
				"GIT_COMMITTER_NAME=Test",
				"GIT_COMMITTER_EMAIL=test@test.com",
			)
			out, err := cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), string(out))
		}

		run("init")
		Expect(os.WriteFile(filepath.Join(repoDir, "hello.txt"), []byte("hello"), 0o644)).To(Succeed())
		run("add", ".")
		run("commit", "-m", "initial")
		Expect(os.WriteFile(filepath.Join(repoDir, "hello.txt"), []byte("world"), 0o644)).To(Succeed())
		run("add", ".")
		run("commit", "-m", "change")
	})

	AfterEach(func() {
		os.RemoveAll(repoDir)
	})

	It("returns unified diff output for a file", func() {
		out, err := git.DiffUnified(repoDir, []string{"HEAD~1..HEAD", "--", "hello.txt"})
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(ContainSubstring("--- a/hello.txt"))
		Expect(out).To(ContainSubstring("+++ b/hello.txt"))
	})
})
```

- [ ] **Step 6: Implement `DiffUnified`**

Add to `pkg/git/git.go`:
```go
// DiffUnified runs `git diff <args>` and returns the raw unified diff output.
func DiffUnified(repoDir string, args []string) (string, error) {
	cmdArgs := append([]string{"diff"}, args...)
	cmd := exec.Command("git", cmdArgs...)
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}
	return string(out), nil
}
```

- [ ] **Step 7: Run tests to verify both pass**

Run: `cd /Users/abe/Projects/git-different && go test ./pkg/git/... -v`
Expected: PASS

- [ ] **Step 8: Write failing test for `ExternalDiff`**

Append to `pkg/git/git_test.go`:
```go
var _ = Describe("ExternalDiff", func() {
	var repoDir string

	BeforeEach(func() {
		var err error
		repoDir, err = os.MkdirTemp("", "git-different-test-*")
		Expect(err).NotTo(HaveOccurred())

		run := func(args ...string) {
			cmd := exec.Command("git", args...)
			cmd.Dir = repoDir
			cmd.Env = append(os.Environ(),
				"GIT_AUTHOR_NAME=Test",
				"GIT_AUTHOR_EMAIL=test@test.com",
				"GIT_COMMITTER_NAME=Test",
				"GIT_COMMITTER_EMAIL=test@test.com",
			)
			out, err := cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), string(out))
		}

		run("init")
		Expect(os.WriteFile(filepath.Join(repoDir, "hello.txt"), []byte("hello"), 0o644)).To(Succeed())
		run("add", ".")
		run("commit", "-m", "initial")
		Expect(os.WriteFile(filepath.Join(repoDir, "hello.txt"), []byte("world"), 0o644)).To(Succeed())
		run("add", ".")
		run("commit", "-m", "change")
	})

	AfterEach(func() {
		os.RemoveAll(repoDir)
	})

	It("invokes git with diff.external set", func() {
		// Use cat as a trivial external diff tool — it just prints whatever git passes
		out, err := git.ExternalDiff(repoDir, "cat", []string{"HEAD~1..HEAD", "--", "hello.txt"})
		Expect(err).NotTo(HaveOccurred())
		// cat will receive the file contents git passes to the external tool
		Expect(out).NotTo(BeEmpty())
	})
})
```

- [ ] **Step 9: Implement `ExternalDiff`**

Add to `pkg/git/git.go`:
```go
// ExternalDiff runs `git -c diff.external=<tool> diff --ext-diff <args>` and returns output.
func ExternalDiff(repoDir string, tool string, args []string) (string, error) {
	cmdArgs := append([]string{
		"-c", "diff.external=" + tool,
		"diff", "--ext-diff",
	}, args...)
	cmd := exec.Command("git", cmdArgs...)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		// External diff tools may exit non-zero — only fail if there's no output
		if len(out) == 0 {
			return "", fmt.Errorf("git diff --ext-diff: %w", err)
		}
	}
	return string(out), nil
}
```

- [ ] **Step 10: Run tests**

Run: `cd /Users/abe/Projects/git-different && go test ./pkg/git/... -v`
Expected: PASS

- [ ] **Step 11: Write failing test for `LogCommits`**

Append to `pkg/git/git_test.go`:
```go
var _ = Describe("LogCommits", func() {
	var repoDir string

	BeforeEach(func() {
		var err error
		repoDir, err = os.MkdirTemp("", "git-different-test-*")
		Expect(err).NotTo(HaveOccurred())

		run := func(args ...string) {
			cmd := exec.Command("git", args...)
			cmd.Dir = repoDir
			cmd.Env = append(os.Environ(),
				"GIT_AUTHOR_NAME=Test",
				"GIT_AUTHOR_EMAIL=test@test.com",
				"GIT_COMMITTER_NAME=Test",
				"GIT_COMMITTER_EMAIL=test@test.com",
			)
			out, err := cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), string(out))
		}

		run("init")
		run("commit", "--allow-empty", "-m", "initial")
		Expect(os.WriteFile(filepath.Join(repoDir, "a.txt"), []byte("a"), 0o644)).To(Succeed())
		run("add", ".")
		run("commit", "-m", "add a")
		Expect(os.WriteFile(filepath.Join(repoDir, "b.txt"), []byte("b"), 0o644)).To(Succeed())
		run("add", ".")
		run("commit", "-m", "add b")
	})

	AfterEach(func() {
		os.RemoveAll(repoDir)
	})

	It("returns commits in chronological order", func() {
		commits, err := git.LogCommits(repoDir, []string{"HEAD~2..HEAD"})
		Expect(err).NotTo(HaveOccurred())
		Expect(commits).To(HaveLen(2))
		Expect(commits[0].Subject).To(Equal("add a"))
		Expect(commits[1].Subject).To(Equal("add b"))
	})
})
```

- [ ] **Step 12: Implement `LogCommits`**

Add to `pkg/git/git.go`:
```go
// Commit represents a single git commit.
type Commit struct {
	Hash    string
	Subject string
}

// LogCommits runs `git log --reverse --format='%H %s' <args>` and returns commits
// in chronological order (oldest first).
func LogCommits(repoDir string, args []string) ([]Commit, error) {
	cmdArgs := append([]string{"log", "--reverse", "--format=%H %s"}, args...)
	cmd := exec.Command("git", cmdArgs...)
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}
	return parseLogCommits(string(out))
}

func parseLogCommits(output string) ([]Commit, error) {
	var commits []Commit
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		// Format: <40-char-hash> <subject>
		if len(line) < 42 {
			continue
		}
		commits = append(commits, Commit{
			Hash:    line[:40],
			Subject: line[41:],
		})
	}
	return commits, nil
}
```

- [ ] **Step 13: Write failing test for `DiffTreeFiles`**

Append to `pkg/git/git_test.go`:
```go
var _ = Describe("DiffTreeFiles", func() {
	var repoDir string

	BeforeEach(func() {
		var err error
		repoDir, err = os.MkdirTemp("", "git-different-test-*")
		Expect(err).NotTo(HaveOccurred())

		run := func(args ...string) {
			cmd := exec.Command("git", args...)
			cmd.Dir = repoDir
			cmd.Env = append(os.Environ(),
				"GIT_AUTHOR_NAME=Test",
				"GIT_AUTHOR_EMAIL=test@test.com",
				"GIT_COMMITTER_NAME=Test",
				"GIT_COMMITTER_EMAIL=test@test.com",
			)
			out, err := cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), string(out))
		}

		run("init")
		run("commit", "--allow-empty", "-m", "initial")
		Expect(os.WriteFile(filepath.Join(repoDir, "a.txt"), []byte("a"), 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(repoDir, "b.txt"), []byte("b"), 0o644)).To(Succeed())
		run("add", ".")
		run("commit", "-m", "add files")
	})

	AfterEach(func() {
		os.RemoveAll(repoDir)
	})

	It("returns changed files for a commit", func() {
		entries, err := git.DiffTreeFiles(repoDir, "HEAD")
		Expect(err).NotTo(HaveOccurred())
		Expect(entries).To(ConsistOf(
			git.FileStatus{Status: "A", Path: "a.txt"},
			git.FileStatus{Status: "A", Path: "b.txt"},
		))
	})
})
```

- [ ] **Step 14: Implement `DiffTreeFiles`**

Add to `pkg/git/git.go`:
```go
// DiffTreeFiles runs `git diff-tree --no-commit-id -r <hash>` and returns file statuses.
func DiffTreeFiles(repoDir string, hash string) ([]FileStatus, error) {
	cmd := exec.Command("git", "diff-tree", "--no-commit-id", "-r", hash)
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff-tree: %w", err)
	}
	return parseDiffTree(string(out))
}

func parseDiffTree(output string) ([]FileStatus, error) {
	var entries []FileStatus
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		// Format: :old-mode new-mode old-hash new-hash status\tpath
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) < 2 {
			continue
		}
		meta := parts[0]
		path := parts[1]
		fields := strings.Fields(meta)
		if len(fields) < 5 {
			continue
		}
		entries = append(entries, FileStatus{
			Status: fields[4],
			Path:   path,
		})
	}
	return entries, nil
}
```

- [ ] **Step 15: Write failing test for `PipeToPager`**

Append to `pkg/git/git_test.go`:
```go
var _ = Describe("PipeToPager", func() {
	var repoDir string

	BeforeEach(func() {
		var err error
		repoDir, err = os.MkdirTemp("", "git-different-test-*")
		Expect(err).NotTo(HaveOccurred())

		run := func(args ...string) {
			cmd := exec.Command("git", args...)
			cmd.Dir = repoDir
			cmd.Env = append(os.Environ(),
				"GIT_AUTHOR_NAME=Test",
				"GIT_AUTHOR_EMAIL=test@test.com",
				"GIT_COMMITTER_NAME=Test",
				"GIT_COMMITTER_EMAIL=test@test.com",
			)
			out, err := cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), string(out))
		}

		run("init")
		Expect(os.WriteFile(filepath.Join(repoDir, "hello.txt"), []byte("hello"), 0o644)).To(Succeed())
		run("add", ".")
		run("commit", "-m", "initial")
		Expect(os.WriteFile(filepath.Join(repoDir, "hello.txt"), []byte("world"), 0o644)).To(Succeed())
		run("add", ".")
		run("commit", "-m", "change")
	})

	AfterEach(func() {
		os.RemoveAll(repoDir)
	})

	It("pipes git diff output through a pager command", func() {
		// Use cat as a trivial pager — output should pass through unchanged
		out, err := git.PipeToPager(repoDir, "cat", []string{"HEAD~1..HEAD", "--", "hello.txt"})
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(ContainSubstring("--- a/hello.txt"))
	})
})
```

- [ ] **Step 16: Implement `PipeToPager`**

Add to `pkg/git/git.go`:
```go
// PipeToPager runs `git diff <args>` and pipes the output through the pager command.
// The pager string is split on spaces to support flags (e.g. "delta --paging=never --side-by-side").
func PipeToPager(repoDir string, pager string, args []string) (string, error) {
	diffArgs := append([]string{"diff"}, args...)
	gitCmd := exec.Command("git", diffArgs...)
	gitCmd.Dir = repoDir

	diffOut, err := gitCmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}

	pagerParts := strings.Fields(pager)
	if len(pagerParts) == 0 {
		return string(diffOut), nil
	}

	pagerCmd := exec.Command(pagerParts[0], pagerParts[1:]...)
	pagerCmd.Stdin = strings.NewReader(string(diffOut))
	pagerCmd.Dir = repoDir
	pagerOut, err := pagerCmd.Output()
	if err != nil {
		return "", fmt.Errorf("pager %q: %w", pager, err)
	}
	return string(pagerOut), nil
}
```

- [ ] **Step 17: Run all tests**

Run: `cd /Users/abe/Projects/git-different && go test ./pkg/git/... -v`
Expected: All PASS

- [ ] **Step 18: Write failing test for `RepoRoot`**

Append to `pkg/git/git_test.go`:
```go
var _ = Describe("RepoRoot", func() {
	It("returns the repo root directory", func() {
		repoDir, err := os.MkdirTemp("", "git-different-test-*")
		Expect(err).NotTo(HaveOccurred())
		defer os.RemoveAll(repoDir)

		cmd := exec.Command("git", "init")
		cmd.Dir = repoDir
		Expect(cmd.Run()).To(Succeed())

		subDir := filepath.Join(repoDir, "sub", "dir")
		Expect(os.MkdirAll(subDir, 0o755)).To(Succeed())

		root, err := git.RepoRoot(subDir)
		Expect(err).NotTo(HaveOccurred())
		// Resolve symlinks for macOS /private/var/folders vs /var/folders
		expectedRoot, _ := filepath.EvalSymlinks(repoDir)
		Expect(root).To(Equal(expectedRoot))
	})
})
```

- [ ] **Step 19: Implement `RepoRoot`**

Add to `pkg/git/git.go`:
```go
// RepoRoot returns the top-level directory of the git repo containing dir.
func RepoRoot(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --show-toplevel: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
```

- [ ] **Step 20: Run all tests and lint**

Run: `cd /Users/abe/Projects/git-different && go test ./pkg/git/... -v && golangci-lint run ./pkg/git/... && betteralign -apply ./pkg/git/...`
Expected: All PASS, no lint issues

- [ ] **Step 21: Add Ginkgo/Gomega dependency**

Run: `cd /Users/abe/Projects/git-different && go get github.com/onsi/ginkgo/v2 github.com/onsi/gomega`

- [ ] **Step 22: Commit**

```bash
git add pkg/git/
git commit -m "Add pkg/git package wrapping git CLI commands

Encapsulates all git invocations behind a typed Go API: NameStatus,
DiffUnified, ExternalDiff, LogCommits, DiffTreeFiles, PipeToPager,
and RepoRoot. This will replace the stdin-based input model."
```

---

### Task 2: Update Config — Pager, ExternalDiff, Rename

**Files:**
- Modify: `pkg/config/config.go`

- [ ] **Step 1: Update `UIConfig` struct**

In `pkg/config/config.go`, replace the `UIConfig` struct and related code:

Remove the `SideBySide` field from `UIConfig`. Add `Pager` and `ExternalDiff` fields:

```go
type UIConfig struct {
	HideHeader      bool   `yaml:"hideHeader"`
	HideFooter      bool   `yaml:"hideFooter"`
	ShowFileTree    bool   `yaml:"showFileTree"`
	FileTreeWidth   int    `yaml:"fileTreeWidth"`
	SearchTreeWidth int    `yaml:"searchTreeWidth"`
	Icons           string `yaml:"icons"`
	ColorFileNames  bool   `yaml:"colorFileNames"`
	ShowDiffStats   bool   `yaml:"showDiffStats"`
	Pager           string `yaml:"pager"`
	ExternalDiff    string `yaml:"externalDiff"`
}
```

Update `DefaultConfig()`:
```go
func DefaultConfig() Config {
	return Config{
		UI: UIConfig{
			HideHeader:      false,
			HideFooter:      false,
			ShowFileTree:    true,
			FileTreeWidth:   30,
			SearchTreeWidth: 50,
			Icons:           "nerd-fonts-status",
			ColorFileNames:  true,
			ShowDiffStats:   true,
			Pager:           "delta --paging=never",
			ExternalDiff:    "",
		},
	}
}
```

- [ ] **Step 2: Rename config directory from `diffnav` to `git-different`**

In `getConfigFilePath()`, replace all occurrences of `"diffnav"` with `"git-different"` and `"DIFFNAV_CONFIG_DIR"` with `"GIT_DIFFERENT_CONFIG_DIR"`:

```go
func getConfigFilePath() string {
	var configDirs []string

	if dir := os.Getenv("GIT_DIFFERENT_CONFIG_DIR"); dir != "" {
		if s, err := os.Stat(dir); err == nil && s.IsDir() {
			return filepath.Join(dir, "config.yml")
		}
	}

	if runtime.GOOS == "darwin" {
		if xdgConfigDir := os.Getenv("XDG_CONFIG_HOME"); xdgConfigDir != "" {
			configDirs = append(configDirs, xdgConfigDir)
		}
		if home := os.Getenv("HOME"); home != "" {
			configDirs = append(configDirs, filepath.Join(home, ".config"))
		}
	}

	if configDir, err := os.UserConfigDir(); err == nil {
		configDirs = append(configDirs, configDir)
	}

	for _, dir := range configDirs {
		configPath := filepath.Join(dir, "git-different", "config.yml")
		if _, err := os.Stat(configPath); err == nil {
			return configPath
		}
	}

	if len(configDirs) > 0 {
		return filepath.Join(configDirs[0], "git-different", "config.yml")
	}
	return ""
}
```

- [ ] **Step 3: Verify compilation**

Run: `cd /Users/abe/Projects/git-different && go build ./...`
Expected: Compilation errors in files that reference `SideBySide` — this is expected and will be fixed in subsequent tasks.

- [ ] **Step 4: Commit**

```bash
git add pkg/config/config.go
git commit -m "Update config: add Pager/ExternalDiff, drop SideBySide, rename to git-different

Config directory changes from diffnav/ to git-different/.
Environment variable changes from DIFFNAV_CONFIG_DIR to GIT_DIFFERENT_CONFIG_DIR.
Pager defaults to 'delta --paging=never'. ExternalDiff is empty (disabled) by default."
```

---

### Task 3: Update Keybindings — Remove `s`, Add `c`

**Files:**
- Modify: `pkg/ui/keys.go`

- [ ] **Step 1: Replace `ToggleDiffView` with `ToggleCommitView`**

In `pkg/ui/keys.go`, in the `KeyMap` struct, replace `ToggleDiffView` with `ToggleCommitView`:

```go
type KeyMap struct {
	ExpandNode       key.Binding
	CollapseNode     key.Binding
	ToggleNode       key.Binding
	Up               key.Binding
	Down             key.Binding
	NextFile         key.Binding
	PrevFile         key.Binding
	CtrlD            key.Binding
	CtrlU            key.Binding
	ToggleFileTree   key.Binding
	Search           key.Binding
	Quit             key.Binding
	Copy             key.Binding
	SwitchPanel      key.Binding
	OpenInEditor     key.Binding
	ToggleCommitView key.Binding
	ToggleIconStyle  key.Binding
	ToggleHelp       key.Binding
	ToggleMessage    key.Binding
}
```

In the `keys` var, replace `ToggleDiffView`:

```go
	ToggleCommitView: key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "toggle commit view"),
	),
```

In `KeyGroups()`, replace `keys.ToggleDiffView` with `keys.ToggleCommitView`.

- [ ] **Step 2: Verify compilation of keys.go**

Run: `cd /Users/abe/Projects/git-different && go vet ./pkg/ui/...`
Expected: Errors in `tui.go` referencing `keys.ToggleDiffView` — expected, will be fixed in Task 5.

- [ ] **Step 3: Commit**

```bash
git add pkg/ui/keys.go
git commit -m "Replace s (toggle side-by-side) with c (toggle commit view)

Side-by-side is now controlled via pager flags in config.
The c keybinding will toggle between unified and commit-segmented file tree."
```

---

### Task 4: Update Diff Viewer — Configurable Pager

**Files:**
- Modify: `pkg/ui/panes/diffviewer/diffviewer.go`

This task replaces the hardcoded delta invocation with configurable pager/external-diff. The diff viewer no longer runs the pager itself — it receives pre-rendered diff text from the main model (which invokes `pkg/git`).

- [ ] **Step 1: Simplify the Model — remove delta, sideBySide, caching**

Rewrite `pkg/ui/panes/diffviewer/diffviewer.go`. The diff viewer becomes a simple viewport that receives rendered diff text. The main model handles invoking git + pager.

```go
package diffviewer

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dlvhdr/diffnav/pkg/icons"
	"github.com/dlvhdr/diffnav/pkg/ui/common"
	"github.com/dlvhdr/diffnav/pkg/utils"
)

const dirHeaderHeight = 3

type Model struct {
	common.Common
	vp       viewport.Model
	header   headerData
	preamble string
}

type headerData struct {
	path      string
	isDir     bool
	additions int64
	deletions int64
}

func New() Model {
	return Model{
		vp: viewport.Model{},
	}
}

func (m *Model) SetPreamble(preamble string) {
	m.preamble = preamble
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "down", "j", "n", "up", "k", "N", "p":
			// Consumed by main model for tree navigation
		default:
			vp, cmd := m.vp.Update(msg)
			m.vp = vp
			return m, cmd
		}
	}
	return m, nil
}

const scrollbarWidth = 3

func (m Model) View() string {
	vpView := m.vp.View()
	scrollbar := common.RenderScrollbar(m.vp.Height(), m.vp.TotalLineCount(), m.vp.YOffset())
	if scrollbar != "" {
		vpView = lipgloss.JoinHorizontal(lipgloss.Top, vpView, " ", scrollbar)
	}
	return lipgloss.JoinVertical(lipgloss.Left, m.headerView(), vpView)
}

// SetContent sets the rendered diff text directly into the viewport.
func (m *Model) SetContent(content string) {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if lipgloss.Width(line) > m.vp.Width() && m.vp.Width() > 0 {
			lines[i] = ansi.Truncate(line, m.vp.Width(), "")
		}
	}
	m.vp.SetContent(strings.Join(lines, "\n"))
}

// SetFileHeader configures the header for a single file diff.
func (m *Model) SetFileHeader(path string, additions, deletions int64) {
	m.header = headerData{path: path, isDir: false, additions: additions, deletions: deletions}
}

// SetDirHeader configures the header for a directory diff.
func (m *Model) SetDirHeader(path string, additions, deletions int64) {
	m.header = headerData{path: path, isDir: true, additions: additions, deletions: deletions}
}

func (m *Model) SetSize(width, height int) {
	m.Width = width
	m.Height = height
	m.vp.SetWidth(m.contentWidth())
	m.vp.SetHeight(m.Height - dirHeaderHeight)
}

func (m Model) contentWidth() int {
	return m.Width - scrollbarWidth
}

func (m *Model) GoToTop() {
	m.vp.GotoTop()
}

func (m *Model) ScrollUp(lines int) {
	m.vp.ScrollUp(lines)
}

func (m *Model) ScrollDown(lines int) {
	m.vp.ScrollDown(lines)
}

func (m Model) headerView() string {
	if m.header.isDir {
		return m.dirHeaderView()
	}
	if m.header.path == "" {
		return ""
	}

	base := lipgloss.NewStyle()
	fileIcon := icons.GetIcon(m.header.path, false)
	prefix := base.Render(fileIcon) + base.Render(" ")
	name := utils.TruncateString(m.header.path, m.Width-lipgloss.Width(prefix))
	top := prefix + base.Bold(true).Render(name)
	bottom := viewDiffStats(m.header.additions, m.header.deletions, base)

	return base.
		Width(m.Width).
		Height(dirHeaderHeight - 1).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(lipgloss.Color("8")).
		Render(lipgloss.JoinVertical(lipgloss.Left, top, bottom))
}

func (m Model) dirHeaderView() string {
	base := lipgloss.NewStyle().Foreground(lipgloss.Blue)
	prefix := base.Render(" ")
	name := utils.TruncateString(m.header.path, m.Width-lipgloss.Width(prefix))
	top := prefix + base.Bold(true).Render(name)
	bottom := viewDiffStats(m.header.additions, m.header.deletions, base)

	return base.
		Width(m.Width).
		Height(dirHeaderHeight - 1).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(lipgloss.Color("8")).
		Render(lipgloss.JoinVertical(lipgloss.Left, top, bottom))
}

func viewDiffStats(added, deleted int64, base lipgloss.Style) string {
	var parts []string
	if added > 0 {
		parts = append(parts, base.Foreground(lipgloss.Green).Render(fmt.Sprintf("+%d", added)))
	}
	if deleted > 0 {
		parts = append(parts, base.Foreground(lipgloss.Red).Render(fmt.Sprintf("-%d", deleted)))
	}
	return strings.Join(parts, base.Render(" "))
}
```

Note: Add `"fmt"` to the imports.

- [ ] **Step 2: Verify compilation**

Run: `cd /Users/abe/Projects/git-different && go build ./pkg/ui/panes/diffviewer/...`
Expected: PASS (this package compiles independently)

- [ ] **Step 3: Commit**

```bash
git add pkg/ui/panes/diffviewer/diffviewer.go
git commit -m "Simplify diff viewer to a dumb viewport

Remove hardcoded delta invocation, caching, and sideBySide toggle.
The diff viewer now receives pre-rendered text via SetContent().
The main model is responsible for invoking git + pager/external-diff."
```

---

### Task 5: Update `cmd/root.go` — Git-Aware CLI

**Files:**
- Modify: `cmd/root.go`

- [ ] **Step 1: Rewrite the CLI to accept git diff args**

Replace the contents of `cmd/root.go`. Key changes:
- `Use` becomes `"git-different"`, description updated
- Drop `--side-by-side` / `--unified` flags
- Add `--pager` / `--external-diff` flags
- `Args` are passed through to git diff
- Drop stdin reading — invoke git to check there's a valid repo
- `ui.New` now receives `gitArgs []string` and `repoRoot string` instead of `input string`

```go
package cmd

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	tea "charm.land/bubbletea/v2"
	"charm.land/fang/v2"
	"charm.land/lipgloss/v2"
	"charm.land/log/v2"
	"github.com/charmbracelet/colorprofile"
	zone "github.com/lrstanley/bubblezone/v2"

	"github.com/dlvhdr/diffnav/pkg/config"
	gitpkg "github.com/dlvhdr/diffnav/pkg/git"
	"github.com/dlvhdr/diffnav/pkg/ui"
	"github.com/dlvhdr/diffnav/pkg/version"
)

//go:embed logo-diff-part.txt
var asciiArtDiffPart string

//go:embed logo-nav-part.txt
var asciiArtNavPart string

var logo = lipgloss.JoinHorizontal(lipgloss.Top,
	lipgloss.NewStyle().Foreground(lipgloss.Green).Render(asciiArtDiffPart),
	lipgloss.NewStyle().Foreground(lipgloss.Red).Render(asciiArtNavPart))

var rootCmd = &cobra.Command{
	Use:   "git-different [flags] [git-diff-args...]",
	Short: "GIT-DIFFERENT — a git diff TUI with file tree navigation and configurable pager.",
	Long: "\n" + logo + lipgloss.NewStyle().Foreground(lipgloss.White).Render(
		"\na git diff TUI with file tree navigation\nand configurable pager"),
	Example: `# diff between branches
git different main...feature-branch

# diff last 3 commits
git different HEAD~3

# staged changes
git different --staged

# working tree changes (default)
git different

# with watch mode
git different --watch --watch-interval 5s HEAD
	`,
	DisableFlagParsing: false,
}

func Execute() {
	themeFunc := fang.WithColorSchemeFunc(func(
		ld lipgloss.LightDarkFunc,
	) fang.ColorScheme {
		def := fang.DefaultColorScheme(ld)
		def.DimmedArgument = ld(lipgloss.Black, lipgloss.White)
		def.Codeblock = ld(lipgloss.Color("#F1EFEF"), lipgloss.Color("#141417"))
		def.Title = lipgloss.Red
		def.Command = lipgloss.Green
		def.Program = lipgloss.Green
		return def
	})

	if err := fang.Execute(
		context.Background(),
		rootCmd,
		themeFunc,
		fang.WithVersion(version.Version),
		fang.WithNotifySignal(os.Interrupt)); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().String("pager", "", "Pager command (overrides config)")
	rootCmd.Flags().String("external-diff", "", "External diff tool (overrides config)")

	rootCmd.Flags().
		BoolP("watch", "w", false, "Watch mode: periodically re-run git diff and refresh")
	rootCmd.Flags().Duration("watch-interval", 2*time.Second, "Interval between watch refreshes")

	rootCmd.SetVersionTemplate("\n" + logo + "\n" + `{{printf "version %s\n" .Version}}`)

	rootCmd.Run = func(cmd *cobra.Command, args []string) {
		pagerFlag, err := cmd.Flags().GetString("pager")
		if err != nil {
			log.Fatal("Cannot parse the pager flag", err)
		}
		externalDiffFlag, err := cmd.Flags().GetString("external-diff")
		if err != nil {
			log.Fatal("Cannot parse the external-diff flag", err)
		}

		watchFlag, err := cmd.Flags().GetBool("watch")
		if err != nil {
			log.Fatal("Cannot parse the watch flag", err)
		}
		watchInterval, err := cmd.Flags().GetDuration("watch-interval")
		if err != nil {
			log.Fatal("Cannot parse the watch-interval flag", err)
		}

		zone.NewGlobal()

		if os.Getenv("DEBUG") == "true" {
			var fileErr error
			logFile, fileErr := os.OpenFile("debug.log",
				os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o666)
			if fileErr != nil {
				fmt.Println("Error opening debug.log:", fileErr)
				os.Exit(1)
			}
			defer func() {
				if err := logFile.Close(); err != nil {
					log.Fatal("failed closing log file", "err", err)
				}
			}()

			if fileErr == nil {
				log.SetOutput(logFile)
				log.SetTimeFormat(time.Kitchen)
				log.SetReportCaller(true)
				log.SetLevel(log.DebugLevel)

				log.SetOutput(logFile)
				log.SetColorProfile(colorprofile.TrueColor)
				wd, err := os.Getwd()
				if err != nil {
					fmt.Println("Error getting current working dir", err)
					os.Exit(1)
				}
				log.Debug("Starting git-different", "logFile",
					wd+string(os.PathSeparator)+logFile.Name())
			}
		} else {
			log.SetOutput(os.Stderr)
			log.SetLevel(log.FatalLevel)
		}

		// Resolve repo root from CWD
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Println("Error getting working directory:", err)
			os.Exit(1)
		}
		repoRoot, err := gitpkg.RepoRoot(cwd)
		if err != nil {
			fmt.Println("Not a git repository (or any parent)")
			os.Exit(1)
		}

		// Check if there are any diffs
		entries, err := gitpkg.NameStatus(repoRoot, args)
		if err != nil {
			fmt.Println("Error running git diff:", err)
			os.Exit(1)
		}
		if len(entries) == 0 && !watchFlag {
			fmt.Println("No diff, exiting")
			os.Exit(0)
		}

		cfg := config.Load()

		// Override config with CLI flags
		if pagerFlag != "" {
			cfg.UI.Pager = pagerFlag
		}
		if externalDiffFlag != "" {
			cfg.UI.ExternalDiff = externalDiffFlag
		}

		cfg.Watch = config.WatchConfig{
			Enabled:  watchFlag,
			Interval: watchInterval,
		}

		ttyIn, _, err := tea.OpenTTY()
		if err != nil {
			log.Fatal(err)
		}
		p := tea.NewProgram(ui.New(repoRoot, args, cfg), tea.WithInput(ttyIn))

		if _, err := p.Run(); err != nil {
			log.Fatal(err)
		}
	}
}
```

- [ ] **Step 2: Verify it compiles (expect errors in ui package)**

Run: `cd /Users/abe/Projects/git-different && go build ./cmd/...`
Expected: Errors because `ui.New` signature has changed — this is expected and will be fixed in Task 6.

- [ ] **Step 3: Commit**

```bash
git add cmd/root.go
git commit -m "Rewrite CLI to invoke git directly instead of reading stdin

Accept git diff args as positional args. Add --pager and --external-diff
flags. Drop --side-by-side/--unified. Resolve repo root from CWD and
verify diff has changes before launching TUI."
```

---

### Task 6: Update `tui.go` — Wire Everything Together

**Files:**
- Modify: `pkg/ui/tui.go`

This is the largest task. The main model changes from holding a pre-parsed `input string` to holding `gitArgs []string` + `repoRoot string`, and delegates all diff rendering to `pkg/git`.

- [ ] **Step 1: Update `mainModel` struct**

In `pkg/ui/tui.go`, replace the struct and `New` function. Key changes:
- Replace `input string` with `repoRoot string` and `gitArgs []string`
- Remove `sideBySide` field
- Add `commitView bool` and `commits []git.Commit` for commit-segmented mode
- `diffViewer.New()` no longer takes `sideBySide`

Replace the `mainModel` struct (line 67-98):
```go
type mainModel struct {
	repoRoot          string
	gitArgs           []string
	files             []*gitdiff.File
	fileTree          filetree.Model
	diffViewer        diffviewer.Model
	width             int
	height            int
	isShowingFileTree bool
	activePanel       Panel
	search            textinput.Model
	resultsVp         viewport.Model
	resultsCursor     int
	searching         bool
	filtered          []string
	config            config.Config
	draggingSidebar   bool
	iconStyle         string
	help              help.Model
	helpOpen          bool
	messageOpen       bool
	messageVp         viewport.Model
	preamble          string
	commitBranch      string
	cachedMeta        commitMeta
	watchEnabled      bool
	watchInterval     time.Duration
	pendingCursorPath string
	watchInFlight     bool
	commitView        bool
	commits           []gitpkg.Commit
}
```

Add the import: `gitpkg "github.com/dlvhdr/diffnav/pkg/git"`

Replace `New` (line 100-134):
```go
func New(repoRoot string, gitArgs []string, cfg config.Config) mainModel {
	m := mainModel{
		repoRoot:          repoRoot,
		gitArgs:           gitArgs,
		isShowingFileTree: cfg.UI.ShowFileTree,
		activePanel:       FileTreePanel,
		config:            cfg,
		iconStyle:         cfg.UI.Icons,
		watchEnabled:      cfg.Watch.Enabled,
		watchInterval:     cfg.Watch.Interval,
	}
	m.fileTree = filetree.New(cfg)
	m.fileTree.SetSize(cfg.UI.FileTreeWidth, 0)
	m.diffViewer = diffviewer.New()
	m.help = help.New()
	m.help.SetKeys(KeyGroups())

	m.search = textinput.New()
	m.search.ShowSuggestions = true
	m.search.KeyMap.AcceptSuggestion = key.NewBinding(key.WithKeys("tab"))
	m.search.Prompt = " "
	m.search.Placeholder = "Filter files 󰬛 "
	m.search.SetStyles(textinput.Styles{
		Focused: textinput.StyleState{
			Placeholder: lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
			Prompt:      lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		},
	})
	m.search.SetWidth(cfg.UI.FileTreeWidth - 2)

	m.resultsVp = viewport.Model{}

	return m
}
```

- [ ] **Step 2: Remove `fetchRepoRoot` — repo root is now passed in**

Delete the `repoRootMsg` type and `fetchRepoRoot` method (lines 136-144). Remove `m.fetchRepoRoot` from `Init()`.

In the `repoRootMsg` case in `Update()` (line 327-328), remove it.

- [ ] **Step 3: Update `Init` and `fetchFileTree` to use `pkg/git`**

Replace `fetchFileTree` (line 587-597):
```go
func (m mainModel) fetchFileTree() tea.Msg {
	diffOutput, err := gitpkg.DiffUnified(m.repoRoot, m.gitArgs)
	if err != nil {
		return common.ErrMsg{Err: err}
	}
	files, preamble, err := gitdiff.Parse(strings.NewReader(diffOutput + "\n"))
	if err != nil {
		return common.ErrMsg{Err: err}
	}
	sortFiles(files)

	branch := resolveBranch(preamble)

	// Fetch commits for segmented view
	commits, _ := gitpkg.LogCommits(m.repoRoot, m.gitArgs)

	return fileTreeMsg{files: files, preamble: preamble, branch: branch, commits: commits}
}
```

Update `fileTreeMsg` to include commits:
```go
type fileTreeMsg struct {
	files    []*gitdiff.File
	preamble string
	branch   string
	commits  []gitpkg.Commit
}
```

Update the `fileTreeMsg` handler in `Update()` to store commits:
```go
	case fileTreeMsg:
		m.files = msg.files
		m.commits = msg.commits
		if len(m.files) == 0 && !m.watchEnabled {
			return m, tea.Quit
		}
		m.fileTree = m.fileTree.SetFiles(m.files)
		m.preamble = strings.TrimSpace(msg.preamble)
		m.commitBranch = msg.branch
		m.cachedMeta = m.parseCommitMeta()
		m.diffViewer.SetPreamble(m.preamble)
		m, cmd = m.setNodeDiff(m.fileTree.GetCurrNode())
		cmds = append(cmds, cmd)
		if m.pendingCursorPath != "" {
			m.fileTree.SetCursorByPath(m.pendingCursorPath)
			node := m.fileTree.GetCurrNode()
			m, cmd = m.setNodeDiff(node)
			cmds = append(cmds, cmd)
			m.pendingCursorPath = ""
		}
```

- [ ] **Step 4: Update `setNodeDiff` to invoke git + pager**

Replace `setNodeDiff` (line 1312-1328). It now shells out to git to render the diff:

```go
func (m mainModel) setNodeDiff(node *tree.Node) (mainModel, tea.Cmd) {
	switch val := node.GivenValue().(type) {
	case *filenode.FileNode:
		fname := filenode.GetFileName(val.File)
		additions, deletions := filenode.DiffStats(val.File)
		m.diffViewer.SetFileHeader(fname, additions, deletions)
		args := append(m.gitArgs, "--", fname)
		return m, m.renderDiff(args)
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

		// Build pathspec for all files in this directory
		var pathArgs []string
		pathArgs = append(pathArgs, m.gitArgs...)
		if fullPath != "/" {
			pathArgs = append(pathArgs, "--", fullPath+"/")
		}
		return m, m.renderDiff(pathArgs)
	}
	return m, nil
}

func (m mainModel) renderDiff(args []string) tea.Cmd {
	return func() tea.Msg {
		var output string
		var err error

		if m.config.UI.ExternalDiff != "" {
			output, err = gitpkg.ExternalDiff(m.repoRoot, m.config.UI.ExternalDiff, args)
		} else {
			output, err = gitpkg.PipeToPager(m.repoRoot, m.config.UI.Pager, args)
		}
		if err != nil {
			return common.ErrMsg{Err: err}
		}
		return diffRenderedMsg(output)
	}
}

type diffRenderedMsg string
```

Add a handler for `diffRenderedMsg` in `Update()`:
```go
	case diffRenderedMsg:
		m.diffViewer.SetContent(string(msg))
```

- [ ] **Step 5: Remove `ToggleDiffView` references, add `ToggleCommitView`**

In the `Update()` key handling, remove the `ToggleDiffView` case:
```go
		// DELETE THIS:
		case key.Matches(msg, keys.ToggleDiffView):
			m.sideBySide = !m.sideBySide
			cmd = m.diffViewer.SetSideBySide(m.sideBySide)
			cmds = append(cmds, cmd)
```

Add the `ToggleCommitView` handler:
```go
		case key.Matches(msg, keys.ToggleCommitView):
			if len(m.commits) > 1 {
				m.commitView = !m.commitView
				// TODO: rebuild file tree in commit-segmented mode (Task 7)
			}
```

- [ ] **Step 6: Update watch mode to use git diff**

Replace `fetchWatchDiff`:
```go
func (m mainModel) fetchWatchDiff() tea.Msg {
	output, err := gitpkg.DiffUnified(m.repoRoot, m.gitArgs)
	return watchResultMsg{output: output, err: err}
}
```

Update `watchResultMsg` handler — replace `m.input = msg.output` with comparing against the new diff output. Store last diff output on the model for change detection:

Add a `lastDiffOutput string` field to `mainModel`, and in `fetchFileTree`, store the output. In `watchResultMsg` handler, compare against `m.lastDiffOutput`.

- [ ] **Step 7: Remove `resolveBranch` exec.Command — use `pkg/git` or keep inline**

The `resolveBranch` function (line 599-639) can stay as-is for now — it's parsing preamble text, not a core change. Keep it.

- [ ] **Step 8: Remove unused imports**

Remove the `"bufio"`, `"io"` imports and the `watch` package import from `tui.go`. Remove `"os/exec"` if no longer needed (check `resolveBranch` and `openInEditor`).

- [ ] **Step 9: Verify compilation**

Run: `cd /Users/abe/Projects/git-different && go build ./...`
Expected: PASS

- [ ] **Step 10: Run existing tests**

Run: `cd /Users/abe/Projects/git-different && go test ./... -v`
Expected: PASS (existing tests in diffviewer_test.go and filetree_test.go may need updating if they reference removed APIs)

- [ ] **Step 11: Lint and format**

Run: `cd /Users/abe/Projects/git-different && golangci-lint run --fix ./... && golangci-lint fmt ./... && betteralign -apply ./...`

- [ ] **Step 12: Commit**

```bash
git add pkg/ui/tui.go
git commit -m "Wire TUI to invoke git directly with configurable pager

Replace stdin input model with git invocation via pkg/git.
The main model stores gitArgs and repoRoot, shells out to git
on file selection, and pipes output through the configured pager
or external diff tool. Watch mode now re-runs git diff directly."
```

---

### Task 7: Commit-Segmented File Tree

**Files:**
- Modify: `pkg/ui/panes/filetree/filetree.go`
- Modify: `pkg/ui/tui.go`

- [ ] **Step 1: Add commit node type**

Create a `CommitNode` in the existing `pkg/dirnode/dir_node.go` (it's a tree structure node, fits here):

Add to `pkg/dirnode/dir_node.go`:
```go
// CommitNode represents a commit header in the segmented file tree view.
type CommitNode struct {
	Hash    string
	Subject string
}

func (c *CommitNode) String() string {
	return c.Hash[:7] + " " + c.Subject
}
```

- [ ] **Step 2: Add `SetCommitFiles` to filetree**

Add to `pkg/ui/panes/filetree/filetree.go`:
```go
// CommitFiles groups files by commit for the segmented view.
type CommitFiles struct {
	Hash    string
	Subject string
	Files   []*gitdiff.File
}

// SetCommitFiles builds the tree in commit-segmented mode.
func (m Model) SetCommitFiles(commitFiles []CommitFiles) Model {
	m.files = nil
	for _, cf := range commitFiles {
		m.files = append(m.files, cf.Files...)
	}
	m.rebuildCommitTree(commitFiles)
	m.t.SetWidth(m.t.Width())
	m.updateStyles()
	return m
}

func (m *Model) rebuildCommitTree(commitFiles []CommitFiles) {
	root := tree.Root(&dirnode.DirNode{FullPath: "/", Name: constants.RootName})

	for _, cf := range commitFiles {
		commitNode := tree.Root(&dirnode.CommitNode{
			Hash:    cf.Hash,
			Subject: cf.Subject,
		})
		for _, file := range cf.Files {
			commitNode.Child(&filenode.FileNode{
				File: file,
				Cfg:  m.cfg,
			})
		}
		root.Child(commitNode)
	}

	root, _ = truncateTree(root, 0, 0, 0, m.cfg, m.t.Width())
	m.t.SetNodes(root)
	m.t.SetWidth(m.t.Width())
	m.updateStyles()
}
```

- [ ] **Step 3: Update `truncateTree` to handle `CommitNode`**

In `truncateTree`, add a case for `*dirnode.CommitNode`:
```go
		case *dirnode.CommitNode:
			subTree, subNum := truncateTree(child, depth+1, numNodes, 0, cfg, width)
			numChildren += subNum
			numNodes += subNum + 1
			newT.Child(subTree)
```

Also update the root check at the top of `truncateTree` — it currently assumes the root is always a `DirNode`. Add handling for `CommitNode`:
```go
func truncateTree(
	t *tree.Node,
	depth int,
	numNodes int,
	numChildren int,
	cfg config.Config,
	width int,
) (*tree.Node, int) {
	var newT *tree.Node
	switch val := t.GivenValue().(type) {
	case *dirnode.DirNode:
		newT = tree.Root(
			&dirnode.DirNode{
				Name:     utils.TruncateString(val.Name, width-depth-2),
				FullPath: val.FullPath,
			},
		)
	case *dirnode.CommitNode:
		subject := utils.TruncateString(val.Subject, width-depth-10)
		newT = tree.Root(
			&dirnode.CommitNode{
				Hash:    val.Hash,
				Subject: subject,
			},
		)
	default:
		return t, 0
	}
	numNodes++

	// ... rest of the function stays the same from the for loop onward
```

- [ ] **Step 4: Update filetree styles for `CommitNode`**

In `updateStyles`, update the `SelectedNodeStyleFunc` to handle `CommitNode`:
```go
		SelectedNodeStyleFunc: func(children tree.Nodes, i int) lipgloss.Style {
			base := base.Bold(true).Background(dimmed)
			child := children.At(i)
			switch child.GivenValue().(type) {
			case *filenode.FileNode:
				return base
			case string, *dirnode.DirNode, *dirnode.CommitNode:
				return base.Foreground(lipgloss.BrightBlue)
			}
			return base
		},
```

- [ ] **Step 5: Wire up commit toggle in `tui.go`**

Update the `ToggleCommitView` handler in `tui.go` `Update()`:
```go
		case key.Matches(msg, keys.ToggleCommitView):
			if len(m.commits) > 1 {
				m.commitView = !m.commitView
				if m.commitView {
					m, cmd = m.rebuildCommitSegmentedTree()
					cmds = append(cmds, cmd)
				} else {
					m.fileTree = m.fileTree.SetFiles(m.files)
					node := m.fileTree.GetCurrNode()
					m, cmd = m.setNodeDiff(node)
					cmds = append(cmds, cmd)
				}
			}
```

Add the `rebuildCommitSegmentedTree` method:
```go
func (m mainModel) rebuildCommitSegmentedTree() (mainModel, tea.Cmd) {
	var commitFiles []filetree.CommitFiles
	for _, commit := range m.commits {
		files, err := gitpkg.DiffTreeFiles(m.repoRoot, commit.Hash)
		if err != nil {
			continue
		}
		// Match DiffTreeFiles results to our parsed gitdiff.File objects
		var matched []*gitdiff.File
		for _, fs := range files {
			for _, f := range m.files {
				if filenode.GetFileName(f) == fs.Path {
					matched = append(matched, f)
					break
				}
			}
		}
		commitFiles = append(commitFiles, filetree.CommitFiles{
			Hash:    commit.Hash,
			Subject: commit.Subject,
			Files:   matched,
		})
	}
	m.fileTree = m.fileTree.SetCommitFiles(commitFiles)
	node := m.fileTree.GetCurrNode()
	var cmd tea.Cmd
	m, cmd = m.setNodeDiff(node)
	return m, cmd
}
```

- [ ] **Step 6: Handle `CommitNode` in `setNodeDiff`**

Update `setNodeDiff` to handle the case where a commit node is selected (show all files in that commit):
```go
func (m mainModel) setNodeDiff(node *tree.Node) (mainModel, tea.Cmd) {
	switch val := node.GivenValue().(type) {
	case *filenode.FileNode:
		fname := filenode.GetFileName(val.File)
		additions, deletions := filenode.DiffStats(val.File)
		m.diffViewer.SetFileHeader(fname, additions, deletions)

		// In commit-segmented mode, diff within the specific commit
		args := m.diffArgsForNode(node, fname)
		return m, m.renderDiff(args)

	case *dirnode.CommitNode:
		// Show all changes in this commit
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
		return m, m.renderDiff(args)

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
		var pathArgs []string
		pathArgs = append(pathArgs, m.gitArgs...)
		if fullPath != "/" {
			pathArgs = append(pathArgs, "--", fullPath+"/")
		}
		return m, m.renderDiff(pathArgs)
	}
	return m, nil
}

// diffArgsForNode returns the git diff args for rendering a file,
// taking commit-segmented mode into account.
func (m mainModel) diffArgsForNode(node *tree.Node, fname string) []string {
	if m.commitView {
		// Walk up to find the parent CommitNode
		// For now, search commits for which one contains this file
		for _, commit := range m.commits {
			files, err := gitpkg.DiffTreeFiles(m.repoRoot, commit.Hash)
			if err != nil {
				continue
			}
			for _, fs := range files {
				if fs.Path == fname {
					return []string{commit.Hash + "~1.." + commit.Hash, "--", fname}
				}
			}
		}
	}
	return append(m.gitArgs, "--", fname)
}
```

- [ ] **Step 7: Verify compilation and test**

Run: `cd /Users/abe/Projects/git-different && go build ./... && go test ./... -v`
Expected: PASS

- [ ] **Step 8: Lint**

Run: `cd /Users/abe/Projects/git-different && golangci-lint run --fix ./... && golangci-lint fmt ./... && betteralign -apply ./...`

- [ ] **Step 9: Commit**

```bash
git add pkg/dirnode/ pkg/ui/panes/filetree/ pkg/ui/tui.go
git commit -m "Add commit-segmented file tree view

Toggle with c key. Groups files under commit headers showing
short hash + subject. Each file's diff is scoped to its specific
commit. Only available when the diff range contains multiple commits."
```

---

### Task 8: Update Watch Mode

**Files:**
- Modify: `pkg/watch/watch.go`

- [ ] **Step 1: Simplify watch.go**

The watch package ran an arbitrary shell command. Now we just need a function that runs `git diff` — but `tui.go` already calls `gitpkg.DiffUnified` directly in `fetchWatchDiff`. The watch package is no longer needed for command execution.

Check if `watch.go` is still imported anywhere. If `fetchWatchDiff` in `tui.go` now uses `gitpkg.DiffUnified` directly, the `watch` package is only needed if other callers exist.

If unused, remove the import from `tui.go` and `cmd/root.go`. The `watch` package and its test can remain for now (it's still a valid utility).

Remove the `watch` import from `cmd/root.go` and `pkg/ui/tui.go` if no longer referenced.

- [ ] **Step 2: Verify compilation**

Run: `cd /Users/abe/Projects/git-different && go build ./...`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add pkg/ui/tui.go cmd/root.go
git commit -m "Remove watch package dependency from main code paths

Watch mode now invokes git diff directly via pkg/git."
```

---

### Task 9: Rename Binary and Branding

**Files:**
- Modify: `main.go`
- Modify: `cmd/logo-diff-part.txt`, `cmd/logo-nav-part.txt` (optional branding update)
- Modify: `flake.nix` (if build name referenced)

- [ ] **Step 1: Verify binary name**

The binary name comes from the Go module build. Since `cmd/root.go` already has `Use: "git-different"`, cobra will show the right name. The actual binary name depends on `go build -o`. Verify with:

Run: `cd /Users/abe/Projects/git-different && go build -o git-different . && ./git-different --help`
Expected: Help output shows `git-different` branding.

- [ ] **Step 2: Update flake.nix if needed**

If the flake.nix builds the binary, update the output name. Currently it only sets up a dev shell, so no change needed.

- [ ] **Step 3: Manual smoke test**

Run the tool against the current repo:
```bash
cd /Users/abe/Projects/git-different && go build -o git-different . && ./git-different HEAD~3
```

Verify:
- File tree appears in sidebar with A/M/D status
- Selecting a file shows its diff in the main panel
- `j`/`k` navigation works
- `c` toggle works (if multiple commits)
- `q` quits cleanly

- [ ] **Step 4: Commit**

```bash
git add .
git commit -m "Rename binary to git-different

Enables git subcommand auto-discovery as 'git different'."
```

---

### Task 10: Clean Up Existing Tests

**Files:**
- Modify: `pkg/ui/panes/diffviewer/diffviewer_test.go`
- Modify: `pkg/ui/tui_test.go`
- Modify: `pkg/ui/panes/filetree/filetree_test.go`

- [ ] **Step 1: Read existing tests**

Read all three test files to understand what they test and what needs updating.

- [ ] **Step 2: Update tests for new API**

Update any tests that reference:
- `diffviewer.New(sideBySide bool)` → `diffviewer.New()`
- `SetFilePatch` / `SetDirPatch` → `SetContent` / `SetFileHeader` / `SetDirHeader`
- `ui.New(input string, cfg)` → `ui.New(repoRoot, gitArgs, cfg)`
- `config.UI.SideBySide` → removed

- [ ] **Step 3: Run all tests**

Run: `cd /Users/abe/Projects/git-different && go test ./... -v`
Expected: All PASS

- [ ] **Step 4: Lint**

Run: `cd /Users/abe/Projects/git-different && golangci-lint run --fix ./... && golangci-lint fmt ./... && betteralign -apply ./...`

- [ ] **Step 5: Commit**

```bash
git add .
git commit -m "Update existing tests for new git-different API

Adapt test assertions to the simplified diffviewer, new tui.New
signature, and removed SideBySide config."
```
