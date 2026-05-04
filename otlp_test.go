package main

import (
	"context"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func newTestOTLPPublisher(t *testing.T, reader *sdkmetric.ManualReader) *OTLPPublisher {
	t.Helper()

	res, err := resource.New(context.Background(),
		resource.WithAttributes(semconv.ServiceName("ailurophile-test")),
	)
	if err != nil {
		t.Fatalf("creating resource: %v", err)
	}

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
		sdkmetric.WithResource(res),
	)

	meter := provider.Meter("ailurophile-test")

	backlogGauge, _ := meter.Int64Gauge("puma.backlog")
	runningGauge, _ := meter.Int64Gauge("puma.running")
	poolCapacityGauge, _ := meter.Int64Gauge("puma.pool_capacity")
	maxThreadsGauge, _ := meter.Int64Gauge("puma.max_threads")
	workerCountGauge, _ := meter.Int64Gauge("puma.worker_count")
	workerBacklog, _ := meter.Int64Gauge("puma.worker.backlog")

	return &OTLPPublisher{
		provider:          provider,
		meter:             meter,
		logger:            slog.Default(),
		backlogGauge:      backlogGauge,
		runningGauge:      runningGauge,
		poolCapacityGauge: poolCapacityGauge,
		maxThreadsGauge:   maxThreadsGauge,
		workerCountGauge:  workerCountGauge,
		workerBacklog:     workerBacklog,
		attrs: []attribute.KeyValue{
			attribute.String("ecs.cluster.name", "test-cluster"),
			attribute.String("ecs.service.name", "web"),
		},
	}
}

func collectMetrics(t *testing.T, reader *sdkmetric.ManualReader) map[string]metricdata.Metrics {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("collecting metrics: %v", err)
	}

	result := map[string]metricdata.Metrics{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			result[m.Name] = m
		}
	}
	return result
}

func getGaugeValue(t *testing.T, m metricdata.Metrics, attrs ...attribute.KeyValue) int64 {
	t.Helper()
	gauge, ok := m.Data.(metricdata.Gauge[int64])
	if !ok {
		t.Fatalf("metric %q is not an Int64Gauge", m.Name)
	}

	attrSet := attribute.NewSet(attrs...)
	for _, dp := range gauge.DataPoints {
		if dp.Attributes.Equals(&attrSet) {
			return dp.Value
		}
	}
	t.Fatalf("metric %q: no data point with matching attributes", m.Name)
	return 0
}

func TestOTLPPublish_SingleWorker(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	pub := newTestOTLPPublisher(t, reader)
	defer pub.Shutdown(context.Background())

	stats := &PumaStats{
		Workers:           []WorkerStats{{Index: 0, Backlog: 2, Running: 3, PoolCapacity: 2, MaxThreads: 5}},
		TotalBacklog:      2,
		TotalRunning:      3,
		TotalPoolCapacity: 2,
		TotalMaxThreads:   5,
		WorkerCount:       1,
		Booted:            true,
	}

	err := pub.Publish(context.Background(), stats)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	metrics := collectMetrics(t, reader)

	baseAttrs := []attribute.KeyValue{
		attribute.String("ecs.cluster.name", "test-cluster"),
		attribute.String("ecs.service.name", "web"),
	}

	if v := getGaugeValue(t, metrics["puma.backlog"], baseAttrs...); v != 2 {
		t.Errorf("puma.backlog = %d, want 2", v)
	}
	if v := getGaugeValue(t, metrics["puma.running"], baseAttrs...); v != 3 {
		t.Errorf("puma.running = %d, want 3", v)
	}
	if v := getGaugeValue(t, metrics["puma.pool_capacity"], baseAttrs...); v != 2 {
		t.Errorf("puma.pool_capacity = %d, want 2", v)
	}
	if v := getGaugeValue(t, metrics["puma.max_threads"], baseAttrs...); v != 5 {
		t.Errorf("puma.max_threads = %d, want 5", v)
	}
	if v := getGaugeValue(t, metrics["puma.worker_count"], baseAttrs...); v != 1 {
		t.Errorf("puma.worker_count = %d, want 1", v)
	}

	workerAttrs := append(baseAttrs, attribute.Int("worker_index", 0))
	if v := getGaugeValue(t, metrics["puma.worker.backlog"], workerAttrs...); v != 2 {
		t.Errorf("puma.worker.backlog = %d, want 2", v)
	}
}

func TestOTLPPublish_MultipleWorkers(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	pub := newTestOTLPPublisher(t, reader)
	defer pub.Shutdown(context.Background())

	stats := &PumaStats{
		Workers: []WorkerStats{
			{Index: 0, Backlog: 1, Running: 2, PoolCapacity: 1, MaxThreads: 3},
			{Index: 1, Backlog: 3, Running: 3, PoolCapacity: 0, MaxThreads: 3},
		},
		TotalBacklog:      4,
		TotalRunning:      5,
		TotalPoolCapacity: 1,
		TotalMaxThreads:   6,
		WorkerCount:       2,
		Booted:            true,
	}

	err := pub.Publish(context.Background(), stats)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	metrics := collectMetrics(t, reader)

	baseAttrs := []attribute.KeyValue{
		attribute.String("ecs.cluster.name", "test-cluster"),
		attribute.String("ecs.service.name", "web"),
	}

	if v := getGaugeValue(t, metrics["puma.backlog"], baseAttrs...); v != 4 {
		t.Errorf("puma.backlog = %d, want 4", v)
	}
	if v := getGaugeValue(t, metrics["puma.worker_count"], baseAttrs...); v != 2 {
		t.Errorf("puma.worker_count = %d, want 2", v)
	}

	worker0Attrs := append(baseAttrs, attribute.Int("worker_index", 0))
	if v := getGaugeValue(t, metrics["puma.worker.backlog"], worker0Attrs...); v != 1 {
		t.Errorf("worker 0 puma.worker.backlog = %d, want 1", v)
	}

	worker1Attrs := append(baseAttrs, attribute.Int("worker_index", 1))
	if v := getGaugeValue(t, metrics["puma.worker.backlog"], worker1Attrs...); v != 3 {
		t.Errorf("worker 1 puma.worker.backlog = %d, want 3", v)
	}
}

func TestOTLPPublish_NoAttributes(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	res, _ := resource.New(context.Background(),
		resource.WithAttributes(semconv.ServiceName("ailurophile-test")),
	)
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
		sdkmetric.WithResource(res),
	)
	meter := provider.Meter("ailurophile-test")

	backlogGauge, _ := meter.Int64Gauge("puma.backlog")
	runningGauge, _ := meter.Int64Gauge("puma.running")
	poolCapacityGauge, _ := meter.Int64Gauge("puma.pool_capacity")
	maxThreadsGauge, _ := meter.Int64Gauge("puma.max_threads")
	workerCountGauge, _ := meter.Int64Gauge("puma.worker_count")
	workerBacklog, _ := meter.Int64Gauge("puma.worker.backlog")

	pub := &OTLPPublisher{
		provider:          provider,
		meter:             meter,
		logger:            slog.Default(),
		backlogGauge:      backlogGauge,
		runningGauge:      runningGauge,
		poolCapacityGauge: poolCapacityGauge,
		maxThreadsGauge:   maxThreadsGauge,
		workerCountGauge:  workerCountGauge,
		workerBacklog:     workerBacklog,
		attrs:             nil,
	}
	defer pub.Shutdown(context.Background())

	stats := &PumaStats{
		Workers:      []WorkerStats{{Index: 0, Backlog: 5}},
		TotalBacklog: 5,
		WorkerCount:  1,
		Booted:       true,
	}

	err := pub.Publish(context.Background(), stats)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	metrics := collectMetrics(t, reader)
	if v := getGaugeValue(t, metrics["puma.backlog"]); v != 5 {
		t.Errorf("puma.backlog = %d, want 5", v)
	}
}

func TestOTLPPublish_UpdatesValues(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	pub := newTestOTLPPublisher(t, reader)
	defer pub.Shutdown(context.Background())

	baseAttrs := []attribute.KeyValue{
		attribute.String("ecs.cluster.name", "test-cluster"),
		attribute.String("ecs.service.name", "web"),
	}

	pub.Publish(context.Background(), &PumaStats{
		Workers:      []WorkerStats{{Index: 0, Backlog: 5}},
		TotalBacklog: 5,
		WorkerCount:  1,
		Booted:       true,
	})

	// Drain the first collection
	collectMetrics(t, reader)

	pub.Publish(context.Background(), &PumaStats{
		Workers:      []WorkerStats{{Index: 0, Backlog: 0}},
		TotalBacklog: 0,
		WorkerCount:  1,
		Booted:       true,
	})

	metrics := collectMetrics(t, reader)
	if v := getGaugeValue(t, metrics["puma.backlog"], baseAttrs...); v != 0 {
		t.Errorf("puma.backlog after update = %d, want 0", v)
	}
}
