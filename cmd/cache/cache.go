package cache

import (
	"fmt"

	"github.com/drapon/envy/cmd/root"
	"github.com/drapon/envy/internal/cache"
	"github.com/drapon/envy/internal/log"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	clearAll bool
	stats    bool
)

// CacheCmd represents cache management commands
var CacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage cache",
	Long: `Display cache statistics and perform clear operations.

Cache temporarily stores environment variable retrieval and
configuration file parsing results to improve performance.`,
	Example: `  # Display cache statistics
  envy cache --stats

  # Clear all cache
  envy cache --clear

  # Run command without cache
  envy pull --no-cache

  # Run command after clearing cache
  envy push --clear-cache`,
	RunE: runCache,
}

func init() {
	root.GetRootCmd().AddCommand(CacheCmd)

	CacheCmd.Flags().BoolVar(&clearAll, "clear", false, "Clear all cache")
	CacheCmd.Flags().BoolVar(&stats, "stats", false, "Display cache statistics")
}

func runCache(cmd *cobra.Command, args []string) error {
	logger := log.WithContext(zap.String("command", "cache"))

	if clearAll {
		return clearCache(logger)
	}

	if stats {
		return showCacheStats(logger)
	}

	// Default to showing statistics
	return showCacheStats(logger)
}

// clearCache clears the cache
func clearCache(logger *zap.Logger) error {
	cacheManager := cache.GetGlobalCache()
	if cacheManager == nil {
		fmt.Println("Cache is not initialized")
		return nil
	}

	// Get statistics before clearing
	statsBefore := cacheManager.Stats()

	// Clear cache
	if err := cacheManager.Clear(); err != nil {
		logger.Error("Failed to clear cache", zap.Error(err))
		return fmt.Errorf("failed to clear cache: %w", err)
	}

	fmt.Printf("Cache cleared successfully\n")
	fmt.Printf("  Entries deleted: %d\n", statsBefore.Entries)
	fmt.Printf("  Size freed: %s\n", formatSize(statsBefore.Size))

	logger.Info("Cache cleared",
		zap.Int("entries_cleared", statsBefore.Entries),
		zap.Int64("size_freed", statsBefore.Size))

	return nil
}

// showCacheStats displays cache statistics
func showCacheStats(logger *zap.Logger) error {
	cacheManager := cache.GetGlobalCache()
	if cacheManager == nil {
		fmt.Println("Cache is not initialized")
		return nil
	}

	stats := cacheManager.Stats()
	formattedStats := cache.FormatCacheStats(stats)
	
	fmt.Print(formattedStats)

	logger.Debug("Displayed cache statistics",
		zap.Int64("hits", stats.Hits),
		zap.Int64("misses", stats.Misses),
		zap.Int("entries", stats.Entries),
		zap.Int64("size", stats.Size))

	return nil
}

// formatSize formats byte size into human-readable format
func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)
	
	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.2f TB", float64(bytes)/TB)
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}