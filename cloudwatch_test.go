package main

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

type mockCWClient struct {
	input *cloudwatch.PutMetricDataInput
	err   error
}

func (m *mockCWClient) PutMetricData(ctx context.Context, params *cloudwatch.PutMetricDataInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.PutMetricDataOutput, error) {
	m.input = params
	return &cloudwatch.PutMetricDataOutput{}, m.err
}

func newTestPublisher(mock *mockCWClient) *CloudWatchPublisher {
	return &CloudWatchPublisher{
		client:    mock,
		namespace: "Puma",
		dims: []types.Dimension{
			{Name: aws.String("ClusterName"), Value: aws.String("test-cluster")},
			{Name: aws.String("ServiceName"), Value: aws.String("web")},
		},
		logger: slog.Default(),
	}
}

func TestCloudWatchPublish_SingleWorker(t *testing.T) {
	mock := &mockCWClient{}
	pub := newTestPublisher(mock)

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

	if mock.input == nil {
		t.Fatal("PutMetricData was not called")
	}
	if *mock.input.Namespace != "Puma" {
		t.Errorf("Namespace = %q, want Puma", *mock.input.Namespace)
	}

	// 5 aggregate metrics + 1 per-worker backlog = 6
	if len(mock.input.MetricData) != 6 {
		t.Errorf("MetricData count = %d, want 6", len(mock.input.MetricData))
	}

	metricsByName := map[string]types.MetricDatum{}
	for _, d := range mock.input.MetricData {
		metricsByName[*d.MetricName] = d
	}

	assertMetricValue(t, metricsByName, "Backlog", 2)
	assertMetricValue(t, metricsByName, "Running", 3)
	assertMetricValue(t, metricsByName, "PoolCapacity", 2)
	assertMetricValue(t, metricsByName, "MaxThreads", 5)
	assertMetricValue(t, metricsByName, "WorkerCount", 1)
	assertMetricValue(t, metricsByName, "WorkerBacklog", 2)
}

func TestCloudWatchPublish_MultipleWorkers(t *testing.T) {
	mock := &mockCWClient{}
	pub := newTestPublisher(mock)

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

	// 5 aggregate + 2 per-worker backlog = 7
	if len(mock.input.MetricData) != 7 {
		t.Errorf("MetricData count = %d, want 7", len(mock.input.MetricData))
	}

	// Verify per-worker backlogs have WorkerIndex dimension
	workerBacklogs := 0
	for _, d := range mock.input.MetricData {
		if *d.MetricName == "WorkerBacklog" {
			workerBacklogs++
			hasWorkerIndex := false
			for _, dim := range d.Dimensions {
				if *dim.Name == "WorkerIndex" {
					hasWorkerIndex = true
				}
			}
			if !hasWorkerIndex {
				t.Error("WorkerBacklog missing WorkerIndex dimension")
			}
		}
	}
	if workerBacklogs != 2 {
		t.Errorf("WorkerBacklog count = %d, want 2", workerBacklogs)
	}
}

func TestCloudWatchPublish_AWSError(t *testing.T) {
	mock := &mockCWClient{err: errors.New("throttling")}
	pub := newTestPublisher(mock)

	stats := &PumaStats{
		Workers:     []WorkerStats{{Index: 0}},
		WorkerCount: 1,
		Booted:      true,
	}

	err := pub.Publish(context.Background(), stats)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestCloudWatchPublish_NoDimensions(t *testing.T) {
	mock := &mockCWClient{}
	pub := &CloudWatchPublisher{
		client:    mock,
		namespace: "Puma",
		dims:      []types.Dimension{},
		logger:    slog.Default(),
	}

	stats := &PumaStats{
		Workers:      []WorkerStats{{Index: 0, Backlog: 1}},
		TotalBacklog: 1,
		WorkerCount:  1,
		Booted:       true,
	}

	err := pub.Publish(context.Background(), stats)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, d := range mock.input.MetricData {
		if *d.MetricName != "WorkerBacklog" && len(d.Dimensions) != 0 {
			t.Errorf("metric %q has %d dimensions, want 0", *d.MetricName, len(d.Dimensions))
		}
	}
}

func assertMetricValue(t *testing.T, metrics map[string]types.MetricDatum, name string, want float64) {
	t.Helper()
	d, ok := metrics[name]
	if !ok {
		t.Errorf("metric %q not found", name)
		return
	}
	if *d.Value != want {
		t.Errorf("%s = %v, want %v", name, *d.Value, want)
	}
}
