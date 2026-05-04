package main

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

type cloudWatchClient interface {
	PutMetricData(ctx context.Context, params *cloudwatch.PutMetricDataInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.PutMetricDataOutput, error)
}

// CloudWatchPublisher publishes Puma metrics to AWS CloudWatch.
type CloudWatchPublisher struct {
	client    cloudWatchClient
	namespace string
	dims      []types.Dimension
	logger    *slog.Logger
}

// NewCloudWatchPublisher creates a new CloudWatch publisher.
func NewCloudWatchPublisher(ctx context.Context, cfg *Config, logger *slog.Logger) (*CloudWatchPublisher, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.AWSRegion))
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}

	dims := []types.Dimension{}
	if cfg.ECSClusterName != "" {
		dims = append(dims, types.Dimension{
			Name:  aws.String("ClusterName"),
			Value: aws.String(cfg.ECSClusterName),
		})
	}
	if cfg.ECSServiceName != "" {
		dims = append(dims, types.Dimension{
			Name:  aws.String("ServiceName"),
			Value: aws.String(cfg.ECSServiceName),
		})
	}

	return &CloudWatchPublisher{
		client:    cloudwatch.NewFromConfig(awsCfg),
		namespace: cfg.CloudWatchNamespace,
		dims:      dims,
		logger:    logger,
	}, nil
}

// Publish sends Puma stats to CloudWatch as a single PutMetricData call.
func (p *CloudWatchPublisher) Publish(ctx context.Context, stats *PumaStats) error {
	metricData := []types.MetricDatum{
		p.newDatum("Backlog", float64(stats.TotalBacklog), p.dims),
		p.newDatum("Running", float64(stats.TotalRunning), p.dims),
		p.newDatum("PoolCapacity", float64(stats.TotalPoolCapacity), p.dims),
		p.newDatum("MaxThreads", float64(stats.TotalMaxThreads), p.dims),
		p.newDatum("WorkerCount", float64(stats.WorkerCount), p.dims),
	}

	// Per-worker backlog metrics
	for _, w := range stats.Workers {
		workerDims := make([]types.Dimension, len(p.dims), len(p.dims)+1)
		copy(workerDims, p.dims)
		workerDims = append(workerDims, types.Dimension{
			Name:  aws.String("WorkerIndex"),
			Value: aws.String(strconv.Itoa(w.Index)),
		})
		metricData = append(metricData, p.newDatum("WorkerBacklog", float64(w.Backlog), workerDims))
	}

	_, err := p.client.PutMetricData(ctx, &cloudwatch.PutMetricDataInput{
		Namespace:  aws.String(p.namespace),
		MetricData: metricData,
	})
	if err != nil {
		return fmt.Errorf("putting CloudWatch metric data: %w", err)
	}

	p.logger.Debug("published metrics to CloudWatch", "metric_count", len(metricData))
	return nil
}

func (p *CloudWatchPublisher) newDatum(name string, value float64, dims []types.Dimension) types.MetricDatum {
	return types.MetricDatum{
		MetricName: aws.String(name),
		Value:      aws.Float64(value),
		Unit:       types.StandardUnitCount,
		Dimensions: dims,
	}
}
