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

	"github.com/WhiteAbeLincoln/git-different/pkg/config"
	gitpkg "github.com/WhiteAbeLincoln/git-different/pkg/git"
	"github.com/WhiteAbeLincoln/git-different/pkg/ui"
	"github.com/WhiteAbeLincoln/git-different/pkg/version"
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
	Args:               cobra.ArbitraryArgs,
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

			log.SetOutput(logFile)
			log.SetTimeFormat(time.Kitchen)
			log.SetReportCaller(true)
			log.SetLevel(log.DebugLevel)
			log.SetColorProfile(colorprofile.TrueColor)
			wd, err := os.Getwd()
			if err != nil {
				fmt.Println("Error getting current working dir", err)
				os.Exit(1)
			}
			log.Debug("Starting git-different", "logFile",
				wd+string(os.PathSeparator)+logFile.Name())
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
