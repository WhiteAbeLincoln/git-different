package config

import (
	"os"
	"path/filepath"
	"runtime"
	"time"

	"gopkg.in/yaml.v3"
)

type UIConfig struct {
	Icons           string `yaml:"icons"` // "nerd-fonts-status" (default), "nerd-fonts-simple", "nerd-fonts-filetype", "nerd-fonts-full", "unicode", "ascii"
	Pager           string `yaml:"pager"`
	ExternalDiff    string `yaml:"externalDiff"`
	FileTreeWidth   int    `yaml:"fileTreeWidth"`
	SearchTreeWidth int    `yaml:"searchTreeWidth"`
	HideHeader      bool   `yaml:"hideHeader"`
	HideFooter      bool   `yaml:"hideFooter"`
	ShowFileTree    bool   `yaml:"showFileTree"`
	ColorFileNames  bool   `yaml:"colorFileNames"` // Color filenames by git status (default: true)
	ShowDiffStats   bool   `yaml:"showDiffStats"`  // Show the amount of lines added / removed next to the file
}

type WatchConfig struct {
	Cmd      string
	Interval time.Duration
	Enabled  bool
}

type Config struct {
	UI    UIConfig    `yaml:"ui"`
	Watch WatchConfig `yaml:"-"`
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
			Pager:           "delta --paging=never",
			ExternalDiff:    "",
		},
	}
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
