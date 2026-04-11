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

// initRepo creates a temp git repo with an initial commit and returns its path.
func initRepo() string {
	dir := GinkgoT().TempDir()

	run(dir, "git", "init")
	run(dir, "git", "config", "user.email", "test@test.com")
	run(dir, "git", "config", "user.name", "Test")
	writeFile(dir, "README.md", "# hello\n")
	run(dir, "git", "add", ".")
	run(dir, "git", "commit", "-m", "initial commit")

	return dir
}

func run(dir string, name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = GinkgoWriter
	cmd.Stderr = GinkgoWriter
	ExpectWithOffset(1, cmd.Run()).To(Succeed())
}

func writeFile(dir, name, content string) {
	full := filepath.Join(dir, name)
	ExpectWithOffset(1, os.MkdirAll(filepath.Dir(full), 0o755)).To(Succeed())
	ExpectWithOffset(1, os.WriteFile(full, []byte(content), 0o644)).To(Succeed())
}

var _ = Describe("NameStatus", func() {
	It("returns modified files between HEAD and working tree", func() {
		repo := initRepo()
		writeFile(repo, "README.md", "# updated\n")

		files, err := git.NameStatus(repo, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(files).To(ConsistOf(
			git.FileStatus{Status: "M", Path: "README.md"},
		))
	})

	It("returns added files in staged changes", func() {
		repo := initRepo()
		writeFile(repo, "new.txt", "new file\n")
		run(repo, "git", "add", "new.txt")

		files, err := git.NameStatus(repo, []string{"--staged"})
		Expect(err).NotTo(HaveOccurred())
		Expect(files).To(ContainElement(
			git.FileStatus{Status: "A", Path: "new.txt"},
		))
	})

	It("handles renames by using the new path", func() {
		repo := initRepo()
		run(repo, "git", "mv", "README.md", "DOCS.md")
		run(repo, "git", "commit", "-m", "rename")

		files, err := git.NameStatus(repo, []string{"HEAD~1..HEAD"})
		Expect(err).NotTo(HaveOccurred())
		Expect(files).To(ConsistOf(
			git.FileStatus{Status: "R", Path: "DOCS.md"},
		))
	})

	It("handles filenames with spaces", func() {
		repo := initRepo()
		writeFile(repo, "file with spaces.txt", "content\n")
		run(repo, "git", "add", ".")
		run(repo, "git", "commit", "-m", "add spaced file")
		writeFile(repo, "file with spaces.txt", "updated\n")

		files, err := git.NameStatus(repo, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(files).To(ConsistOf(
			git.FileStatus{Status: "M", Path: "file with spaces.txt"},
		))
	})

	It("returns empty slice when there are no changes", func() {
		repo := initRepo()

		files, err := git.NameStatus(repo, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(files).To(BeEmpty())
	})
})

var _ = Describe("DiffUnified", func() {
	It("returns unified diff output", func() {
		repo := initRepo()
		writeFile(repo, "README.md", "# updated\n")

		out, err := git.DiffUnified(repo, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(ContainSubstring("--- a/README.md"))
		Expect(out).To(ContainSubstring("+++ b/README.md"))
		Expect(out).To(ContainSubstring("-# hello"))
		Expect(out).To(ContainSubstring("+# updated"))
	})

	It("returns empty string when there are no changes", func() {
		repo := initRepo()

		out, err := git.DiffUnified(repo, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(BeEmpty())
	})
})

var _ = Describe("ExternalDiff", func() {
	It("captures output from an external diff tool", func() {
		repo := initRepo()
		writeFile(repo, "README.md", "# updated\n")

		// Use a simple shell script as the external diff tool.
		// Git passes: path old-file old-hex old-mode new-file new-hex new-mode
		tool := filepath.Join(repo, "my-diff")
		writeFile(repo, "my-diff", "#!/bin/sh\necho \"EXTERNAL_DIFF: $1\"\n")
		run(repo, "chmod", "+x", tool)

		out, err := git.ExternalDiff(repo, tool, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(ContainSubstring("EXTERNAL_DIFF:"))
	})

	It("returns error when tool produces no output", func() {
		repo := initRepo()
		writeFile(repo, "README.md", "# updated\n")

		tool := filepath.Join(repo, "silent-diff")
		writeFile(repo, "silent-diff", "#!/bin/sh\n# produces nothing\n")
		run(repo, "chmod", "+x", tool)

		_, err := git.ExternalDiff(repo, tool, nil)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("no output"))
	})
})

var _ = Describe("LogCommits", func() {
	It("returns commits in chronological order", func() {
		repo := initRepo()
		writeFile(repo, "a.txt", "a\n")
		run(repo, "git", "add", ".")
		run(repo, "git", "commit", "-m", "add a")
		writeFile(repo, "b.txt", "b\n")
		run(repo, "git", "add", ".")
		run(repo, "git", "commit", "-m", "add b")

		commits, err := git.LogCommits(repo, []string{"HEAD~2..HEAD"})
		Expect(err).NotTo(HaveOccurred())
		Expect(commits).To(HaveLen(2))
		Expect(commits[0].Subject).To(Equal("add a"))
		Expect(commits[1].Subject).To(Equal("add b"))
		Expect(commits[0].Hash).To(HaveLen(40))
		Expect(commits[1].Hash).To(HaveLen(40))
	})

	It("returns empty slice for empty range", func() {
		repo := initRepo()

		commits, err := git.LogCommits(repo, []string{"HEAD..HEAD"})
		Expect(err).NotTo(HaveOccurred())
		Expect(commits).To(BeEmpty())
	})
})

var _ = Describe("DiffTreeFiles", func() {
	It("returns files changed in a specific commit", func() {
		repo := initRepo()
		writeFile(repo, "src/main.go", "package main\n")
		writeFile(repo, "src/util.go", "package main\n")
		run(repo, "git", "add", ".")
		run(repo, "git", "commit", "-m", "add source files")

		// Get the HEAD hash
		cmd := exec.Command("git", "rev-parse", "HEAD")
		cmd.Dir = repo
		hashBytes, err := cmd.Output()
		Expect(err).NotTo(HaveOccurred())

		hash := string(hashBytes[:len(hashBytes)-1]) // trim newline

		files, err := git.DiffTreeFiles(repo, hash)
		Expect(err).NotTo(HaveOccurred())
		Expect(files).To(ConsistOf(
			git.FileStatus{Status: "A", Path: "src/main.go"},
			git.FileStatus{Status: "A", Path: "src/util.go"},
		))
	})
})

var _ = Describe("PipeToPager", func() {
	It("pipes diff output through the pager command", func() {
		repo := initRepo()
		writeFile(repo, "README.md", "# updated\n")

		out, err := git.PipeToPager(repo, "cat", nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(ContainSubstring("--- a/README.md"))
		Expect(out).To(ContainSubstring("+++ b/README.md"))
	})

	It("supports pager commands with flags", func() {
		repo := initRepo()
		writeFile(repo, "README.md", "# updated\n")

		// sed is a simple command that takes flags; use it as a "pager"
		// that transforms the output.
		out, err := git.PipeToPager(repo, "sed s/hello/world/", nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(ContainSubstring("-# world"))
	})
})

var _ = Describe("RepoRoot", func() {
	It("returns the repository root directory", func() {
		repo := initRepo()
		// Create a subdirectory and ask for root from there
		subdir := filepath.Join(repo, "sub", "dir")
		Expect(os.MkdirAll(subdir, 0o755)).To(Succeed())

		root, err := git.RepoRoot(subdir)
		Expect(err).NotTo(HaveOccurred())
		// Resolve symlinks to handle macOS /var -> /private/var
		realRepo, err := filepath.EvalSymlinks(repo)
		Expect(err).NotTo(HaveOccurred())
		Expect(root).To(Equal(realRepo))
	})

	It("returns error for non-repo directory", func() {
		dir := GinkgoT().TempDir()

		_, err := git.RepoRoot(dir)
		Expect(err).To(HaveOccurred())
	})
})
