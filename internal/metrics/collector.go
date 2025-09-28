package metrics

import (
	"net/http"
	"time"

	"minio2rustfs/internal/progress"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Collector collects and exposes metrics
type Collector struct {
	objectsTotal    *prometheus.CounterVec
	bytesTotal      prometheus.Counter
	inflightWorkers prometheus.Gauge
	duration        prometheus.Histogram
	progressTracker *progress.Tracker // Add progress tracker
}

// New creates a new metrics collector
func New() *Collector {
	c := &Collector{
		objectsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "migrate_objects_total",
				Help: "Total number of objects processed",
			},
			[]string{"status"},
		),
		bytesTotal: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "migrate_bytes_total",
				Help: "Total bytes migrated",
			},
		),
		inflightWorkers: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "migrate_inflight_workers",
				Help: "Number of workers currently processing",
			},
		),
		duration: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "migrate_object_duration_seconds",
				Help:    "Time taken to migrate an object",
				Buckets: prometheus.DefBuckets,
			},
		),
		progressTracker: progress.NewTracker(), // Initialize progress tracker
	}

	// Register metrics
	prometheus.MustRegister(c.objectsTotal)
	prometheus.MustRegister(c.bytesTotal)
	prometheus.MustRegister(c.inflightWorkers)
	prometheus.MustRegister(c.duration)

	return c
}

// IncSuccess increments successful object counter
func (c *Collector) IncSuccess() {
	c.objectsTotal.WithLabelValues("success").Inc()
}

// IncSuccessWithBytes increments successful object counter and updates progress
func (c *Collector) IncSuccessWithBytes(bytes int64) {
	c.objectsTotal.WithLabelValues("success").Inc()
	c.progressTracker.AddSuccess(bytes)
}

// IncFailed increments failed object counter
func (c *Collector) IncFailed() {
	c.objectsTotal.WithLabelValues("failed").Inc()
	c.progressTracker.AddFailed() // Update progress tracker
}

// IncSkipped increments skipped object counter
func (c *Collector) IncSkipped() {
	c.objectsTotal.WithLabelValues("skipped").Inc()
}

// IncSkippedWithBytes increments skipped object counter and updates progress
func (c *Collector) IncSkippedWithBytes(bytes int64) {
	c.objectsTotal.WithLabelValues("skipped").Inc()
	c.progressTracker.AddSkipped(bytes)
}

// AddBytes adds to total bytes migrated
func (c *Collector) AddBytes(bytes int64) {
	c.bytesTotal.Add(float64(bytes))
}

// SetInflightWorkers sets the number of inflight workers
func (c *Collector) SetInflightWorkers(count int) {
	c.inflightWorkers.Set(float64(count))
}

// ObserveDuration observes migration duration
func (c *Collector) ObserveDuration(duration time.Duration) {
	c.duration.Observe(duration.Seconds())
}

// StartServer starts the metrics HTTP server
func (c *Collector) StartServer(addr string) error {
	http.Handle("/metrics", promhttp.Handler())
	return http.ListenAndServe(addr, nil)
}

// GetProgressTracker returns the progress tracker
func (c *Collector) GetProgressTracker() *progress.Tracker {
	return c.progressTracker
}

// SetTotalCounts sets the total counts for progress tracking
func (c *Collector) SetTotalCounts(objects, bytes int64) {
	c.progressTracker.SetTotal(objects, bytes)
}
