package main

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration for ailurophile, populated from environment variables.
type Config struct {
	PumaControlURL      string
	PollInterval        time.Duration
	CloudWatchNamespace string
	CloudWatchEnabled   bool
	AWSRegion           string
	ECSClusterName      string
	ECSServiceName      string
	OTLPEndpoint        string
	OTLPEnabled         bool
	HealthPort          int
	LogLevel            slog.Level
}

// LoadConfig reads configuration from environment variables and returns a Config.
func LoadConfig() (*Config, error) {
	cfg := &Config{
		PumaControlURL:      envOrDefault("PUMA_CONTROL_URL", "http://127.0.0.1:9293"),
		CloudWatchNamespace: envOrDefault("CLOUDWATCH_NAMESPACE", "Puma"),
		OTLPEndpoint:        envOrDefault("OTLP_ENDPOINT", "http://localhost:4318"),
		ECSClusterName:      os.Getenv("ECS_CLUSTER_NAME"),
		ECSServiceName:      os.Getenv("ECS_SERVICE_NAME"),
		AWSRegion:           os.Getenv("AWS_REGION"),
	}

	// Parse POLL_INTERVAL
	pollStr := envOrDefault("POLL_INTERVAL", "10s")
	dur, err := time.ParseDuration(pollStr)
	if err != nil {
		return nil, fmt.Errorf("invalid POLL_INTERVAL %q: %w", pollStr, err)
	}
	cfg.PollInterval = dur

	// Parse CLOUDWATCH_ENABLED
	cfg.CloudWatchEnabled, err = parseBoolEnv("CLOUDWATCH_ENABLED", true)
	if err != nil {
		return nil, err
	}

	// Parse OTLP_ENABLED
	cfg.OTLPEnabled, err = parseBoolEnv("OTLP_ENABLED", true)
	if err != nil {
		return nil, err
	}

	// Parse HEALTH_PORT
	healthPortStr := envOrDefault("HEALTH_PORT", "8090")
	cfg.HealthPort, err = strconv.Atoi(healthPortStr)
	if err != nil {
		return nil, fmt.Errorf("invalid HEALTH_PORT %q: %w", healthPortStr, err)
	}

	// Parse LOG_LEVEL
	cfg.LogLevel, err = parseLogLevel(envOrDefault("LOG_LEVEL", "info"))
	if err != nil {
		return nil, err
	}

	// Validate: AWS_REGION is required if CloudWatch is enabled
	if cfg.CloudWatchEnabled && cfg.AWSRegion == "" {
		return nil, fmt.Errorf("AWS_REGION is required when CLOUDWATCH_ENABLED=true")
	}

	return cfg, nil
}

// LogConfig logs the current configuration at info level.
func (c *Config) LogConfig(logger *slog.Logger) {
	logger.Info("configuration loaded",
		"puma_control_url", c.PumaControlURL,
		"poll_interval", c.PollInterval.String(),
		"cloudwatch_enabled", c.CloudWatchEnabled,
		"cloudwatch_namespace", c.CloudWatchNamespace,
		"aws_region", c.AWSRegion,
		"ecs_cluster_name", c.ECSClusterName,
		"ecs_service_name", c.ECSServiceName,
		"otlp_enabled", c.OTLPEnabled,
		"otlp_endpoint", c.OTLPEndpoint,
		"health_port", c.HealthPort,
		"log_level", c.LogLevel.String(),
	)
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func parseBoolEnv(key string, defaultVal bool) (bool, error) {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("invalid %s %q: %w", key, v, err)
	}
	return b, nil
}

func parseLogLevel(s string) (slog.Level, error) {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("invalid LOG_LEVEL %q (valid: debug, info, warn, error)", s)
	}
}
