package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

// ResolveDiffTool fills in a default pager/externalDiff when the user hasn't
// configured either. The priority is difftastic, then delta, then bat, then
// git's raw unified-diff output. If either field is already set (via config
// or CLI flag) the config is returned unchanged.
//
// The bat default disables bat's own file header via "--style=-header"
// because git-different already renders a file header above the diff
// viewport.
//
// The delta default forces "--true-color=always". Delta's auto-detection
// reads COLORTERM but also gates on stdout being a TTY, and when delta is
// launched as a subprocess of the TUI its stdout is a pipe — so auto ends
// up downgrading to 8-bit color even when COLORTERM=truecolor is inherited
// correctly.
func ResolveDiffTool(cfg Config) Config {
	if cfg.UI.Pager != "" || cfg.UI.ExternalDiff != "" {
		return cfg
	}
	if _, err := lookPath("difft"); err == nil {
		cfg.UI.ExternalDiff = "difft"
		return cfg
	}
	if _, err := lookPath("delta"); err == nil {
		cfg.UI.Pager = "delta --paging=never --true-color=always"
		return cfg
	}
	if _, err := lookPath("bat"); err == nil {
		cfg.UI.Pager = "bat --color=always --language=Diff --style=-header"
		return cfg
	}
	// None of the tools are on PATH — leave both empty so PipeToPager
	// falls through to returning raw "git diff" output.
	return cfg
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

	return cfg
}
