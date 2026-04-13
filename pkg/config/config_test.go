package config

import (
	"errors"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Config Suite")
}

// fakeLookPath returns a stub of exec.LookPath that reports the given names
// as present in PATH. Any lookup for a name not in `present` returns an error.
func fakeLookPath(present ...string) func(string) (string, error) {
	set := make(map[string]struct{}, len(present))
	for _, p := range present {
		set[p] = struct{}{}
	}
	return func(name string) (string, error) {
		if _, ok := set[name]; ok {
			return "/usr/bin/" + name, nil
		}
		return "", errors.New("not found")
	}
}

var _ = Describe("builtinProfiles", func() {
	It("contains builtin-delta as a pager profile", func() {
		p, ok := builtinProfiles["builtin-delta"]
		Expect(ok).To(BeTrue())
		Expect(p.Pager).To(Equal("delta --paging=never --true-color=always"))
		Expect(p.ExternalDiff).To(BeEmpty())
	})

	It("contains builtin-difftastic as an external diff profile", func() {
		p, ok := builtinProfiles["builtin-difftastic"]
		Expect(ok).To(BeTrue())
		Expect(p.ExternalDiff).To(Equal("difft"))
		Expect(p.Pager).To(BeEmpty())
	})

	It("contains builtin-bat as a pager profile", func() {
		p, ok := builtinProfiles["builtin-bat"]
		Expect(ok).To(BeTrue())
		Expect(p.Pager).To(Equal("bat --color=always --language=Diff --style=-header"))
		Expect(p.ExternalDiff).To(BeEmpty())
	})
})

var _ = Describe("ResolveDiffTool", func() {
	var originalLookPath func(string) (string, error)

	BeforeEach(func() {
		originalLookPath = lookPath
	})

	AfterEach(func() {
		lookPath = originalLookPath
	})

	Context("when neither tool is configured", func() {
		It("prefers difftastic when available", func() {
			lookPath = fakeLookPath("difft", "delta")

			got := ResolveDiffTool(DefaultConfig())
			Expect(got.UI.ExternalDiff).To(Equal("difft"))
			Expect(got.UI.Pager).To(BeEmpty())
		})

		It("falls back to delta when difftastic is missing", func() {
			lookPath = fakeLookPath("delta", "bat")

			got := ResolveDiffTool(DefaultConfig())
			Expect(got.UI.ExternalDiff).To(BeEmpty())
			Expect(got.UI.Pager).To(Equal("delta --paging=never --true-color=always"))
		})

		It("falls back to bat when difftastic and delta are missing", func() {
			lookPath = fakeLookPath("bat")

			got := ResolveDiffTool(DefaultConfig())
			Expect(got.UI.ExternalDiff).To(BeEmpty())
			Expect(got.UI.Pager).To(Equal("bat --color=always --language=Diff --style=-header"))
		})

		It("leaves both empty when none of the tools are on PATH", func() {
			lookPath = fakeLookPath()

			got := ResolveDiffTool(DefaultConfig())
			Expect(got.UI.ExternalDiff).To(BeEmpty())
			Expect(got.UI.Pager).To(BeEmpty())
		})
	})

	Context("when the user has configured a tool", func() {
		It("respects an explicit pager even when difftastic is on PATH", func() {
			lookPath = fakeLookPath("difft", "delta")

			cfg := DefaultConfig()
			cfg.UI.Pager = "bat --style=plain"

			got := ResolveDiffTool(cfg)
			Expect(got.UI.Pager).To(Equal("bat --style=plain"))
			Expect(got.UI.ExternalDiff).To(BeEmpty())
		})

		It("respects an explicit external diff even when delta is on PATH", func() {
			lookPath = fakeLookPath("delta")

			cfg := DefaultConfig()
			cfg.UI.ExternalDiff = "difft --display=inline"

			got := ResolveDiffTool(cfg)
			Expect(got.UI.ExternalDiff).To(Equal("difft --display=inline"))
			Expect(got.UI.Pager).To(BeEmpty())
		})
	})
})
