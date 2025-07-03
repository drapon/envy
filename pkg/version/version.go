package version

import (
	"fmt"
	"runtime"
)

var (
	// Version is the current version of envy
	Version = "dev"

	// BuildTime is the time when the binary was built
	BuildTime = "unknown"

	// GitCommit is the git commit hash
	GitCommit = "unknown"
)

// BuildInfo contains build information
type BuildInfo struct {
	Version   string
	BuildTime string
	GitCommit string
	GoVersion string
	Platform  string
}

// GetBuildInfo returns the build information
func GetBuildInfo() BuildInfo {
	return BuildInfo{
		Version:   Version,
		BuildTime: BuildTime,
		GitCommit: GitCommit,
		GoVersion: runtime.Version(),
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}

// String returns a formatted version string
func (b BuildInfo) String() string {
	return fmt.Sprintf("envy version %s (commit: %s, built: %s)\nGo version: %s\nPlatform: %s",
		b.Version, b.GitCommit, b.BuildTime, b.GoVersion, b.Platform)
}

// Short returns a short version string
func (b BuildInfo) Short() string {
	return fmt.Sprintf("envy %s", b.Version)
}