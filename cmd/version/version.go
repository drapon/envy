// Package version implements the version command
package version

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/drapon/envy/internal/log"
	"github.com/drapon/envy/internal/updater"
	"github.com/drapon/envy/internal/version"
)

// Options holds the command options
type Options struct {
	Detailed     bool
	CheckUpdate  bool
	NoColor      bool
	UpdatePrompt bool
}

// NewCommand creates a new version command
func NewCommand() *cobra.Command {
	opts := &Options{}

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Long: `Show version information for envy.

This command displays the current version of envy along with build information.
It can also check for available updates.`,
		Example: `  # Show simple version information
  envy version
  
  # Show detailed version information
  envy version --detailed
  
  # Check for updates
  envy version --check-update`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVersion(cmd.Context(), opts)
		},
	}

	// Add flags
	cmd.Flags().BoolVarP(&opts.Detailed, "detailed", "d", false, "Show detailed version information")
	cmd.Flags().BoolVarP(&opts.CheckUpdate, "check-update", "c", false, "Check for available updates")
	cmd.Flags().BoolVar(&opts.NoColor, "no-color", false, "Disable colored output")
	cmd.Flags().BoolVar(&opts.UpdatePrompt, "update-prompt", true, "Show update prompt if new version is available")

	return cmd
}

// runVersion executes the version command
func runVersion(ctx context.Context, opts *Options) error {
	info := version.GetInfo()

	// Configure color output
	var color struct {
		reset  string
		bold   string
		green  string
		yellow string
		cyan   string
	}

	if !opts.NoColor {
		color.reset = "\033[0m"
		color.bold = "\033[1m"
		color.green = "\033[32m"
		color.yellow = "\033[33m"
		color.cyan = "\033[36m"
	}

	// Display version information
	if opts.Detailed {
		fmt.Printf("%s%senvy%s version %s%s%s\n",
			color.bold, color.cyan, color.reset,
			color.green, info.Version, color.reset)
		fmt.Printf("  Git commit:  %s%s%s\n", color.yellow, info.GitCommit, color.reset)
		fmt.Printf("  Build date:  %s\n", info.BuildDate)
		fmt.Printf("  Go version:  %s\n", info.GoVersion)
		fmt.Printf("  Compiler:    %s\n", info.Compiler)
		fmt.Printf("  Platform:    %s\n", info.Platform)
	} else {
		fmt.Printf("%s%s%s\n", color.green, info.String(), color.reset)
	}

	// Check for updates
	if opts.CheckUpdate || opts.UpdatePrompt {
		log.Debug("Checking for updates...")

		// Context with timeout
		checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		u := updater.New()
		release, err := u.CheckForUpdate(checkCtx, info.Version)
		if err != nil {
			if opts.CheckUpdate {
				// Show error only when explicitly requested
				log.Debug("Update check failed", log.ErrorField(err))
				fmt.Printf("\n%sFailed to check for updates: %v%s\n",
					color.yellow, err, color.reset)
			}
			return nil
		}

		if release != nil {
			fmt.Printf("\n%s%sNew version available: %s%s\n",
				color.bold, color.yellow, release.Version, color.reset)
			fmt.Printf("Release date: %s\n", release.PublishedAt.Format("2006-01-02"))

			if release.ReleaseNotes != "" {
				fmt.Printf("\nRelease notes:\n%s\n", release.ReleaseNotes)
			}

			fmt.Printf("\nHow to update:\n")
			fmt.Printf("  %sbrew upgrade envy%s  # Homebrew\n", color.cyan, color.reset)
			fmt.Printf("  %scurl -sSL https://github.com/drapon/envy/releases/latest/download/install.sh | bash%s  # Direct install\n",
				color.cyan, color.reset)

			// Suggest auto-download
			if opts.UpdatePrompt && !opts.CheckUpdate {
				fmt.Printf("\nFor more details, run %senvy version --check-update%s\n",
					color.cyan, color.reset)
			}
		} else if opts.CheckUpdate {
			fmt.Printf("\n%sYou are using the latest version!%s\n",
				color.green, color.reset)
		}
	}

	return nil
}
