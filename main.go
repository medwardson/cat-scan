package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// Load configuration
	cfg, err := LoadConfig()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Set up structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))
	slog.SetDefault(logger)

	logger.Info("ailurophile starting")
	cfg.LogConfig(logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize Puma client
	pumaClient := NewPumaClient(cfg.PumaControlURL)

	// Initialize publishers
	var cwPublisher *CloudWatchPublisher
	if cfg.CloudWatchEnabled {
		cwPublisher, err = NewCloudWatchPublisher(ctx, cfg, logger)
		if err != nil {
			logger.Error("failed to create CloudWatch publisher", "error", err)
			os.Exit(1)
		}
		logger.Info("CloudWatch publisher initialized")
	}

	var otlpPublisher *OTLPPublisher
	if cfg.OTLPEnabled {
		otlpPublisher, err = NewOTLPPublisher(ctx, cfg, logger)
		if err != nil {
			logger.Error("failed to create OTLP publisher", "error", err)
			os.Exit(1)
		}
		logger.Info("OTLP publisher initialized")
	}

	// Start health server
	health := NewHealthServer(logger)
	httpSrv := health.ListenAndServe(cfg.HealthPort)

	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	// Start poll loop
	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	logger.Info("starting poll loop", "interval", cfg.PollInterval.String())

	poll := func() {
		stats, err := pumaClient.FetchStats()
		if err != nil {
			logger.Warn("failed to fetch puma stats", "error", err)
			health.SetUnhealthy(err.Error())
			return
		}

		logger.Debug("fetched puma stats",
			"worker_count", stats.WorkerCount,
			"total_backlog", stats.TotalBacklog,
			"total_running", stats.TotalRunning,
			"total_pool_capacity", stats.TotalPoolCapacity,
			"booted", stats.Booted,
		)

		if cwPublisher != nil {
			if err := cwPublisher.Publish(ctx, stats); err != nil {
				logger.Warn("failed to publish to CloudWatch", "error", err)
			}
		}

		if otlpPublisher != nil {
			if err := otlpPublisher.Publish(ctx, stats); err != nil {
				logger.Warn("failed to publish to OTLP", "error", err)
			}
		}

		health.SetHealthy()
	}

	// Run first poll immediately
	poll()

	for {
		select {
		case <-ticker.C:
			poll()
		case sig := <-sigCh:
			logger.Info("received signal, shutting down", "signal", sig.String())

			// Stop accepting new work
			ticker.Stop()

			// Shutdown health server
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()

			if err := httpSrv.Shutdown(shutdownCtx); err != nil {
				logger.Error("health server shutdown error", "error", err)
			}

			// Flush OTLP metrics
			if otlpPublisher != nil {
				if err := otlpPublisher.Shutdown(shutdownCtx); err != nil {
					logger.Error("OTLP shutdown error", "error", err)
				}
			}

			logger.Info("ailurophile stopped")
			return
		}
	}
}
