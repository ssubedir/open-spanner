package observability

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestMetricsExposeBoundedCoreSignals(t *testing.T) {
	metrics, err := New()
	if err != nil {
		t.Fatalf("new metrics: %v", err)
	}
	t.Cleanup(func() { _ = metrics.Shutdown(context.Background()) })

	router := chi.NewRouter()
	router.Use(metrics.HTTPMiddleware)
	router.Get("/things/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/things/tenant-secret", nil))

	metrics.RecordIngestion(context.Background(), "bulk", "accepted", 3)
	if err := metrics.RegisterDBPool(func() sql.DBStats {
		return sql.DBStats{MaxOpenConnections: 8, OpenConnections: 3, InUse: 2, Idle: 1, WaitCount: 4, WaitDuration: 2 * time.Second}
	}, "postgres"); err != nil {
		t.Fatalf("register db pool: %v", err)
	}
	if err := metrics.RegisterWorkers(func(context.Context) ([]WorkerStats, error) {
		return []WorkerStats{{Name: "export", LastHeartbeatAt: time.Now().Add(-time.Second), PendingJobs: 2, RunningJobs: 1}}, nil
	}); err != nil {
		t.Fatalf("register workers: %v", err)
	}

	_, grpcErr := metrics.UnaryServerInterceptor()(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/open_spanner.v1.UsageService/CreateUsage"}, func(context.Context, any) (any, error) {
		return nil, status.Error(codes.InvalidArgument, "bad request")
	})
	if status.Code(grpcErr) != codes.InvalidArgument {
		t.Fatalf("grpc status = %v", grpcErr)
	}

	response := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("metrics status = %d", response.Code)
	}
	body := response.Body.String()
	for _, expected := range []string{
		"open_spanner_http_server_requests_total",
		`http_route="/things/{id}"`,
		`http_response_status_code="204"`,
		"open_spanner_grpc_server_requests_total",
		`rpc_grpc_status_code="InvalidArgument"`,
		"open_spanner_ingestion_events_total",
		`ingestion_outcome="accepted"`,
		"open_spanner_db_client_connections",
		`db_system="postgres"`,
		"open_spanner_worker_heartbeat_age_seconds",
		"open_spanner_worker_jobs",
		`worker_name="export"`,
		"go_goroutines",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("metrics missing %q:\n%s", expected, body)
		}
	}
	if strings.Contains(body, "tenant-secret") {
		t.Fatal("metrics included a raw route parameter")
	}
}

func TestStreamInterceptorRecordsHandlerStatus(t *testing.T) {
	metrics, err := New()
	if err != nil {
		t.Fatalf("new metrics: %v", err)
	}
	t.Cleanup(func() { _ = metrics.Shutdown(context.Background()) })

	want := errors.New("stream failed")
	err = metrics.StreamServerInterceptor()(nil, fakeServerStream{ctx: context.Background()}, &grpc.StreamServerInfo{FullMethod: "/open_spanner.v1.UsageService/StreamUsage"}, func(any, grpc.ServerStream) error {
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("stream error = %v", err)
	}
}

type fakeServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s fakeServerStream) Context() context.Context { return s.ctx }
