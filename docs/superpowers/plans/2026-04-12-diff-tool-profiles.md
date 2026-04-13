# Diff Tool Profiles Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace top-level pager/externalDiff config with named profiles that bundle those settings, add a `--profile` CLI flag, and swap the auto-detect priority to prefer delta over difftastic.

**Architecture:** A new `Profile` struct holds pager + externalDiff. `UIConfig` gains a `Profiles` map and `ProfileName` field deserialized from YAML, while `Pager` and `ExternalDiff` become runtime-only. A single `ResolveProfile` function handles the full resolution chain: CLI flags > profile lookup > PATH auto-detect.

**Tech Stack:** Go, Cobra (CLI), yaml.v3 (config), Ginkgo/Gomega (tests)

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `pkg/config/config.go` | Modify | Add `Profile` struct, `builtinProfiles` map, update `UIConfig`, rename `ResolveDiffTool` to `ResolveProfile`, add validation in `Load()` |
| `pkg/config/config_test.go` | Modify | Update existing tests, add profile selection and validation tests |
| `cmd/root.go` | Modify | Add `--profile` flag, wire it into config, replace `ResolveDiffTool` call with `ResolveProfile` |

No changes to `pkg/git/git.go`, `pkg/ui/tui.go`, or any other files.

---

### Task 1: Add Profile struct and builtinProfiles map

**Files:**
- Modify: `pkg/config/config.go:17-28` (UIConfig struct area)
- Test: `pkg/config/config_test.go`

- [ ] **Step 1: Write failing test for builtin profile lookup**

Add a new `Describe("builtinProfiles")` block in `config_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/config/ --ginkgo.focus="builtinProfiles" -v`
Expected: compilation error — `builtinProfiles` undefined, `Profile` type undefined.

- [ ] **Step 3: Add Profile struct and builtinProfiles map**

In `config.go`, add after the imports (before `UIConfig`):

```go
// Profile bundles a pager and/or external diff tool into a named preset.
type Profile struct {
	Pager        string `yaml:"pager"`
	ExternalDiff string `yaml:"externalDiff"`
}

// builtinProfiles are the built-in presets. User-defined profiles cannot use
// the "builtin-" prefix.
var builtinProfiles = map[string]Profile{
	"builtin-delta":      {Pager: "delta --paging=never --true-color=always"},
	"builtin-difftastic": {ExternalDiff: "difft"},
	"builtin-bat":        {Pager: "bat --color=always --language=Diff --style=-header"},
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/config/ --ginkgo.focus="builtinProfiles" -v`
Expected: 3 passing tests.

- [ ] **Step 5: Commit**

```bash
git add pkg/config/config.go pkg/config/config_test.go
git commit -m "Add Profile struct and builtinProfiles map"
```

---

### Task 2: Update UIConfig to use profiles

**Files:**
- Modify: `pkg/config/config.go:17-56` (UIConfig struct and DefaultConfig)

- [ ] **Step 1: Update UIConfig struct**

Change `UIConfig` so `Pager` and `ExternalDiff` are runtime-only (`yaml:"-"`), and add the two new YAML-deserialized fields:

```go
type UIConfig struct {
	Icons           string             `yaml:"icons"`
	ProfileName     string             `yaml:"profile"`
	Profiles        map[string]Profile `yaml:"profiles"`
	Pager           string             `yaml:"-"`
	ExternalDiff    string             `yaml:"-"`
	FileTreeWidth   int                `yaml:"fileTreeWidth"`
	SearchTreeWidth int                `yaml:"searchTreeWidth"`
	HideHeader      bool               `yaml:"hideHeader"`
	HideFooter      bool               `yaml:"hideFooter"`
	ShowFileTree    bool               `yaml:"showFileTree"`
	ColorFileNames  bool               `yaml:"colorFileNames"`
	ShowDiffStats   bool               `yaml:"showDiffStats"`
}
```

- [ ] **Step 2: Update DefaultConfig**

Remove `Pager` and `ExternalDiff` from `DefaultConfig` (they default to zero-value `""` which is correct):

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
		},
	}
}
```

- [ ] **Step 3: Run all tests to check for regressions**

Run: `go test ./pkg/config/ -v`
Expected: existing `ResolveDiffTool` tests still pass (they set `cfg.UI.Pager`/`cfg.UI.ExternalDiff` directly, which still works since those fields exist).

- [ ] **Step 4: Commit**

```bash
git add pkg/config/config.go
git commit -m "Make Pager/ExternalDiff runtime-only, add ProfileName and Profiles to UIConfig"
```

---

### Task 3: Add validation in Load()

**Files:**
- Modify: `pkg/config/config.go:135-153` (Load function)
- Test: `pkg/config/config_test.go`

- [ ] **Step 1: Write failing test for builtin- prefix rejection**

Add a new `Describe("Load")` block:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/config/ --ginkgo.focus="Load" -v`
Expected: compilation error — `validate` method undefined.

- [ ] **Step 3: Add validate method and wire into Load**

Add a `validate` method on `Config`:

```go
// validate checks that user-defined profile names don't collide with the
// reserved "builtin-" namespace.
func (c Config) validate() error {
	for name := range c.UI.Profiles {
		if strings.HasPrefix(name, "builtin-") {
			return fmt.Errorf("profile name %q uses reserved prefix \"builtin-\"", name)
		}
	}
	return nil
}
```

Add `"fmt"` and `"strings"` to the import block.

Update `Load()` to call `validate()` after unmarshalling:

```go
func Load() Config {
	cfg := DefaultConfig()

	configPath := getConfigFilePath()
	if configPath == "" {
		return cfg
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return cfg
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return DefaultConfig()
	}

	if err := cfg.validate(); err != nil {
		return DefaultConfig()
	}

	return cfg
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/config/ --ginkgo.focus="Load" -v`
Expected: 2 passing tests.

- [ ] **Step 5: Commit**

```bash
git add pkg/config/config.go pkg/config/config_test.go
git commit -m "Validate that user-defined profile names don't use builtin- prefix"
```

---

### Task 4: Replace ResolveDiffTool with ResolveProfile

**Files:**
- Modify: `pkg/config/config.go:58-91` (ResolveDiffTool function)
- Test: `pkg/config/config_test.go`

- [ ] **Step 1: Write failing tests for profile resolution**

Replace the entire `Describe("ResolveDiffTool")` block with a new `Describe("ResolveProfile")` block. This covers auto-detect (with new priority), profile lookup, and CLI override:

```go
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
			// This can't actually happen because of validation, but the
			// lookup logic should prefer user profiles regardless.
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/config/ --ginkgo.focus="ResolveProfile" -v`
Expected: compilation error — `ResolveProfile` undefined.

- [ ] **Step 3: Replace ResolveDiffTool with ResolveProfile**

Delete `ResolveDiffTool` and add `ResolveProfile` in its place:

```go
// ResolveProfile populates the runtime Pager and ExternalDiff fields on
// UIConfig. Resolution order:
//
//  1. CLI flags (Pager/ExternalDiff already set on cfg) — returned as-is.
//  2. Named profile (cfg.UI.ProfileName) — looked up in user-defined
//     profiles first, then built-in profiles.
//  3. Auto-detect from PATH: delta > difftastic > bat > raw git diff.
func ResolveProfile(cfg Config) (Config, error) {
	// 1. CLI flags take highest priority.
	if cfg.UI.Pager != "" || cfg.UI.ExternalDiff != "" {
		return cfg, nil
	}

	// 2. Named profile.
	if cfg.UI.ProfileName != "" {
		if p, ok := cfg.UI.Profiles[cfg.UI.ProfileName]; ok {
			cfg.UI.Pager = p.Pager
			cfg.UI.ExternalDiff = p.ExternalDiff
			return cfg, nil
		}
		if p, ok := builtinProfiles[cfg.UI.ProfileName]; ok {
			cfg.UI.Pager = p.Pager
			cfg.UI.ExternalDiff = p.ExternalDiff
			return cfg, nil
		}
		return cfg, fmt.Errorf("unknown profile %q", cfg.UI.ProfileName)
	}

	// 3. Auto-detect from PATH.
	if _, err := lookPath("delta"); err == nil {
		cfg.UI.Pager = "delta --paging=never --true-color=always"
		return cfg, nil
	}
	if _, err := lookPath("difft"); err == nil {
		cfg.UI.ExternalDiff = "difft"
		return cfg, nil
	}
	if _, err := lookPath("bat"); err == nil {
		cfg.UI.Pager = "bat --color=always --language=Diff --style=-header"
		return cfg, nil
	}
	return cfg, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/config/ -v`
Expected: all tests pass (builtinProfiles, Load, ResolveProfile).

- [ ] **Step 5: Run linters**

Run: `golangci-lint run --fix ./pkg/config/... && golangci-lint fmt ./pkg/config/... && betteralign -apply ./pkg/config/...`
Fix any issues.

- [ ] **Step 6: Commit**

```bash
git add pkg/config/config.go pkg/config/config_test.go
git commit -m "Replace ResolveDiffTool with ResolveProfile

Profiles bundle pager + externalDiff into named presets. Resolution
order: CLI flags > named profile > PATH auto-detect (delta > difftastic
> bat > raw)."
```

---

### Task 5: Wire --profile flag into cmd/root.go

**Files:**
- Modify: `cmd/root.go:70-185` (init function)

- [ ] **Step 1: Add --profile flag**

In the `init()` function, after the `external-diff` flag (line 72), add:

```go
rootCmd.Flags().String("profile", "", "Diff tool profile (overrides config)")
```

- [ ] **Step 2: Parse the flag in the Run function**

After the `externalDiffFlag` parsing block (after line 88), add:

```go
profileFlag, err := cmd.Flags().GetString("profile")
if err != nil {
	log.Fatal("Cannot parse the profile flag", err)
}
```

- [ ] **Step 3: Wire flag into config and update ResolveProfile call**

Replace the block at lines 157-168 (CLI override + ResolveDiffTool call):

```go
// Override config with CLI flags
if pagerFlag != "" {
	cfg.UI.Pager = pagerFlag
}
if externalDiffFlag != "" {
	cfg.UI.ExternalDiff = externalDiffFlag
}
if profileFlag != "" {
	cfg.UI.ProfileName = profileFlag
}

cfg, err = config.ResolveProfile(cfg)
if err != nil {
	fmt.Println("Error resolving diff tool profile:", err)
	os.Exit(1)
}
```

Note: `err` is already declared earlier in the function, so use `=` not `:=`. The existing `err` variable from `cmd.Flags().GetBool("watch")` is suitable for reuse, but since that's a separate block, declare with `cfg, err =` (reassignment).

- [ ] **Step 4: Remove the old comment above ResolveDiffTool call**

Delete the comment block at lines 165-168:
```
// Auto-detect a diff tool when nothing was set on the command line
// or in the config file: prefer difftastic, then delta, then fall
// back to git's raw unified-diff output.
```

- [ ] **Step 5: Build to verify compilation**

Run: `go build ./...`
Expected: compiles successfully.

- [ ] **Step 6: Run all tests**

Run: `go test ./... -v`
Expected: all tests pass.

- [ ] **Step 7: Run linters**

Run: `golangci-lint run --fix ./... && golangci-lint fmt ./... && betteralign -apply ./...`
Fix any issues.

- [ ] **Step 8: Commit**

```bash
git add cmd/root.go
git commit -m "Add --profile CLI flag for selecting diff tool profiles"
```
