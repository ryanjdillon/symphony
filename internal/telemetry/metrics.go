package telemetry

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Metrics holds all Symphony OTEL metric instruments.
type Metrics struct {
	// Counters
	DispatchedTotal metric.Int64Counter
	CompletedTotal  metric.Int64Counter
	FailedTotal     metric.Int64Counter
	TokensTotal     metric.Int64Counter
	PollTicksTotal  metric.Int64Counter

	// Histograms
	SessionDuration metric.Float64Histogram
	PollDuration    metric.Float64Histogram

	// UpDownCounters (for gauge-like behavior)
	RunningSessions metric.Int64UpDownCounter
	RetryQueueSize  metric.Int64UpDownCounter
}

// NewMetrics creates and registers all metric instruments.
func NewMetrics(logger *slog.Logger) *Metrics {
	m := Meter()

	dispatched, err := m.Int64Counter("symphony.dispatched_total",
		metric.WithDescription("Issues dispatched to agents"))
	logErr(logger, "dispatched_total", err)

	completed, err := m.Int64Counter("symphony.completed_total",
		metric.WithDescription("Issues completed successfully"))
	logErr(logger, "completed_total", err)

	failed, err := m.Int64Counter("symphony.failed_total",
		metric.WithDescription("Agent sessions that failed"))
	logErr(logger, "failed_total", err)

	tokens, err := m.Int64Counter("symphony.tokens_total",
		metric.WithDescription("Tokens consumed by agent sessions"))
	logErr(logger, "tokens_total", err)

	pollTicks, err := m.Int64Counter("symphony.poll_ticks_total",
		metric.WithDescription("Poll cycles executed"))
	logErr(logger, "poll_ticks_total", err)

	sessionDuration, err := m.Float64Histogram("symphony.session_duration_seconds",
		metric.WithDescription("Agent session duration"),
		metric.WithUnit("s"))
	logErr(logger, "session_duration_seconds", err)

	pollDuration, err := m.Float64Histogram("symphony.poll_duration_seconds",
		metric.WithDescription("Poll tick duration"),
		metric.WithUnit("s"))
	logErr(logger, "poll_duration_seconds", err)

	running, err := m.Int64UpDownCounter("symphony.running_sessions",
		metric.WithDescription("Current number of running agent sessions"))
	logErr(logger, "running_sessions", err)

	retryQueue, err := m.Int64UpDownCounter("symphony.retry_queue_size",
		metric.WithDescription("Current retry queue depth"))
	logErr(logger, "retry_queue_size", err)

	return &Metrics{
		DispatchedTotal: dispatched,
		CompletedTotal:  completed,
		FailedTotal:     failed,
		TokensTotal:     tokens,
		PollTicksTotal:  pollTicks,
		SessionDuration: sessionDuration,
		PollDuration:    pollDuration,
		RunningSessions: running,
		RetryQueueSize:  retryQueue,
	}
}

// Common attribute keys.
var (
	AttrWorkflow  = attribute.Key("workflow")
	AttrState     = attribute.Key("state")
	AttrHost      = attribute.Key("host")
	AttrReason    = attribute.Key("reason")
	AttrDirection = attribute.Key("direction")
)

// RecordDispatched records an issue dispatch.
func (m *Metrics) RecordDispatched(ctx context.Context, workflow, state string) {
	if m == nil {
		return
	}
	m.DispatchedTotal.Add(ctx, 1,
		metric.WithAttributes(AttrWorkflow.String(workflow), AttrState.String(state)))
	m.RunningSessions.Add(ctx, 1,
		metric.WithAttributes(AttrWorkflow.String(workflow), AttrState.String(state)))
}

// RecordCompleted records a successful session completion.
func (m *Metrics) RecordCompleted(ctx context.Context, workflow string, duration time.Duration) {
	if m == nil {
		return
	}
	m.CompletedTotal.Add(ctx, 1, metric.WithAttributes(AttrWorkflow.String(workflow)))
	m.RunningSessions.Add(ctx, -1, metric.WithAttributes(AttrWorkflow.String(workflow)))
	m.SessionDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(AttrWorkflow.String(workflow)))
}

// RecordFailed records a failed session.
func (m *Metrics) RecordFailed(ctx context.Context, workflow, reason string, duration time.Duration) {
	if m == nil {
		return
	}
	m.FailedTotal.Add(ctx, 1,
		metric.WithAttributes(AttrWorkflow.String(workflow), AttrReason.String(reason)))
	m.RunningSessions.Add(ctx, -1, metric.WithAttributes(AttrWorkflow.String(workflow)))
	m.SessionDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(AttrWorkflow.String(workflow)))
}

// RecordTokens records token consumption.
func (m *Metrics) RecordTokens(ctx context.Context, workflow string, input, output int64) {
	if m == nil {
		return
	}
	m.TokensTotal.Add(ctx, input,
		metric.WithAttributes(AttrWorkflow.String(workflow), AttrDirection.String("input")))
	m.TokensTotal.Add(ctx, output,
		metric.WithAttributes(AttrWorkflow.String(workflow), AttrDirection.String("output")))
}

// RecordPollTick records a poll cycle.
func (m *Metrics) RecordPollTick(ctx context.Context, workflow string, duration time.Duration) {
	if m == nil {
		return
	}
	m.PollTicksTotal.Add(ctx, 1, metric.WithAttributes(AttrWorkflow.String(workflow)))
	m.PollDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(AttrWorkflow.String(workflow)))
}

// RecordRetryQueued increments the retry queue gauge.
func (m *Metrics) RecordRetryQueued(ctx context.Context, workflow string) {
	if m == nil {
		return
	}
	m.RetryQueueSize.Add(ctx, 1, metric.WithAttributes(AttrWorkflow.String(workflow)))
}

// RecordRetryDequeued decrements the retry queue gauge.
func (m *Metrics) RecordRetryDequeued(ctx context.Context, workflow string) {
	if m == nil {
		return
	}
	m.RetryQueueSize.Add(ctx, -1, metric.WithAttributes(AttrWorkflow.String(workflow)))
}

func logErr(logger *slog.Logger, name string, err error) {
	if err != nil {
		logger.Warn("failed to create metric", "name", name, "error", err)
	}
}
