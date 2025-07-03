// Package version provides version information for the envy application
package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
)

var (
	// Version is the semantic version of the application
	Version = "dev"

	// GitCommit is the git commit hash
	GitCommit = "unknown"

	// BuildDate is the build timestamp
	BuildDate = "unknown"
)

func init() {
	// 埋め込まれたバージョン情報があれば使用
	if embeddedVersion != "" && embeddedVersion != "dev" {
		Version = embeddedVersion
	}
	if embeddedGitCommit != "" && embeddedGitCommit != "unknown" {
		GitCommit = embeddedGitCommit
	}
	if embeddedBuildDate != "" && embeddedBuildDate != "unknown" {
		BuildDate = embeddedBuildDate
	}

	// ビルド情報から取得を試みる
	if info, ok := debug.ReadBuildInfo(); ok {
		if Version == "dev" && info.Main.Version != "" && info.Main.Version != "(devel)" {
			Version = info.Main.Version
		}

		// VCS情報を取得
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				if GitCommit == "unknown" && setting.Value != "" {
					GitCommit = setting.Value
					if len(GitCommit) > 7 {
						GitCommit = GitCommit[:7]
					}
				}
			case "vcs.time":
				if BuildDate == "unknown" && setting.Value != "" {
					BuildDate = setting.Value
				}
			}
		}
	}
}

// Info holds version information
type Info struct {
	Version   string
	GitCommit string
	BuildDate string
	GoVersion string
	Compiler  string
	Platform  string
	OS        string
	Arch      string
}

// GetInfo returns version information
func GetInfo() *Info {
	return &Info{
		Version:   Version,
		GitCommit: GitCommit,
		BuildDate: BuildDate,
		GoVersion: runtime.Version(),
		Compiler:  runtime.Compiler,
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}
}

// String returns a simple version string
func (i *Info) String() string {
	return fmt.Sprintf("envy version %s", i.Version)
}

// DetailedString returns detailed version information
func (i *Info) DetailedString() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("envy version %s\n", i.Version))
	sb.WriteString(fmt.Sprintf("  Git commit:  %s\n", i.GitCommit))
	sb.WriteString(fmt.Sprintf("  Build date:  %s\n", i.BuildDate))
	sb.WriteString(fmt.Sprintf("  Go version:  %s\n", i.GoVersion))
	sb.WriteString(fmt.Sprintf("  Compiler:    %s\n", i.Compiler))
	sb.WriteString(fmt.Sprintf("  Platform:    %s\n", i.Platform))

	return sb.String()
}

// Compare compares two version strings
// Returns:
//
//	-1 if v1 < v2
//	 0 if v1 == v2
//	 1 if v1 > v2
func Compare(v1, v2 string) int {
	// Remove 'v' prefix if present
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	// Handle dev versions
	if v1 == "dev" {
		if v2 == "dev" {
			return 0
		}
		return -1
	}
	if v2 == "dev" {
		return 1
	}

	// Parse versions
	parts1 := parseVersion(v1)
	parts2 := parseVersion(v2)

	// Compare major, minor, patch
	for i := 0; i < 3; i++ {
		if i >= len(parts1) {
			if i >= len(parts2) {
				return 0
			}
			return -1
		}
		if i >= len(parts2) {
			return 1
		}

		if parts1[i] < parts2[i] {
			return -1
		}
		if parts1[i] > parts2[i] {
			return 1
		}
	}

	return 0
}

// parseVersion parses a version string into major, minor, patch
func parseVersion(v string) []int {
	parts := strings.Split(v, ".")
	result := make([]int, 3)

	for i, part := range parts {
		if i >= 3 {
			break
		}

		// Remove any pre-release suffix
		if idx := strings.IndexAny(part, "-+"); idx >= 0 {
			part = part[:idx]
		}

		var num int
		fmt.Sscanf(part, "%d", &num)
		result[i] = num
	}

	return result
}

// IsNewer checks if v1 is newer than v2
func IsNewer(v1, v2 string) bool {
	return Compare(v1, v2) > 0
}
