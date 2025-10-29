package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus metrics for the protocol bridge
type Metrics struct {
	// Counter metrics
	MessagesReceivedTotal    *prometheus.CounterVec
	MessagesTransformedTotal *prometheus.CounterVec
	MessagesSentTotal        *prometheus.CounterVec
	MessagesFailedTotal      *prometheus.CounterVec
	MessagesDLQTotal         *prometheus.CounterVec

	// Gauge metrics
	ConnectionsActive   *prometheus.GaugeVec
	BufferSize          *prometheus.GaugeVec
	GoroutinesActive    prometheus.Gauge
	DLQSize             *prometheus.GaugeVec
	DiskFreeBytes       prometheus.Gauge
	DiskUsedPercent     prometheus.Gauge
	CertExpirationDays  *prometheus.GaugeVec
	ServerRestartsTotal *prometheus.CounterVec

	// Histogram metrics
	MessageProcessingDuration *prometheus.HistogramVec
	TransformationDuration    *prometheus.HistogramVec
	OutputPublishDuration     *prometheus.HistogramVec
}

// New creates a new Metrics instance with all metric definitions
func New() *Metrics {
	return &Metrics{
		// Counter metrics
		MessagesReceivedTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "messages_received_total",
				Help: "Total number of messages received",
			},
			[]string{"input", "pipeline"},
		),
		MessagesTransformedTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "messages_transformed_total",
				Help: "Total number of messages transformed",
			},
			[]string{"transformer", "pipeline"},
		),
		MessagesSentTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "messages_sent_total",
				Help: "Total number of messages sent",
			},
			[]string{"output", "pipeline"},
		),
		MessagesFailedTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "messages_failed_total",
				Help: "Total number of failed messages",
			},
			[]string{"output", "pipeline", "reason"},
		),
		MessagesDLQTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "messages_dlq_total",
				Help: "Total number of messages sent to DLQ",
			},
			[]string{"pipeline"},
		),

		// Gauge metrics
		ConnectionsActive: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "connections_active",
				Help: "Number of active connections",
			},
			[]string{"type", "name"},
		),
		BufferSize: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "buffer_size",
				Help: "Current message buffer size",
			},
			[]string{"pipeline"},
		),
		GoroutinesActive: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "goroutines_active",
				Help: "Number of active goroutines",
			},
		),
		DLQSize: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "dlq_size_bytes",
				Help: "Current size of dead letter queue in bytes",
			},
			[]string{"pipeline"},
		),
		DiskFreeBytes: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "disk_free_bytes",
				Help: "Free disk space in bytes",
			},
		),
		DiskUsedPercent: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "disk_used_percent",
				Help: "Percentage of disk space used",
			},
		),
		CertExpirationDays: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "cert_expiration_days",
				Help: "Days until certificate expiration",
			},
			[]string{"cert_path"},
		),
		ServerRestartsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "server_restarts_total",
				Help: "Total number of server restarts",
			},
			[]string{"server_name"},
		),

		// Histogram metrics
		MessageProcessingDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "message_processing_duration_seconds",
				Help:    "Message processing duration in seconds",
				Buckets: prometheus.ExponentialBuckets(0.001, 2, 10), // 1ms to 512ms
			},
			[]string{"pipeline"},
		),
		TransformationDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "transformation_duration_seconds",
				Help:    "Message transformation duration in seconds",
				Buckets: prometheus.ExponentialBuckets(0.0001, 2, 10), // 0.1ms to 51.2ms
			},
			[]string{"transformer"},
		),
		OutputPublishDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "output_publish_duration_seconds",
				Help:    "Output message publishing duration in seconds",
				Buckets: prometheus.ExponentialBuckets(0.001, 2, 10), // 1ms to 512ms
			},
			[]string{"output"},
		),
	}
}

// RecordMessageReceived records a message received
func (m *Metrics) RecordMessageReceived(input, pipeline string) {
	m.MessagesReceivedTotal.WithLabelValues(input, pipeline).Inc()
}

// RecordMessageTransformed records a message transformed
func (m *Metrics) RecordMessageTransformed(transformer, pipeline string) {
	m.MessagesTransformedTotal.WithLabelValues(transformer, pipeline).Inc()
}

// RecordMessageSent records a message sent
func (m *Metrics) RecordMessageSent(output, pipeline string) {
	m.MessagesSentTotal.WithLabelValues(output, pipeline).Inc()
}

// RecordMessageFailed records a failed message
func (m *Metrics) RecordMessageFailed(output, pipeline, reason string) {
	m.MessagesFailedTotal.WithLabelValues(output, pipeline, reason).Inc()
}

// RecordMessageDLQ records a message sent to DLQ
func (m *Metrics) RecordMessageDLQ(pipeline string) {
	m.MessagesDLQTotal.WithLabelValues(pipeline).Inc()
}

// SetConnectionActive sets connection active status
func (m *Metrics) SetConnectionActive(connType, name string, active bool) {
	if active {
		m.ConnectionsActive.WithLabelValues(connType, name).Set(1)
	} else {
		m.ConnectionsActive.WithLabelValues(connType, name).Set(0)
	}
}

// SetBufferSize sets the buffer size for a pipeline
func (m *Metrics) SetBufferSize(pipeline string, size float64) {
	m.BufferSize.WithLabelValues(pipeline).Set(size)
}

// UpdateGoroutines updates the goroutine count
func (m *Metrics) UpdateGoroutines(count float64) {
	m.GoroutinesActive.Set(count)
}

// RecordProcessingDuration records processing duration
func (m *Metrics) RecordProcessingDuration(pipeline string, duration float64) {
	m.MessageProcessingDuration.WithLabelValues(pipeline).Observe(duration)
}

// RecordTransformationDuration records transformation duration
func (m *Metrics) RecordTransformationDuration(transformer string, duration float64) {
	m.TransformationDuration.WithLabelValues(transformer).Observe(duration)
}

// RecordPublishDuration records publish duration
func (m *Metrics) RecordPublishDuration(output string, duration float64) {
	m.OutputPublishDuration.WithLabelValues(output).Observe(duration)
}

// SetDLQSize sets the DLQ size for a pipeline
func (m *Metrics) SetDLQSize(pipeline string, size float64) {
	m.DLQSize.WithLabelValues(pipeline).Set(size)
}

// SetDiskFreeBytes sets the free disk space
func (m *Metrics) SetDiskFreeBytes(bytes float64) {
	m.DiskFreeBytes.Set(bytes)
}

// SetDiskUsedPercent sets the disk usage percentage
func (m *Metrics) SetDiskUsedPercent(percent float64) {
	m.DiskUsedPercent.Set(percent)
}

// SetCertExpirationDays sets the certificate expiration days
func (m *Metrics) SetCertExpirationDays(certPath string, days float64) {
	m.CertExpirationDays.WithLabelValues(certPath).Set(days)
}

// RecordServerRestart records a server restart
func (m *Metrics) RecordServerRestart(serverName string) {
	m.ServerRestartsTotal.WithLabelValues(serverName).Inc()
}
