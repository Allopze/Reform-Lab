package observability

import "github.com/prometheus/client_golang/prometheus"

// Metrics holds all application-level Prometheus metrics.
type Metrics struct {
	UploadsTotal   *prometheus.CounterVec
	JobsTotal      *prometheus.CounterVec
	JobDuration    *prometheus.HistogramVec
	ArtifactsTotal prometheus.Counter
}

// NewMetrics registers and returns application metrics.
func NewMetrics() *Metrics {
	m := &Metrics{
		UploadsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "reform_uploads_total",
				Help: "Total uploaded files by format family",
			},
			[]string{"family"},
		),
		JobsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "reform_jobs_total",
				Help: "Total jobs by capability and final status",
			},
			[]string{"capability", "status"},
		),
		JobDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "reform_job_duration_seconds",
				Help:    "Job execution duration in seconds by capability",
				Buckets: prometheus.ExponentialBuckets(1, 2, 10),
			},
			[]string{"capability"},
		),
		ArtifactsTotal: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "reform_artifacts_total",
				Help: "Total artifacts produced",
			},
		),
	}

	prometheus.MustRegister(m.UploadsTotal, m.JobsTotal, m.JobDuration, m.ArtifactsTotal)
	return m
}
