package config

import (
	"fmt"
	"os"

	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Source    S3Config  `yaml:"source"`
	Target    S3Config  `yaml:"target"`
	Migration Migration `yaml:"migration"`
	LogLevel  string    `yaml:"log_level"`
}

// S3Config represents S3-compatible storage configuration
type S3Config struct {
	Endpoint  string `yaml:"endpoint"`
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
	Secure    bool   `yaml:"secure"`
}

// Migration represents migration-specific configuration
type Migration struct {
	Bucket             string `yaml:"bucket"`
	Prefix             string `yaml:"prefix"`
	Object             string `yaml:"object"`
	Concurrency        int    `yaml:"concurrency"`
	MultipartThreshold int64  `yaml:"multipart_threshold"`
	PartSize           int64  `yaml:"part_size"`
	Retries            int    `yaml:"retries"`
	RetryBackoffMs     int    `yaml:"retry_backoff_ms"`
	DryRun             bool   `yaml:"dry_run"`
	Checkpoint         string `yaml:"checkpoint"`
	SkipExisting       bool   `yaml:"skip_existing"`
	Resume             bool   `yaml:"resume"`
	ShowProgress       bool   `yaml:"show_progress"`
}

// Load loads configuration from file and command line flags
func Load(configFile string, flags *pflag.FlagSet) (*Config, error) {
	cfg := &Config{
		LogLevel: "info",
		Migration: Migration{
			Concurrency:        16,
			MultipartThreshold: 104857600, // 100MB
			PartSize:           67108864,  // 64MB
			Retries:            5,
			RetryBackoffMs:     500,
			Checkpoint:         "./checkpoint.db",
			SkipExisting:       true,
			ShowProgress:       true, // Default to true
		},
	}

	// Load from YAML file if provided
	if configFile != "" {
		if err := loadFromFile(cfg, configFile); err != nil {
			return nil, fmt.Errorf("failed to load config file: %w", err)
		}
	}

	// Override with command line flags
	if err := loadFromFlags(cfg, flags); err != nil {
		return nil, fmt.Errorf("failed to load flags: %w", err)
	}

	// Validate configuration
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

func loadFromFile(cfg *Config, filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(data, cfg)
}

func loadFromFlags(cfg *Config, flags *pflag.FlagSet) error {
	if flags.Changed("src-endpoint") {
		cfg.Source.Endpoint, _ = flags.GetString("src-endpoint")
	}
	if flags.Changed("src-access-key") {
		cfg.Source.AccessKey, _ = flags.GetString("src-access-key")
	}
	if flags.Changed("src-secret-key") {
		cfg.Source.SecretKey, _ = flags.GetString("src-secret-key")
	}
	if flags.Changed("src-secure") {
		cfg.Source.Secure, _ = flags.GetBool("src-secure")
	}

	if flags.Changed("dst-endpoint") {
		cfg.Target.Endpoint, _ = flags.GetString("dst-endpoint")
	}
	if flags.Changed("dst-access-key") {
		cfg.Target.AccessKey, _ = flags.GetString("dst-access-key")
	}
	if flags.Changed("dst-secret-key") {
		cfg.Target.SecretKey, _ = flags.GetString("dst-secret-key")
	}
	if flags.Changed("dst-secure") {
		cfg.Target.Secure, _ = flags.GetBool("dst-secure")
	}

	if flags.Changed("bucket") {
		cfg.Migration.Bucket, _ = flags.GetString("bucket")
	}
	if flags.Changed("prefix") {
		cfg.Migration.Prefix, _ = flags.GetString("prefix")
	}
	if flags.Changed("object") {
		cfg.Migration.Object, _ = flags.GetString("object")
	}
	if flags.Changed("concurrency") {
		cfg.Migration.Concurrency, _ = flags.GetInt("concurrency")
	}
	if flags.Changed("multipart-threshold") {
		cfg.Migration.MultipartThreshold, _ = flags.GetInt64("multipart-threshold")
	}
	if flags.Changed("part-size") {
		cfg.Migration.PartSize, _ = flags.GetInt64("part-size")
	}
	if flags.Changed("retries") {
		cfg.Migration.Retries, _ = flags.GetInt("retries")
	}
	if flags.Changed("retry-backoff-ms") {
		cfg.Migration.RetryBackoffMs, _ = flags.GetInt("retry-backoff-ms")
	}
	if flags.Changed("dry-run") {
		cfg.Migration.DryRun, _ = flags.GetBool("dry-run")
	}
	if flags.Changed("checkpoint") {
		cfg.Migration.Checkpoint, _ = flags.GetString("checkpoint")
	}
	if flags.Changed("skip-existing") {
		cfg.Migration.SkipExisting, _ = flags.GetBool("skip-existing")
	}
	if flags.Changed("resume") {
		cfg.Migration.Resume, _ = flags.GetBool("resume")
	}
	if flags.Changed("log-level") {
		cfg.LogLevel, _ = flags.GetString("log-level")
	}
	if flags.Changed("show-progress") {
		cfg.Migration.ShowProgress, _ = flags.GetBool("show-progress")
	}

	return nil
}

func (c *Config) validate() error {
	if c.Source.Endpoint == "" {
		return fmt.Errorf("source endpoint is required")
	}
	if c.Source.AccessKey == "" {
		return fmt.Errorf("source access key is required")
	}
	if c.Source.SecretKey == "" {
		return fmt.Errorf("source secret key is required")
	}

	if c.Target.Endpoint == "" {
		return fmt.Errorf("target endpoint is required")
	}
	if c.Target.AccessKey == "" {
		return fmt.Errorf("target access key is required")
	}
	if c.Target.SecretKey == "" {
		return fmt.Errorf("target secret key is required")
	}

	if c.Migration.Bucket == "" {
		return fmt.Errorf("bucket is required")
	}

	if c.Migration.Concurrency <= 0 {
		return fmt.Errorf("concurrency must be positive")
	}

	if c.Migration.PartSize < 5*1024*1024 { // 5MB minimum for S3
		return fmt.Errorf("part size must be at least 5MB")
	}

	return nil
}
