package bootstrap

import (
	"context"
	"net"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ssubedir/open-spanner/internal/config"
	grpcadapter "github.com/ssubedir/open-spanner/internal/metering/adapters/grpc"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/grpc/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestIntegrationSQLiteGRPCUsageFlow(t *testing.T) {
	runIntegrationGRPCUsageFlow(t, config.Config{
		DBDriver:   "sqlite",
		SQLitePath: ":memory:",
		DBPool:     config.DBPoolConfig{MaxOpenConns: 1},
	}, "sqlite")
}

func TestIntegrationPostgresGRPCUsageFlow(t *testing.T) {
	dsn := os.Getenv("OPEN_SPANNER_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set OPEN_SPANNER_TEST_POSTGRES_DSN to run Postgres bootstrap integration tests")
	}

	runIntegrationGRPCUsageFlow(t, config.Config{
		DBDriver:    "postgres",
		PostgresDSN: dsn,
		DBPool:      config.DBPoolConfig{MaxOpenConns: 1},
	}, "postgres")
}

func runIntegrationGRPCUsageFlow(t *testing.T, cfg config.Config, namespace string) {
	t.Helper()

	if cfg.ExportStoragePath == "" {
		cfg.ExportStoragePath = t.TempDir()
	}

	ctx := context.Background()
	router := chi.NewRouter()
	app, err := RegisterRoutes(ctx, router, cfg)
	if err != nil {
		t.Fatalf("register routes: %v", err)
	}
	t.Cleanup(func() {
		if err := app.Cleanup(); err != nil {
			t.Fatalf("cleanup: %v", err)
		}
	})

	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpcadapter.NewServer(app.UsageService, app.AlertService, app.AuthService)
	go func() {
		_ = grpcServer.Serve(listener)
	}()
	t.Cleanup(grpcServer.Stop)

	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial grpc server: %v", err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Fatalf("close grpc client: %v", err)
		}
	})
	client := pb.NewUsageServiceClient(conn)

	_, err = client.CreateUsage(ctx, &pb.CreateUsageRequest{})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("unauthenticated grpc status = %v, want %v", status.Code(err), codes.Unauthenticated)
	}

	suffix := namespace + "_grpc_" + strconv.FormatInt(time.Now().UTC().UnixNano(), 36)
	meterName := "grpc_api_calls_" + suffix
	apiKey := createTestDashboardAPIKey(t, router, "grpc+"+suffix+"@example.com")
	authHeaders := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}

	createMeter := requestJSONWithHeaders(t, router, http.MethodPost, "/v1/meters", map[string]any{
		"name":        meterName,
		"description": "gRPC API calls",
		"unit":        "call",
		"aggregation": "sum",
		"dimensions":  meterDimensionsFromSchema(map[string]string{"endpoint": "string", "status": "number"}),
	}, authHeaders, nil)
	if createMeter.Code != http.StatusCreated {
		t.Fatalf("create meter status = %d, want %d: %s", createMeter.Code, http.StatusCreated, createMeter.Body.String())
	}

	grpcCtx := metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+apiKey)
	bulk, err := client.CreateUsageBulk(grpcCtx, &pb.CreateUsageBulkRequest{
		IdempotencyKey: "grpc-bulk-" + suffix,
		Events: []*pb.UsageEventInput{
			grpcUsageEvent("grpc-bulk-"+suffix+"-1", "org_123_"+suffix, meterName, 2, "2026-06-08T10:00:00Z", map[string]any{"endpoint": "/orders", "status": 200}),
			grpcUsageEvent("grpc-bulk-"+suffix+"-2", "org_123_"+suffix, meterName, 3, "2026-06-08T11:00:00Z", map[string]any{"endpoint": "/users", "status": 201}),
		},
	})
	if err != nil {
		t.Fatalf("create grpc bulk usage: %v", err)
	}
	if bulk.GetAcceptedCount() != 2 || bulk.GetDuplicateCount() != 0 || bulk.GetFailedCount() != 0 {
		t.Fatalf("grpc bulk result = accepted %d duplicate %d failed %d, want 2/0/0", bulk.GetAcceptedCount(), bulk.GetDuplicateCount(), bulk.GetFailedCount())
	}

	stream, err := client.StreamUsage(metadata.AppendToOutgoingContext(grpcCtx, "idempotency-key", "grpc-stream-"+suffix))
	if err != nil {
		t.Fatalf("open grpc stream: %v", err)
	}
	if err := stream.Send(&pb.StreamUsageRequest{
		Event: grpcUsageEvent("grpc-stream-"+suffix+"-1", "org_456_"+suffix, meterName, 7, "2026-06-08T12:00:00Z", map[string]any{"endpoint": "/orders", "status": 200}),
	}); err != nil {
		t.Fatalf("send grpc stream usage: %v", err)
	}
	streamResult, err := stream.CloseAndRecv()
	if err != nil {
		t.Fatalf("close grpc stream: %v", err)
	}
	if streamResult.GetAcceptedCount() != 1 || streamResult.GetDuplicateCount() != 0 || streamResult.GetFailedCount() != 0 {
		t.Fatalf("grpc stream result = accepted %d duplicate %d failed %d, want 1/0/0", streamResult.GetAcceptedCount(), streamResult.GetDuplicateCount(), streamResult.GetFailedCount())
	}

	eventsRes := requestJSONWithHeaders(t, router, http.MethodGet, "/v1/usageevents?meter="+meterName+"&limit=10", nil, authHeaders, nil)
	if eventsRes.Code != http.StatusOK {
		t.Fatalf("list grpc-created events status = %d, want %d: %s", eventsRes.Code, http.StatusOK, eventsRes.Body.String())
	}
	var events usageEventListResponse
	decodeJSON(t, eventsRes, &events)
	if len(events.Items) != 3 {
		t.Fatalf("grpc-created events = %d, want 3", len(events.Items))
	}
}

func grpcUsageEvent(idempotencyKey string, subject string, meterName string, quantity float64, timestamp string, metadata map[string]any) *pb.UsageEventInput {
	eventTime, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		panic(err)
	}

	return &pb.UsageEventInput{
		IdempotencyKey: idempotencyKey,
		Subject:        subject,
		Meter:          meterName,
		Quantity:       quantity,
		Timestamp:      timestamppb.New(eventTime),
		Metadata:       grpcMetadata(metadata),
	}
}

func grpcMetadata(metadata map[string]any) map[string]*structpb.Value {
	values := make(map[string]*structpb.Value, len(metadata))
	for key, value := range metadata {
		protoValue, err := structpb.NewValue(value)
		if err != nil {
			panic(err)
		}
		values[key] = protoValue
	}
	return values
}
