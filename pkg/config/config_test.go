package config

import (
	"errors"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
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

var _ = Describe("Load", func() {
	It("rejects user-defined profiles with builtin- prefix", func() {
		yamlData := []byte(`
ui:
  profiles:
    builtin-mine:
      pager: "delta"
`)
		var cfg Config
		err := yaml.Unmarshal(yamlData, &cfg)
		Expect(err).NotTo(HaveOccurred())

		err = cfg.validate()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("builtin-"))
	})

	It("accepts user-defined profiles without builtin- prefix", func() {
		yamlData := []byte(`
ui:
  profiles:
    my-delta:
      pager: "delta --side-by-side"
`)
		var cfg Config
		err := yaml.Unmarshal(yamlData, &cfg)
		Expect(err).NotTo(HaveOccurred())

		err = cfg.validate()
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = Describe("ResolveProfile", func() {
	var originalLookPath func(string) (string, error)

	BeforeEach(func() {
		originalLookPath = lookPath
	})

	AfterEach(func() {
		lookPath = originalLookPath
	})

	Context("auto-detect when no profile is selected", func() {
		It("prefers delta over difftastic when both are available", func() {
			lookPath = fakeLookPath("difft", "delta")

			got, err := ResolveProfile(DefaultConfig())
			Expect(err).NotTo(HaveOccurred())
			Expect(got.UI.Pager).To(Equal("delta --paging=never --true-color=always"))
			Expect(got.UI.ExternalDiff).To(BeEmpty())
		})

		It("falls back to difftastic when delta is missing", func() {
			lookPath = fakeLookPath("difft", "bat")

			got, err := ResolveProfile(DefaultConfig())
			Expect(err).NotTo(HaveOccurred())
			Expect(got.UI.ExternalDiff).To(Equal("difft"))
			Expect(got.UI.Pager).To(BeEmpty())
		})

		It("falls back to bat when delta and difftastic are missing", func() {
			lookPath = fakeLookPath("bat")

			got, err := ResolveProfile(DefaultConfig())
			Expect(err).NotTo(HaveOccurred())
			Expect(got.UI.ExternalDiff).To(BeEmpty())
			Expect(got.UI.Pager).To(Equal("bat --color=always --language=Diff --style=-header"))
		})

		It("leaves both empty when none of the tools are on PATH", func() {
			lookPath = fakeLookPath()

			got, err := ResolveProfile(DefaultConfig())
			Expect(err).NotTo(HaveOccurred())
			Expect(got.UI.ExternalDiff).To(BeEmpty())
			Expect(got.UI.Pager).To(BeEmpty())
		})
	})

	Context("when a profile is selected", func() {
		It("resolves a user-defined profile by name", func() {
			cfg := DefaultConfig()
			cfg.UI.ProfileName = "my-delta"
			cfg.UI.Profiles = map[string]Profile{
				"my-delta": {Pager: "delta --side-by-side --paging=never"},
			}

			got, err := ResolveProfile(cfg)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.UI.Pager).To(Equal("delta --side-by-side --paging=never"))
			Expect(got.UI.ExternalDiff).To(BeEmpty())
		})

		It("resolves a built-in profile by name", func() {
			cfg := DefaultConfig()
			cfg.UI.ProfileName = "builtin-difftastic"

			got, err := ResolveProfile(cfg)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.UI.ExternalDiff).To(Equal("difft"))
			Expect(got.UI.Pager).To(BeEmpty())
		})

		It("prefers user-defined profile over built-in with same name", func() {
			cfg := DefaultConfig()
			cfg.UI.ProfileName = "custom"
			cfg.UI.Profiles = map[string]Profile{
				"custom": {Pager: "my-custom-pager"},
			}

			got, err := ResolveProfile(cfg)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.UI.Pager).To(Equal("my-custom-pager"))
		})

		It("returns an error for a nonexistent profile name", func() {
			cfg := DefaultConfig()
			cfg.UI.ProfileName = "does-not-exist"

			_, err := ResolveProfile(cfg)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("does-not-exist"))
		})
	})

	Context("when CLI flags override", func() {
		It("uses CLI pager even when a profile is selected", func() {
			cfg := DefaultConfig()
			cfg.UI.ProfileName = "builtin-difftastic"
			cfg.UI.Pager = "bat --style=plain"

			got, err := ResolveProfile(cfg)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.UI.Pager).To(Equal("bat --style=plain"))
			Expect(got.UI.ExternalDiff).To(BeEmpty())
		})

		It("uses CLI external diff even when a profile is selected", func() {
			cfg := DefaultConfig()
			cfg.UI.ProfileName = "builtin-delta"
			cfg.UI.ExternalDiff = "difft --display=inline"

			got, err := ResolveProfile(cfg)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.UI.ExternalDiff).To(Equal("difft --display=inline"))
			Expect(got.UI.Pager).To(BeEmpty())
		})
	})
})
