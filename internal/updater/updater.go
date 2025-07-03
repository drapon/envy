// Package updater provides automatic update checking functionality
package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/drapon/envy/internal/log"
	"github.com/drapon/envy/internal/version"
)

const (
	// GitHubAPIURL is the GitHub API endpoint for releases
	GitHubAPIURL = "https://api.github.com/repos/drapon/envy/releases/latest"

	// UpdateCheckInterval is the minimum interval between update checks
	UpdateCheckInterval = 24 * time.Hour

	// UpdateCacheFile is the cache file for update check results
	UpdateCacheFile = ".envy-update-cache"
)

// Release represents a GitHub release
type Release struct {
	Version      string    `json:"tag_name"`
	Name         string    `json:"name"`
	Draft        bool      `json:"draft"`
	Prerelease   bool      `json:"prerelease"`
	PublishedAt  time.Time `json:"published_at"`
	ReleaseNotes string    `json:"body"`
	Assets       []Asset   `json:"assets"`
}

// Asset represents a release asset
type Asset struct {
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	DownloadURL string `json:"browser_download_url"`
	ContentType string `json:"content_type"`
}

// UpdateCache represents cached update check results
type UpdateCache struct {
	LastCheck     time.Time `json:"last_check"`
	LatestVersion string    `json:"latest_version"`
	ReleaseNotes  string    `json:"release_notes,omitempty"`
}

// Updater handles update checking
type Updater struct {
	httpClient *http.Client
	cacheDir   string
}

// New creates a new updater
func New() *Updater {
	return &Updater{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		cacheDir: getCacheDir(),
	}
}

// CheckForUpdate checks if a new version is available
func (u *Updater) CheckForUpdate(ctx context.Context, currentVersion string) (*Release, error) {

	// Check cache first
	if cached, err := u.loadCache(); err == nil {
		if time.Since(cached.LastCheck) < UpdateCheckInterval {
			log.Debug("Using cached update check results")
			if version.IsNewer(cached.LatestVersion, currentVersion) {
				return &Release{
					Version:      cached.LatestVersion,
					ReleaseNotes: cached.ReleaseNotes,
				}, nil
			}
			return nil, nil
		}
	}

	// Fetch latest release from GitHub
	log.Debug("Fetching latest release info from GitHub")
	release, err := u.fetchLatestRelease(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest release: %w", err)
	}

	// Skip pre-releases and drafts
	if release.Draft || release.Prerelease {
		log.Debug("Skipping latest release as it's a draft or pre-release")
		return nil, nil
	}

	// Cache the result
	u.saveCache(&UpdateCache{
		LastCheck:     time.Now(),
		LatestVersion: release.Version,
		ReleaseNotes:  release.ReleaseNotes,
	})

	// Check if newer version is available
	if version.IsNewer(release.Version, currentVersion) {
		log.Info("New version available", log.Field("new_version", release.Version))
		return release, nil
	}

	return nil, nil
}

// fetchLatestRelease fetches the latest release from GitHub
func (u *Updater) fetchLatestRelease(ctx context.Context) (*Release, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", GitHubAPIURL, nil)
	if err != nil {
		return nil, err
	}

	// Add headers
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", fmt.Sprintf("envy/%s", version.Version))

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(body))
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to decode release: %w", err)
	}

	return &release, nil
}

// GetDownloadURL returns the download URL for the current platform
func (u *Updater) GetDownloadURL(release *Release) (string, error) {
	platform := fmt.Sprintf("%s_%s", runtime.GOOS, runtime.GOARCH)

	for _, asset := range release.Assets {
		if strings.Contains(asset.Name, platform) {
			return asset.DownloadURL, nil
		}
	}

	return "", fmt.Errorf("no download available for platform %s", platform)
}

// DownloadBinary downloads the binary for the current platform
func (u *Updater) DownloadBinary(ctx context.Context, release *Release, destPath string) error {

	downloadURL, err := u.GetDownloadURL(release)
	if err != nil {
		return err
	}

	log.Debug("Downloading binary", log.Field("url", downloadURL))

	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", fmt.Sprintf("envy/%s", version.Version))

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Create temporary file
	tempFile, err := os.CreateTemp(filepath.Dir(destPath), ".envy-update-*")
	if err != nil {
		return err
	}
	defer os.Remove(tempFile.Name())

	// Download to temporary file
	if _, err := io.Copy(tempFile, resp.Body); err != nil {
		tempFile.Close()
		return err
	}
	tempFile.Close()

	// Make executable
	if err := os.Chmod(tempFile.Name(), 0755); err != nil {
		return err
	}

	// Replace the current binary
	return os.Rename(tempFile.Name(), destPath)
}

// loadCache loads the update cache
func (u *Updater) loadCache() (*UpdateCache, error) {
	cacheFile := filepath.Join(u.cacheDir, UpdateCacheFile)

	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil, err
	}

	var cache UpdateCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}

	return &cache, nil
}

// saveCache saves the update cache
func (u *Updater) saveCache(cache *UpdateCache) error {
	if u.cacheDir == "" {
		return nil // Skip caching if no cache dir
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(u.cacheDir, 0755); err != nil {
		return err
	}

	cacheFile := filepath.Join(u.cacheDir, UpdateCacheFile)

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cacheFile, data, 0644)
}

// getCacheDir returns the cache directory
func getCacheDir() string {
	// Try XDG_CACHE_HOME first
	if cacheHome := os.Getenv("XDG_CACHE_HOME"); cacheHome != "" {
		return filepath.Join(cacheHome, "envy")
	}

	// Fall back to home directory
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".cache", "envy")
	}

	// Last resort: use temp directory
	return filepath.Join(os.TempDir(), "envy-cache")
}

// CheckAndNotify performs a background update check and notifies if update is available
func CheckAndNotify(ctx context.Context, currentVersion string) {
	// Skip in development versions
	if currentVersion == "dev" || strings.Contains(currentVersion, "-dev") {
		return
	}

	// Run in background
	go func() {
		u := New()
		release, err := u.CheckForUpdate(ctx, currentVersion)
		if err != nil {
			// Silently ignore errors in background check
			return
		}

		if release != nil {
			// Print notification to stderr so it doesn't interfere with stdout
			fmt.Fprintf(os.Stderr, "\nA new version of envy is available: %s (current: %s)\n",
				release.Version, currentVersion)
			fmt.Fprintf(os.Stderr, "Run 'envy version --check-update' for more details.\n\n")
		}
	}()
}
