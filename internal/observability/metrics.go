package observability

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/felixge/httpsnoop"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelprometheus "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const instrumentationName = "github.com/ssubedir/open-spanner"

// Metrics owns the process-wide OpenTelemetry meter provider and Prometheus endpoint.
type Metrics struct {
	provider                  *sdkmetric.MeterProvider
	handler                   http.Handler
	httpRequests              metric.Int64Counter
	httpDuration              metric.Float64Histogram
	grpcRequests              metric.Int64Counter
	grpcDuration              metric.Float64Histogram
	ingestionEvents           metric.Int64Counter
	transactionRetries        metric.Int64Counter
	transactionRetryExhausted metric.Int64Counter
}

type WorkerStats struct {
	Name            string
	LastHeartbeatAt time.Time
	PendingJobs     int
	RunningJobs     int
	FailedJobs      int
	OldestPendingAt time.Time
	LastSuccessAt   time.Time
	LastFailureAt   time.Time
}

// New configures OpenTelemetry metrics with a Prometheus reader and an isolated registry.
func New() (*Metrics, error) {
	registry := prometheus.NewRegistry()
	if err := registry.Register(collectors.NewGoCollector()); err != nil {
		return nil, err
	}
	if err := registry.Register(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{})); err != nil {
		return nil, err
	}

	exporter, err := otelprometheus.New(otelprometheus.WithRegisterer(registry))
	if err != nil {
		return nil, err
	}
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
		sdkmetric.WithResource(resource.NewSchemaless(attribute.String("service.name", "open-spanner"))),
	)
	otel.SetMeterProvider(provider)
	meter := provider.Meter(instrumentationName)

	httpRequests, err := meter.Int64Counter("open_spanner.http.server.requests", metric.WithUnit("{request}"))
	if err != nil {
		return nil, errors.Join(err, provider.Shutdown(context.Background()))
	}
	httpDuration, err := meter.Float64Histogram("open_spanner.http.server.duration", metric.WithUnit("s"))
	if err != nil {
		return nil, errors.Join(err, provider.Shutdown(context.Background()))
	}
	grpcRequests, err := meter.Int64Counter("open_spanner.grpc.server.requests", metric.WithUnit("{request}"))
	if err != nil {
		return nil, errors.Join(err, provider.Shutdown(context.Background()))
	}
	grpcDuration, err := meter.Float64Histogram("open_spanner.grpc.server.duration", metric.WithUnit("s"))
	if err != nil {
		return nil, errors.Join(err, provider.Shutdown(context.Background()))
	}
	ingestionEvents, err := meter.Int64Counter("open_spanner.ingestion.events", metric.WithUnit("{event}"))
	if err != nil {
		return nil, errors.Join(err, provider.Shutdown(context.Background()))
	}
	transactionRetries, err := meter.Int64Counter("open_spanner.db.client.transaction.retries", metric.WithUnit("{retry}"))
	if err != nil {
		return nil, errors.Join(err, provider.Shutdown(context.Background()))
	}
	transactionRetryExhausted, err := meter.Int64Counter("open_spanner.db.client.transaction.retry_exhausted", metric.WithUnit("{transaction}"))
	if err != nil {
		return nil, errors.Join(err, provider.Shutdown(context.Background()))
	}

	return &Metrics{
		provider: provider, handler: promhttp.HandlerFor(registry, promhttp.HandlerOpts{}),
		httpRequests: httpRequests, httpDuration: httpDuration,
		grpcRequests: grpcRequests, grpcDuration: grpcDuration, ingestionEvents: ingestionEvents,
		transactionRetries: transactionRetries, transactionRetryExhausted: transactionRetryExhausted,
	}, nil
}

// Handler returns the Prometheus scrape endpoint.
func (m *Metrics) Handler() http.Handler { return m.handler }

// Shutdown flushes and stops the meter provider.
func (m *Metrics) Shutdown(ctx context.Context) error {
	if m == nil || m.provider == nil {
		return nil
	}
	return m.provider.Shutdown(ctx)
}

// HTTPMiddleware records bounded HTTP server metrics using Chi route templates.
func (m *Metrics) HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}
		started := time.Now()
		captured := httpsnoop.CaptureMetrics(next, w, r)
		route := chi.RouteContext(r.Context()).RoutePattern()
		if route == "" {
			route = "unmatched"
		}
		attrs := metric.WithAttributes(
			attribute.String("http.request.method", r.Method),
			attribute.String("http.route", route),
			attribute.String("http.response.status_code", strconv.Itoa(captured.Code)),
		)
		m.httpRequests.Add(r.Context(), 1, attrs)
		m.httpDuration.Record(r.Context(), time.Since(started).Seconds(), attrs)
	})
}

// UnaryServerInterceptor records bounded gRPC method, status, and duration metrics.
func (m *Metrics) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		started := time.Now()
		response, err := handler(ctx, req)
		m.recordGRPC(ctx, info.FullMethod, status.Code(err), time.Since(started))
		return response, err
	}
}

// StreamServerInterceptor records bounded gRPC method, status, and duration metrics.
func (m *Metrics) StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		started := time.Now()
		err := handler(srv, stream)
		m.recordGRPC(stream.Context(), info.FullMethod, status.Code(err), time.Since(started))
		return err
	}
}

func (m *Metrics) recordGRPC(ctx context.Context, method string, code codes.Code, duration time.Duration) {
	attrs := metric.WithAttributes(
		attribute.String("rpc.system", "grpc"),
		attribute.String("rpc.method", method),
		attribute.String("rpc.grpc.status_code", code.String()),
	)
	m.grpcRequests.Add(ctx, 1, attrs)
	m.grpcDuration.Record(ctx, duration.Seconds(), attrs)
}

// RecordIngestion records event outcomes. Kind and outcome are fixed application enums.
func (m *Metrics) RecordIngestion(ctx context.Context, kind, outcome string, count int) {
	if m == nil || count <= 0 {
		return
	}
	m.ingestionEvents.Add(ctx, int64(count), metric.WithAttributes(
		attribute.String("ingestion.kind", kind),
		attribute.String("ingestion.outcome", outcome),
	))
}

// RecordTransactionRetry records one bounded retry of an aborted Postgres transaction.
func (m *Metrics) RecordTransactionRetry(ctx context.Context, reason string) {
	if m == nil {
		return
	}
	m.transactionRetries.Add(ctx, 1, metric.WithAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.transaction.retry.reason", reason),
	))
}

// RecordTransactionRetryExhausted records a transaction that remained aborted after all attempts.
func (m *Metrics) RecordTransactionRetryExhausted(ctx context.Context, reason string) {
	if m == nil {
		return
	}
	m.transactionRetryExhausted.Add(ctx, 1, metric.WithAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.transaction.retry.reason", reason),
	))
}

// RegisterDBPool exposes database/sql pool saturation without database or tenant identifiers.
func (m *Metrics) RegisterDBPool(stats func() sql.DBStats, driver string) error {
	if m == nil || stats == nil {
		return nil
	}
	meter := m.provider.Meter(instrumentationName)
	connections, err := meter.Int64ObservableGauge("open_spanner.db.client.connections", metric.WithUnit("{connection}"))
	if err != nil {
		return err
	}
	maxOpen, err := meter.Int64ObservableGauge("open_spanner.db.client.connections.max", metric.WithUnit("{connection}"))
	if err != nil {
		return err
	}
	waits, err := meter.Int64ObservableCounter("open_spanner.db.client.connection.waits", metric.WithUnit("{wait}"))
	if err != nil {
		return err
	}
	waitDuration, err := meter.Float64ObservableCounter("open_spanner.db.client.connection.wait_duration", metric.WithUnit("s"))
	if err != nil {
		return err
	}
	_, err = meter.RegisterCallback(func(ctx context.Context, observer metric.Observer) error {
		value := stats()
		driverAttr := attribute.String("db.system", driver)
		observer.ObserveInt64(connections, int64(value.OpenConnections), metric.WithAttributes(driverAttr, attribute.String("state", "open")))
		observer.ObserveInt64(connections, int64(value.InUse), metric.WithAttributes(driverAttr, attribute.String("state", "in_use")))
		observer.ObserveInt64(connections, int64(value.Idle), metric.WithAttributes(driverAttr, attribute.String("state", "idle")))
		observer.ObserveInt64(maxOpen, int64(value.MaxOpenConnections), metric.WithAttributes(driverAttr))
		observer.ObserveInt64(waits, value.WaitCount, metric.WithAttributes(driverAttr))
		observer.ObserveFloat64(waitDuration, value.WaitDuration.Seconds(), metric.WithAttributes(driverAttr))
		return nil
	}, connections, maxOpen, waits, waitDuration)
	return err
}

// RegisterWorkers exposes durable worker heartbeat age and queue diagnostics.
func (m *Metrics) RegisterWorkers(provider func(context.Context) ([]WorkerStats, error)) error {
	if m == nil || provider == nil {
		return nil
	}
	meter := m.provider.Meter(instrumentationName)
	heartbeatAge, err := meter.Float64ObservableGauge("open_spanner.worker.heartbeat.age", metric.WithUnit("s"))
	if err != nil {
		return err
	}
	jobs, err := meter.Int64ObservableGauge("open_spanner.worker.jobs", metric.WithUnit("{job}"))
	if err != nil {
		return err
	}
	oldestPendingAge, err := meter.Float64ObservableGauge("open_spanner.worker.oldest_pending.age", metric.WithUnit("s"))
	if err != nil {
		return err
	}
	lastOutcomeAge, err := meter.Float64ObservableGauge("open_spanner.worker.last_outcome.age", metric.WithUnit("s"))
	if err != nil {
		return err
	}
	_, err = meter.RegisterCallback(func(ctx context.Context, observer metric.Observer) error {
		items, err := provider(ctx)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		for _, item := range items {
			worker := attribute.String("worker.name", item.Name)
			if !item.LastHeartbeatAt.IsZero() {
				observer.ObserveFloat64(heartbeatAge, nonNegativeAge(now, item.LastHeartbeatAt), metric.WithAttributes(worker))
			}
			observer.ObserveInt64(jobs, int64(item.PendingJobs), metric.WithAttributes(worker, attribute.String("state", "pending")))
			observer.ObserveInt64(jobs, int64(item.RunningJobs), metric.WithAttributes(worker, attribute.String("state", "running")))
			observer.ObserveInt64(jobs, int64(item.FailedJobs), metric.WithAttributes(worker, attribute.String("state", "failed")))
			if !item.OldestPendingAt.IsZero() {
				observer.ObserveFloat64(oldestPendingAge, nonNegativeAge(now, item.OldestPendingAt), metric.WithAttributes(worker))
			}
			if !item.LastSuccessAt.IsZero() {
				observer.ObserveFloat64(lastOutcomeAge, nonNegativeAge(now, item.LastSuccessAt), metric.WithAttributes(worker, attribute.String("outcome", "success")))
			}
			if !item.LastFailureAt.IsZero() {
				observer.ObserveFloat64(lastOutcomeAge, nonNegativeAge(now, item.LastFailureAt), metric.WithAttributes(worker, attribute.String("outcome", "failure")))
			}
		}
		return nil
	}, heartbeatAge, jobs, oldestPendingAge, lastOutcomeAge)
	return err
}

func nonNegativeAge(now, value time.Time) float64 {
	age := now.Sub(value).Seconds()
	if age < 0 {
		return 0
	}
	return age
}
