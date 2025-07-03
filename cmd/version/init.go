// Package version provides the init function to register the version command
package version

import (
	"github.com/drapon/envy/cmd/root"
)

func init() {
	// Register the version command with the root command
	root.GetRootCmd().AddCommand(NewCommand())
}
