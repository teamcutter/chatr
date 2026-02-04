package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/teamcutter/chatr/internal/cache"
	"github.com/teamcutter/chatr/internal/config"
)

func newClearCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Clear the packages cache",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			c, err := cache.New(cfg.CacheDir)
			if err != nil {
				return err
			}

			size, _ := c.Size()

			if err := c.Clear(); err != nil {
				return fmt.Errorf("failed to clear cache: %w", err)
			}

			fmt.Printf("%s Cache cleared (%s freed)\n", green("✓"), formatSize(size))
			return nil
		},
	}
}

func formatSize(bytes int64) string {
	const (
		KB = 1 << 10
		MB = 1 << 20
		GB = 1 << 30
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
