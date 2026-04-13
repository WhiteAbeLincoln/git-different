package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// lookPath is a package-level indirection over exec.LookPath so tests can
// stub PATH resolution without mutating the real environment.
var lookPath = exec.LookPath

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

type UIConfig struct {
	Icons           string             `yaml:"icons"` // "nerd-fonts-status" (default), "nerd-fonts-simple", "nerd-fonts-filetype", "nerd-fonts-full", "unicode", "ascii"
	ProfileName     string             `yaml:"profile"`
	Profiles        map[string]Profile `yaml:"profiles"`
	Pager           string             `yaml:"-"`
	ExternalDiff    string             `yaml:"-"`
	FileTreeWidth   int                `yaml:"fileTreeWidth"`
	SearchTreeWidth int                `yaml:"searchTreeWidth"`
	HideHeader      bool               `yaml:"hideHeader"`
	HideFooter      bool               `yaml:"hideFooter"`
	ShowFileTree    bool               `yaml:"showFileTree"`
	ColorFileNames  bool               `yaml:"colorFileNames"` // Color filenames by git status (default: true)
	ShowDiffStats   bool               `yaml:"showDiffStats"`  // Show the amount of lines added / removed next to the file
}

type WatchConfig struct {
	Cmd      string
	Interval time.Duration
	Enabled  bool
}

type Config struct {
	Watch WatchConfig `yaml:"-"`
	UI    UIConfig    `yaml:"ui"`
}

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

func getConfigFilePath() string {
	var configDirs []string

	// Environment variable override - useful for development or non-standard setups.
	if dir := os.Getenv("GIT_DIFFERENT_CONFIG_DIR"); dir != "" {
		if s, err := os.Stat(dir); err == nil && s.IsDir() {
			return filepath.Join(dir, "config.yml")
		}
	}

	// On macOS, check XDG_CONFIG_HOME first (if user explicitly set it),
	// then fall back to ~/.config (common for CLI tools).
	// os.UserConfigDir() already handles this for Linux.
	if runtime.GOOS == "darwin" {
		if xdgConfigDir := os.Getenv("XDG_CONFIG_HOME"); xdgConfigDir != "" {
			configDirs = append(configDirs, xdgConfigDir)
		}
		if home := os.Getenv("HOME"); home != "" {
			configDirs = append(configDirs, filepath.Join(home, ".config"))
		}
	}

	// Standard OS-specific config directory.
	if configDir, err := os.UserConfigDir(); err == nil {
		configDirs = append(configDirs, configDir)
	}

	// Return the first config file that exists.
	for _, dir := range configDirs {
		configPath := filepath.Join(dir, "git-different", "config.yml")
		if _, err := os.Stat(configPath); err == nil {
			return configPath
		}
	}

	// If no config file exists, return the preferred path for creation.
	if len(configDirs) > 0 {
		return filepath.Join(configDirs[0], "git-different", "config.yml")
	}
	return ""
}

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
