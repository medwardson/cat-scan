package main

import (
	"log/slog"
	"testing"
)

func TestLoadConfig_Defaults(t *testing.T) {
	t.Setenv("CLOUDWATCH_ENABLED", "false")
	t.Setenv("OTLP_ENABLED", "false")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.PumaControlURL != "http://127.0.0.1:9293" {
		t.Errorf("PumaControlURL = %q, want default", cfg.PumaControlURL)
	}
	if cfg.PollInterval.String() != "10s" {
		t.Errorf("PollInterval = %s, want 10s", cfg.PollInterval)
	}
	if cfg.CloudWatchNamespace != "Puma" {
		t.Errorf("CloudWatchNamespace = %q, want Puma", cfg.CloudWatchNamespace)
	}
	if cfg.OTLPEndpoint != "http://localhost:4318" {
		t.Errorf("OTLPEndpoint = %q, want default", cfg.OTLPEndpoint)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want Info", cfg.LogLevel)
	}
}

func TestLoadConfig_CustomValues(t *testing.T) {
	t.Setenv("PUMA_CONTROL_URL", "http://localhost:9999")
	t.Setenv("POLL_INTERVAL", "30s")
	t.Setenv("CLOUDWATCH_ENABLED", "false")
	t.Setenv("CLOUDWATCH_NAMESPACE", "MyApp")
	t.Setenv("OTLP_ENABLED", "false")
	t.Setenv("OTLP_ENDPOINT", "http://otel:4318")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("ECS_CLUSTER_NAME", "my-cluster")
	t.Setenv("ECS_SERVICE_NAME", "web")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.PumaControlURL != "http://localhost:9999" {
		t.Errorf("PumaControlURL = %q", cfg.PumaControlURL)
	}
	if cfg.PollInterval.String() != "30s" {
		t.Errorf("PollInterval = %s", cfg.PollInterval)
	}
	if cfg.CloudWatchNamespace != "MyApp" {
		t.Errorf("CloudWatchNamespace = %q", cfg.CloudWatchNamespace)
	}
	if cfg.OTLPEndpoint != "http://otel:4318" {
		t.Errorf("OTLPEndpoint = %q", cfg.OTLPEndpoint)
	}
	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("LogLevel = %v", cfg.LogLevel)
	}
	if cfg.ECSClusterName != "my-cluster" {
		t.Errorf("ECSClusterName = %q", cfg.ECSClusterName)
	}
	if cfg.ECSServiceName != "web" {
		t.Errorf("ECSServiceName = %q", cfg.ECSServiceName)
	}
}

func TestLoadConfig_CloudWatchRequiresRegion(t *testing.T) {
	t.Setenv("CLOUDWATCH_ENABLED", "true")
	t.Setenv("AWS_REGION", "")

	_, err := LoadConfig()
	if err == nil {
		t.Error("expected error when CloudWatch enabled without AWS_REGION")
	}
}

func TestLoadConfig_InvalidPollInterval(t *testing.T) {
	t.Setenv("POLL_INTERVAL", "not-a-duration")
	t.Setenv("CLOUDWATCH_ENABLED", "false")

	_, err := LoadConfig()
	if err == nil {
		t.Error("expected error for invalid POLL_INTERVAL")
	}
}

func TestLoadConfig_InvalidBool(t *testing.T) {
	t.Setenv("CLOUDWATCH_ENABLED", "maybe")

	_, err := LoadConfig()
	if err == nil {
		t.Error("expected error for invalid CLOUDWATCH_ENABLED")
	}
}

func TestLoadConfig_InvalidLogLevel(t *testing.T) {
	t.Setenv("LOG_LEVEL", "verbose")
	t.Setenv("CLOUDWATCH_ENABLED", "false")

	_, err := LoadConfig()
	if err == nil {
		t.Error("expected error for invalid LOG_LEVEL")
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
	}

	for _, tt := range tests {
		got, err := parseLogLevel(tt.input)
		if err != nil {
			t.Errorf("parseLogLevel(%q) error: %v", tt.input, err)
		}
		if got != tt.want {
			t.Errorf("parseLogLevel(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
