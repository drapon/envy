package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersion(t *testing.T) {
	// Test version constants
	assert.NotEmpty(t, Version)
	assert.NotEmpty(t, GitCommit)
	assert.NotEmpty(t, BuildTime)
}

func TestGetBuildInfo(t *testing.T) {
	info := GetBuildInfo()
	
	assert.Equal(t, Version, info.Version)
	assert.Equal(t, BuildTime, info.BuildTime)
	assert.Equal(t, GitCommit, info.GitCommit)
	assert.NotEmpty(t, info.GoVersion)
	assert.NotEmpty(t, info.Platform)
}

func TestBuildInfoString(t *testing.T) {
	info := GetBuildInfo()
	str := info.String()
	
	assert.Contains(t, str, "envy version")
	assert.Contains(t, str, info.Version)
	assert.Contains(t, str, info.GitCommit)
	assert.Contains(t, str, info.BuildTime)
	assert.Contains(t, str, info.GoVersion)
	assert.Contains(t, str, info.Platform)
}

func TestBuildInfoShort(t *testing.T) {
	info := GetBuildInfo()
	short := info.Short()
	
	assert.Contains(t, short, "envy")
	assert.Contains(t, short, info.Version)
}