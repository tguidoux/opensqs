package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Collector holds all Prometheus metric collectors for the OpenSQS server.
// Each metric is registered with the default Prometheus registry on creation.
type Collector struct {
	// messagesSentTotal counts total messages sent to a queue.
	messagesSentTotal *prometheus.CounterVec
	// messagesReceivedTotal counts total messages received from a queue.
	messagesReceivedTotal *prometheus.CounterVec
	// messagesDeletedTotal counts total messages deleted from a queue.
	messagesDeletedTotal *prometheus.CounterVec
	// queueSize is a gauge for the approximate number of messages in a queue.
	queueSize *prometheus.GaugeVec
	// apiRequestsTotal counts total API requests by action and protocol.
	apiRequestsTotal *prometheus.CounterVec
	// apiRequestDuration tracks API request latency by action and protocol.
	apiRequestDuration *prometheus.HistogramVec
	// moveTaskMessagesMoved counts messages moved by move tasks.
	moveTaskMessagesMoved *prometheus.CounterVec
	// moveTaskActive tracks the number of active move tasks.
	moveTaskActive prometheus.Gauge
}

// NewCollector creates and registers all Prometheus metrics with the default
// registry. Panics if registration fails (duplicate registration is a programming error).
func NewCollector() *Collector {
	return NewCollectorWithRegistry(prometheus.DefaultRegisterer)
}

// NewCollectorWithRegistry creates and registers all Prometheus metrics with
// the provided registerer. Use this in tests with a non-default registry to
// avoid duplicate-registration panics.
func NewCollectorWithRegistry(reg prometheus.Registerer) *Collector {
	c := &Collector{
		messagesSentTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "opensqs_messages_sent_total",
			Help: "Total number of messages sent to a queue.",
		}, []string{"queue"}),
		messagesReceivedTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "opensqs_messages_received_total",
			Help: "Total number of messages received from a queue.",
		}, []string{"queue"}),
		messagesDeletedTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "opensqs_messages_deleted_total",
			Help: "Total number of messages deleted from a queue.",
		}, []string{"queue"}),
		queueSize: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "opensqs_queue_size",
			Help: "Approximate number of messages in a queue.",
		}, []string{"queue", "type"}),
		apiRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "opensqs_api_requests_total",
			Help: "Total number of API requests by action and protocol.",
		}, []string{"action", "protocol"}),
		apiRequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "opensqs_api_request_duration_seconds",
			Help:    "API request latency in seconds by action and protocol.",
			Buckets: prometheus.DefBuckets,
		}, []string{"action", "protocol"}),
		moveTaskMessagesMoved: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "opensqs_move_task_messages_moved_total",
			Help: "Total number of messages moved by move tasks.",
		}, []string{"source_arn", "destination_arn"}),
		moveTaskActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "opensqs_move_task_active",
			Help: "Number of active message move tasks.",
		}),
	}

	reg.MustRegister(
		c.messagesSentTotal,
		c.messagesReceivedTotal,
		c.messagesDeletedTotal,
		c.queueSize,
		c.apiRequestsTotal,
		c.apiRequestDuration,
		c.moveTaskMessagesMoved,
		c.moveTaskActive,
	)

	return c
}

// IncMessagesSent increments the sent messages counter for the given queue.
func (c *Collector) IncMessagesSent(queueName string) {
	c.messagesSentTotal.WithLabelValues(queueName).Inc()
}

// IncMessagesReceived increments the received messages counter for the given queue.
func (c *Collector) IncMessagesReceived(queueName string, count int) {
	c.messagesReceivedTotal.WithLabelValues(queueName).Add(float64(count))
}

// IncMessagesDeleted increments the deleted messages counter for the given queue.
func (c *Collector) IncMessagesDeleted(queueName string) {
	c.messagesDeletedTotal.WithLabelValues(queueName).Inc()
}

// SetQueueSize sets the queue size gauge for the given queue and type.
// queueType is "available", "inflight", or "delayed".
func (c *Collector) SetQueueSize(queueName, queueType string, size int) {
	c.queueSize.WithLabelValues(queueName, queueType).Set(float64(size))
}

// IncAPIRequest increments the API request counter for the given action and protocol.
func (c *Collector) IncAPIRequest(action, protocol string) {
	c.apiRequestsTotal.WithLabelValues(action, protocol).Inc()
}

// ObserveAPIRequestDuration records the API request duration for the given action and protocol.
func (c *Collector) ObserveAPIRequestDuration(action, protocol string, durationSeconds float64) {
	c.apiRequestDuration.WithLabelValues(action, protocol).Observe(durationSeconds)
}

// IncMoveTaskMessagesMoved increments the moved messages counter for a move task.
func (c *Collector) IncMoveTaskMessagesMoved(sourceArn, destinationArn string) {
	c.moveTaskMessagesMoved.WithLabelValues(sourceArn, destinationArn).Inc()
}

// IncMoveTaskActive increments the active move task gauge.
func (c *Collector) IncMoveTaskActive() {
	c.moveTaskActive.Inc()
}

// DecMoveTaskActive decrements the active move task gauge.
func (c *Collector) DecMoveTaskActive() {
	c.moveTaskActive.Dec()
}
