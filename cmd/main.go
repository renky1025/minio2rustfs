package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"minio2rustfs/internal/app"
	"minio2rustfs/internal/config"
	"minio2rustfs/internal/logger"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	configFile string
	cfg        *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "minio2rustfs",
	Short: "Migrate objects from MinIO to RustFS (S3 compatible)",
	Long:  `A concurrent, resumable object migration tool from MinIO to RustFS with support for checkpointing, retry, and monitoring.`,
	RunE:  runMigration,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "config file (default is ./config.yaml)")

	// Source flags
	rootCmd.Flags().String("src-endpoint", "", "MinIO endpoint")
	rootCmd.Flags().String("src-access-key", "", "MinIO access key")
	rootCmd.Flags().String("src-secret-key", "", "MinIO secret key")
	rootCmd.Flags().Bool("src-secure", false, "Use HTTPS for source")

	// Destination flags
	rootCmd.Flags().String("dst-endpoint", "", "RustFS endpoint")
	rootCmd.Flags().String("dst-access-key", "", "RustFS access key")
	rootCmd.Flags().String("dst-secret-key", "", "RustFS secret key")
	rootCmd.Flags().Bool("dst-secure", true, "Use HTTPS for destination")

	// Migration flags
	rootCmd.Flags().String("bucket", "", "Bucket name (required)")
	rootCmd.Flags().String("prefix", "", "Object prefix filter")
	rootCmd.Flags().String("object", "", "Single object key")
	rootCmd.Flags().Int("concurrency", 16, "Number of concurrent workers")
	rootCmd.Flags().Int64("multipart-threshold", 104857600, "Multipart upload threshold in bytes")
	rootCmd.Flags().Int64("part-size", 67108864, "Multipart part size in bytes")
	rootCmd.Flags().Int("retries", 5, "Maximum retry attempts")
	rootCmd.Flags().Int("retry-backoff-ms", 500, "Initial retry backoff in milliseconds")
	rootCmd.Flags().Bool("dry-run", false, "List objects without migrating")
	rootCmd.Flags().String("checkpoint", "./checkpoint.db", "Checkpoint database file")
	rootCmd.Flags().String("log-level", "info", "Log level (debug/info/warn/error)")
	rootCmd.Flags().Bool("skip-existing", true, "Skip objects that already exist with same size/etag")
	rootCmd.Flags().Bool("resume", false, "Resume from checkpoint")
	rootCmd.Flags().Bool("show-progress", true, "Show progress display (auto-disabled for dry-run)")
}

func runMigration(cmd *cobra.Command, args []string) error {
	// Load configuration
	var err error
	cfg, err = config.Load(configFile, cmd.Flags())
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize logger
	log, err := logger.New(cfg.LogLevel)
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer log.Sync()

	// Create application
	migrator, err := app.New(cfg, log)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Info("Received shutdown signal, gracefully stopping...")
		cancel()
	}()

	// Run migration
	err = migrator.Run(ctx)

	// Close migrator resources after migration completes or is cancelled
	if closeErr := migrator.Close(); closeErr != nil {
		log.Error("Error closing migrator", zap.Error(closeErr))
	}

	return err
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
