// Package git wraps git CLI interactions, returning structured data parsed from
// git's machine-readable output formats. All commands that support it use the
// -z flag for NUL-delimited output, avoiding fragile parsing that breaks on
// filenames with special characters.
package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
)

// FileStatus represents a file and its status from a git diff or diff-tree command.
type FileStatus struct {
	Status string
	Path   string
}

// Commit represents a single commit with its hash and subject line.
type Commit struct {
	Hash    string
	Subject string
}

// NameStatus runs "git diff --name-status -z <args>" and parses the
// NUL-delimited output into FileStatus entries. For renames (R) and copies (C),
// only the new (destination) path is used, and the score suffix is stripped from
// the status letter.
func NameStatus(repoDir string, args []string) ([]FileStatus, error) {
	cmdArgs := slices.Concat([]string{"diff", "--name-status", "-z"}, args)
	out, err := runGit(repoDir, cmdArgs)
	if err != nil {
		return nil, err
	}
	return parseNameStatus(out), nil
}

// DiffUnified runs "git diff <args>" and returns the raw unified diff output.
func DiffUnified(repoDir string, args []string) (string, error) {
	cmdArgs := slices.Concat([]string{"diff"}, args)
	out, err := runGit(repoDir, cmdArgs)
	if err != nil {
		return "", err
	}
	return out, nil
}

// ExternalDiff runs "git -c diff.external=<tool> diff --ext-diff <args>".
// External diff tools may exit non-zero even on success, so this function only
// returns an error when the command produces no output at all.
//
// The width parameter sets DFT_WIDTH and COLUMNS environment variables so that
// tools like difftastic can size their output to match the viewport.
func ExternalDiff(repoDir string, tool string, width int, args []string) (string, error) {
	cmdArgs := slices.Concat([]string{"-c", "diff.external=" + tool, "diff", "--ext-diff"}, args)
	cmd := exec.Command("git", cmdArgs...)
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("DFT_WIDTH=%d", width),
		fmt.Sprintf("COLUMNS=%d", width),
		"DFT_COLOR=always",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	_ = cmd.Run() // ignore exit code; external diff tools may exit non-zero

	out := stdout.String()
	if out == "" {
		return "", fmt.Errorf(
			"external diff tool %q produced no output (stderr: %s)",
			tool,
			stderr.String(),
		)
	}
	return out, nil
}

// LogCommits runs "git log -z --reverse --format=%H%x00%s <args>" and returns
// commits in chronological order (oldest first).
//
// The args are converted from git-diff semantics to git-log semantics: a single
// ref like "HEAD~5" becomes "HEAD~5..HEAD" because git-diff treats it as "diff
// ref against working tree" while git-log treats it as "show all ancestors of ref".
func LogCommits(repoDir string, args []string) ([]Commit, error) {
	logArgs := diffArgsToLogArgs(args)
	cmdArgs := slices.Concat([]string{"log", "-z", "--reverse", "--format=%H%x00%s"}, logArgs)
	out, err := runGit(repoDir, cmdArgs)
	if err != nil {
		return nil, err
	}
	return parseLogCommits(out), nil
}

// diffArgsToLogArgs converts git-diff positional args to git-log range args.
// A single ref without ".." or "..." is converted to "ref..HEAD" so that
// git-log returns the commits in the diff range rather than the ref's ancestors.
func diffArgsToLogArgs(args []string) []string {
	// Collect non-flag positional args (stop at "--")
	var refs []string
	for _, a := range args {
		if a == "--" {
			break
		}
		if strings.HasPrefix(a, "-") {
			continue
		}
		refs = append(refs, a)
	}

	// Single ref without range notation → convert to ref..HEAD
	if len(refs) == 1 && !strings.Contains(refs[0], "..") {
		result := make([]string, len(args))
		copy(result, args)
		for i, a := range result {
			if a == refs[0] {
				result[i] = refs[0] + "..HEAD"
				break
			}
		}
		return result
	}

	return args
}

// DiffTreeFiles runs "git diff-tree --no-commit-id -r --name-status -z <hash>"
// and returns the list of files changed in the given commit.
func DiffTreeFiles(repoDir string, hash string) ([]FileStatus, error) {
	out, err := runGit(
		repoDir,
		[]string{"diff-tree", "--no-commit-id", "-r", "--name-status", "-z", hash},
	)
	if err != nil {
		return nil, err
	}
	return parseNameStatus(out), nil
}

// PipeToPager runs "git diff <args>" and pipes the output through the given
// pager command. The pager string is split on spaces to support flags
// (e.g. "delta --paging=never").
func PipeToPager(repoDir string, pager string, args []string) (string, error) {
	diffArgs := slices.Concat([]string{"diff"}, args)
	diffCmd := exec.Command("git", diffArgs...)
	diffCmd.Dir = repoDir

	pagerParts := strings.Fields(pager)
	if len(pagerParts) == 0 {
		diffOut, err := runGit(repoDir, diffArgs)
		if err != nil {
			return "", err
		}
		return diffOut, nil
	}
	pagerCmd := exec.Command(pagerParts[0], pagerParts[1:]...)

	var pagerOut, pagerErr bytes.Buffer
	pagerCmd.Stdout = &pagerOut
	pagerCmd.Stderr = &pagerErr

	pipe, err := diffCmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("creating pipe: %w", err)
	}
	pagerCmd.Stdin = pipe

	if err := diffCmd.Start(); err != nil {
		return "", fmt.Errorf("starting git diff: %w", err)
	}
	if err := pagerCmd.Start(); err != nil {
		return "", fmt.Errorf("starting pager %q: %w", pager, err)
	}

	// Wait for pager first (it reads until EOF), then wait for git.
	pagerWaitErr := pagerCmd.Wait()
	_ = diffCmd.Wait()

	if pagerWaitErr != nil {
		return "", fmt.Errorf(
			"pager %q failed: %w (stderr: %s)",
			pager,
			pagerWaitErr,
			pagerErr.String(),
		)
	}

	return pagerOut.String(), nil
}

// RepoRoot runs "git rev-parse --show-toplevel" and returns the absolute path
// to the repository root.
func RepoRoot(dir string) (string, error) {
	out, err := runGit(dir, []string{"rev-parse", "--show-toplevel"})
	if err != nil {
		return "", err
	}
	return strings.TrimRight(out, "\n"), nil
}

// runGit executes a git command in the given directory and returns its combined
// stdout. Returns an error if the command exits non-zero.
func runGit(dir string, args []string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s failed: %w (stderr: %s)", args[0], err, stderr.String())
	}
	return stdout.String(), nil
}

// parseNameStatus parses NUL-delimited name-status output. The format is:
// STATUS\0PATH\0 for normal entries, and STATUS\0OLD_PATH\0NEW_PATH\0 for
// renames/copies (R/C with optional score suffix like R100).
func parseNameStatus(out string) []FileStatus {
	if out == "" {
		return nil
	}

	fields := strings.Split(out, "\x00")
	// Trim trailing empty field from final NUL delimiter
	if len(fields) > 0 && fields[len(fields)-1] == "" {
		fields = fields[:len(fields)-1]
	}

	var results []FileStatus
	for i := 0; i < len(fields); {
		status := fields[i]
		if status == "" {
			i++
			continue
		}

		// Renames/copies have a score suffix (e.g., R100, C050)
		isRenameOrCopy := strings.HasPrefix(status, "R") || strings.HasPrefix(status, "C")

		if isRenameOrCopy {
			// Format: R100\0old_path\0new_path
			if i+2 >= len(fields) {
				break
			}
			// Strip score suffix, keep just the letter
			cleanStatus := status[:1]
			newPath := fields[i+2]
			results = append(results, FileStatus{Status: cleanStatus, Path: newPath})
			i += 3
		} else {
			// Format: M\0path
			if i+1 >= len(fields) {
				break
			}
			results = append(results, FileStatus{Status: status, Path: fields[i+1]})
			i += 2
		}
	}

	return results
}

// parseLogCommits parses NUL-delimited log output where each record is
// "HASH\0SUBJECT" and records are separated by NUL.
func parseLogCommits(out string) []Commit {
	if out == "" {
		return nil
	}

	// Format from --format=%H%x00%s with -z: each commit is HASH\0SUBJECT,
	// and -z separates commits with \0. So the full output is:
	// HASH\0SUBJECT\0HASH\0SUBJECT\0...
	fields := strings.Split(out, "\x00")
	// Trim trailing empty field
	if len(fields) > 0 && fields[len(fields)-1] == "" {
		fields = fields[:len(fields)-1]
	}

	var commits []Commit
	for i := 0; i+1 < len(fields); i += 2 {
		commits = append(commits, Commit{
			Hash:    fields[i],
			Subject: fields[i+1],
		})
	}

	return commits
}
