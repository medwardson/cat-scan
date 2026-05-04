package main

import (
	"context"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	otelmetric "go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// OTLPPublisher publishes Puma metrics via OTLP HTTP.
type OTLPPublisher struct {
	provider *sdkmetric.MeterProvider
	meter    otelmetric.Meter
	logger   *slog.Logger

	// Instruments
	backlogGauge      otelmetric.Int64Gauge
	runningGauge      otelmetric.Int64Gauge
	poolCapacityGauge otelmetric.Int64Gauge
	maxThreadsGauge   otelmetric.Int64Gauge
	workerCountGauge  otelmetric.Int64Gauge
	workerBacklog     otelmetric.Int64Gauge

	// Attributes
	attrs []attribute.KeyValue
}

// NewOTLPPublisher creates a new OTLP metrics publisher.
func NewOTLPPublisher(ctx context.Context, cfg *Config, logger *slog.Logger) (*OTLPPublisher, error) {
	exporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithEndpointURL(cfg.OTLPEndpoint),
		otlpmetrichttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("creating OTLP exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("ailurophile"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("creating OTLP resource: %w", err)
	}

	// Match export interval to poll interval — gauges only retain the last
	// recorded value, so a slower export would silently drop intermediate samples.
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter,
			sdkmetric.WithInterval(cfg.PollInterval),
		)),
		sdkmetric.WithResource(res),
	)

	meter := provider.Meter("ailurophile")

	backlogGauge, err := meter.Int64Gauge("puma.backlog")
	if err != nil {
		return nil, fmt.Errorf("creating puma.backlog gauge: %w", err)
	}

	runningGauge, err := meter.Int64Gauge("puma.running")
	if err != nil {
		return nil, fmt.Errorf("creating puma.running gauge: %w", err)
	}

	poolCapacityGauge, err := meter.Int64Gauge("puma.pool_capacity")
	if err != nil {
		return nil, fmt.Errorf("creating puma.pool_capacity gauge: %w", err)
	}

	maxThreadsGauge, err := meter.Int64Gauge("puma.max_threads")
	if err != nil {
		return nil, fmt.Errorf("creating puma.max_threads gauge: %w", err)
	}

	workerCountGauge, err := meter.Int64Gauge("puma.worker_count")
	if err != nil {
		return nil, fmt.Errorf("creating puma.worker_count gauge: %w", err)
	}

	workerBacklog, err := meter.Int64Gauge("puma.worker.backlog")
	if err != nil {
		return nil, fmt.Errorf("creating puma.worker.backlog gauge: %w", err)
	}

	var attrs []attribute.KeyValue
	if cfg.ECSClusterName != "" {
		attrs = append(attrs, attribute.String("ecs.cluster.name", cfg.ECSClusterName))
	}
	if cfg.ECSServiceName != "" {
		attrs = append(attrs, attribute.String("ecs.service.name", cfg.ECSServiceName))
	}

	return &OTLPPublisher{
		provider:          provider,
		meter:             meter,
		logger:            logger,
		backlogGauge:      backlogGauge,
		runningGauge:      runningGauge,
		poolCapacityGauge: poolCapacityGauge,
		maxThreadsGauge:   maxThreadsGauge,
		workerCountGauge:  workerCountGauge,
		workerBacklog:     workerBacklog,
		attrs:             attrs,
	}, nil
}

// Publish records Puma stats as OTLP metrics.
func (p *OTLPPublisher) Publish(ctx context.Context, stats *PumaStats) error {
	opts := otelmetric.WithAttributes(p.attrs...)

	p.backlogGauge.Record(ctx, int64(stats.TotalBacklog), opts)
	p.runningGauge.Record(ctx, int64(stats.TotalRunning), opts)
	p.poolCapacityGauge.Record(ctx, int64(stats.TotalPoolCapacity), opts)
	p.maxThreadsGauge.Record(ctx, int64(stats.TotalMaxThreads), opts)
	p.workerCountGauge.Record(ctx, int64(stats.WorkerCount), opts)

	for _, w := range stats.Workers {
		workerAttrs := make([]attribute.KeyValue, len(p.attrs), len(p.attrs)+1)
		copy(workerAttrs, p.attrs)
		workerAttrs = append(workerAttrs, attribute.Int("worker_index", w.Index))
		p.workerBacklog.Record(ctx, int64(w.Backlog), otelmetric.WithAttributes(workerAttrs...))
	}

	p.logger.Debug("recorded OTLP metrics")
	return nil
}

// Shutdown flushes and shuts down the OTLP meter provider.
func (p *OTLPPublisher) Shutdown(ctx context.Context) error {
	return p.provider.Shutdown(ctx)
}
