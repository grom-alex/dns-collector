package metrics

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"time"

	"dns-collector/internal/config"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
	dto "github.com/prometheus/client_model/go"
)

// InfluxDBClient pushes metrics to InfluxDB periodically.
type InfluxDBClient struct {
	cfg      config.InfluxDBConfig
	registry *Registry
	client   influxdb2.Client
	writeAPI api.WriteAPIBlocking
	stopCh   chan struct{}
	doneCh   chan struct{}
	started  bool
}

// NewInfluxDBClient creates a new InfluxDB client for pushing metrics.
func NewInfluxDBClient(cfg config.InfluxDBConfig, registry *Registry) *InfluxDBClient {
	opts := influxdb2.DefaultOptions()

	if cfg.InsecureSkipVerify {
		opts.SetHTTPClient(&http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		})
	}

	client := influxdb2.NewClientWithOptions(cfg.URL, cfg.Token, opts)
	writeAPI := client.WriteAPIBlocking(cfg.Organization, cfg.Bucket)

	return &InfluxDBClient{
		cfg:      cfg,
		registry: registry,
		client:   client,
		writeAPI: writeAPI,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
}

// Start begins the periodic push of metrics to InfluxDB.
func (c *InfluxDBClient) Start() error {
	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	health, err := c.client.Health(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to InfluxDB: %w", err)
	}
	if health.Status != "pass" {
		return fmt.Errorf("InfluxDB health check failed: %s", health.Status)
	}

	log.Printf("InfluxDB client connected to %s (org: %s, bucket: %s)",
		c.cfg.URL, c.cfg.Organization, c.cfg.Bucket)

	c.started = true
	go c.pushLoop()
	return nil
}

// Stop stops the InfluxDB client.
func (c *InfluxDBClient) Stop() error {
	log.Println("Stopping InfluxDB client...")

	// Only wait for doneCh if Start() was called
	if c.started {
		close(c.stopCh)
		<-c.doneCh
	}

	c.client.Close()
	log.Println("InfluxDB client stopped")
	return nil
}

func (c *InfluxDBClient) pushLoop() {
	defer close(c.doneCh)

	ticker := time.NewTicker(time.Duration(c.cfg.IntervalSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			if err := c.pushMetrics(); err != nil {
				log.Printf("Error pushing metrics to InfluxDB: %v", err)
			}
		}
	}
}

func (c *InfluxDBClient) pushMetrics() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Gather all metrics from registry
	mfs, err := c.registry.GetRegistry().Gather()
	if err != nil {
		return fmt.Errorf("failed to gather metrics: %w", err)
	}

	now := time.Now()
	var points []*write.Point

	for _, mf := range mfs {
		for _, m := range mf.GetMetric() {
			point := c.metricToPoint(mf.GetName(), mf.GetType(), m, now)
			if point != nil {
				points = append(points, point)
			}
		}
	}

	// Batch write all points - continue on partial failures
	var lastErr error
	failedCount := 0
	successCount := 0

	for _, point := range points {
		if err := c.writeAPI.WritePoint(ctx, point); err != nil {
			failedCount++
			lastErr = err
			log.Printf("Warning: Failed to write metric point %s: %v", point.Name(), err)
		} else {
			successCount++
		}
	}

	if failedCount > 0 {
		log.Printf("InfluxDB batch write completed: %d succeeded, %d failed", successCount, failedCount)
		// Return error only if all points failed
		if successCount == 0 {
			return fmt.Errorf("all %d points failed to write: %w", failedCount, lastErr)
		}
	}

	return nil
}

func (c *InfluxDBClient) metricToPoint(name string, mtype dto.MetricType, m *dto.Metric, t time.Time) *write.Point {
	tags := make(map[string]string)
	for _, lp := range m.GetLabel() {
		tags[lp.GetName()] = lp.GetValue()
	}

	fields := make(map[string]interface{})

	switch mtype {
	case dto.MetricType_COUNTER:
		fields["value"] = m.GetCounter().GetValue()
	case dto.MetricType_GAUGE:
		fields["value"] = m.GetGauge().GetValue()
	case dto.MetricType_HISTOGRAM:
		h := m.GetHistogram()
		fields["count"] = float64(h.GetSampleCount())
		fields["sum"] = h.GetSampleSum()
		for _, b := range h.GetBucket() {
			bucketTag := fmt.Sprintf("le_%v", b.GetUpperBound())
			fields[bucketTag] = float64(b.GetCumulativeCount())
		}
	case dto.MetricType_SUMMARY:
		s := m.GetSummary()
		fields["count"] = float64(s.GetSampleCount())
		fields["sum"] = s.GetSampleSum()
		for _, q := range s.GetQuantile() {
			quantileTag := fmt.Sprintf("quantile_%v", q.GetQuantile())
			fields[quantileTag] = q.GetValue()
		}
	default:
		return nil
	}

	return influxdb2.NewPoint(name, tags, fields, t)
}
