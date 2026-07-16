package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ssubedir/open-spanner/internal/config"
	grpcadapter "github.com/ssubedir/open-spanner/internal/metering/adapters/grpc"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/grpc/pb"
	"github.com/ssubedir/open-spanner/internal/metering/bootstrap"
	"github.com/ssubedir/open-spanner/internal/observability"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type options struct {
	dsn           string
	events        int
	concurrency   int
	bulkSize      int
	maxP95        time.Duration
	minThroughput float64
	timeout       time.Duration
}

type loadEvent struct {
	IdempotencyKey string         `json:"idempotency_key"`
	Subject        string         `json:"subject"`
	Meter          string         `json:"meter"`
	Quantity       float64        `json:"quantity"`
	Timestamp      string         `json:"timestamp"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type operation struct {
	scenario string
	events   int
	run      func(context.Context) error
}

type measurement struct {
	scenario string
	duration time.Duration
	events   int
	err      error
}

type scenarioSummary struct {
	operations int
	events     int
	errors     int
	durations  []time.Duration
}

type environment struct {
	metrics    *observability.Metrics
	app        *bootstrap.App
	httpServer *http.Server
	httpURL    string
	grpcServer *grpc.Server
	grpcConn   *grpc.ClientConn
	httpClient *http.Client
	grpcClient pb.UsageServiceClient
	apiKey     string
	meter      string
	subject    string
	runID      string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "load regression failed:", err)
		os.Exit(1)
	}
}

func run() error {
	opts := parseOptions()
	if err := validateOptions(opts); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), opts.timeout)
	defer cancel()

	env, err := startEnvironment(ctx, opts)
	if err != nil {
		return err
	}
	defer env.close()

	operations, expectedKeys, replay, err := buildOperations(env, opts)
	if err != nil {
		return err
	}
	rand.New(rand.NewSource(1)).Shuffle(len(operations), func(i, j int) { operations[i], operations[j] = operations[j], operations[i] })

	started := time.Now()
	measurements := execute(ctx, operations, opts.concurrency)
	elapsed := time.Since(started)
	summary, failures := summarize(measurements)
	printSummary(opts, summary, elapsed)

	if len(failures) > 0 {
		return fmt.Errorf("%d operations failed; first failures: %s", len(failures), strings.Join(failures[:min(5, len(failures))], "; "))
	}
	if err := replay(ctx); err != nil {
		return fmt.Errorf("idempotent replay: %w", err)
	}
	if err := verifyEvents(ctx, env, expectedKeys); err != nil {
		return err
	}
	if err := verifyMetrics(ctx, env); err != nil {
		return err
	}

	all := summary["all"]
	p95 := percentile(all.durations, 0.95)
	throughput := float64(len(expectedKeys)) / elapsed.Seconds()
	if p95 > opts.maxP95 {
		return fmt.Errorf("p95 latency %s exceeded maximum %s", p95.Round(time.Millisecond), opts.maxP95)
	}
	if throughput < opts.minThroughput {
		return fmt.Errorf("throughput %.1f events/s fell below minimum %.1f", throughput, opts.minThroughput)
	}

	fmt.Printf("PASS: %d unique events persisted with safe replays and bounded telemetry labels\n", len(expectedKeys))
	return nil
}

func parseOptions() options {
	var opts options
	flag.StringVar(&opts.dsn, "dsn", os.Getenv("OPEN_SPANNER_LOAD_POSTGRES_DSN"), "Postgres DSN (or OPEN_SPANNER_LOAD_POSTGRES_DSN)")
	flag.IntVar(&opts.events, "events", 500, "unique events to ingest across all scenarios")
	flag.IntVar(&opts.concurrency, "concurrency", 12, "concurrent client operations")
	flag.IntVar(&opts.bulkSize, "bulk-size", 20, "events per REST, gRPC bulk, or stream operation")
	flag.DurationVar(&opts.maxP95, "max-p95", 2*time.Second, "maximum overall operation p95 latency")
	flag.Float64Var(&opts.minThroughput, "min-throughput", 20, "minimum accepted events per second")
	flag.DurationVar(&opts.timeout, "timeout", 3*time.Minute, "overall test timeout")
	flag.Parse()
	return opts
}

func validateOptions(opts options) error {
	if strings.TrimSpace(opts.dsn) == "" {
		return errors.New("Postgres DSN is required; set OPEN_SPANNER_LOAD_POSTGRES_DSN")
	}
	if opts.events < 25 || opts.concurrency <= 0 || opts.bulkSize <= 0 || opts.bulkSize > 1000 {
		return errors.New("events must be at least 25, concurrency positive, and bulk-size between 1 and 1000")
	}
	if opts.maxP95 <= 0 || opts.minThroughput < 0 || opts.timeout <= 0 {
		return errors.New("invalid regression threshold")
	}
	return nil
}

func startEnvironment(ctx context.Context, opts options) (_ *environment, err error) {
	metrics, err := observability.New()
	if err != nil {
		return nil, fmt.Errorf("initialize metrics: %w", err)
	}
	env := &environment{metrics: metrics}
	defer func() {
		if err != nil {
			env.close()
		}
	}()

	cfg := config.Config{
		DBDriver: "postgres", PostgresDSN: opts.dsn,
		DBPool:              config.DBPoolConfig{MaxOpenConns: opts.concurrency + 4, MaxIdleConns: opts.concurrency + 4},
		RegistrationEnabled: true, ExportStorageDriver: "filesystem", ExportStoragePath: os.TempDir(),
		IngestionMaxBodyBytes: 4 * 1024 * 1024, IngestionMaxBulkEvents: 1000, IngestionMaxStreamEvents: 1000,
		IngestionRateLimitEvents: opts.events * 10, IngestionRateLimitWindow: time.Minute,
		RetentionPruneInterval: time.Hour, ReconciliationStaleAfter: time.Hour,
	}
	router := chi.NewRouter()
	router.Use(metrics.HTTPMiddleware)
	router.Handle("/metrics", metrics.Handler())
	app, err := bootstrap.RegisterRoutesWithMetrics(ctx, router, cfg, metrics)
	if err != nil {
		return nil, fmt.Errorf("initialize app: %w", err)
	}
	env.app = app
	if err := metrics.RegisterDBPool(app.DatabaseStats, cfg.DBDriver); err != nil {
		return nil, fmt.Errorf("register database metrics: %w", err)
	}

	httpListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen HTTP: %w", err)
	}
	env.httpServer = &http.Server{Handler: router, ReadHeaderTimeout: 5 * time.Second}
	env.httpURL = "http://" + httpListener.Addr().String()
	go func() { _ = env.httpServer.Serve(httpListener) }()

	grpcListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen gRPC: %w", err)
	}
	env.grpcServer = grpcadapter.NewInstrumentedServerWithIngestionLimits(app.UsageService, app.AlertService, app.EntitlementService, app.AuthService, app.Authorizer, grpcadapter.IngestionLimits{MaxBulkEvents: 1000, MaxStreamEvents: 1000}, metrics)
	go func() { _ = env.grpcServer.Serve(grpcListener) }()
	env.grpcConn, err = grpc.NewClient(grpcListener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial gRPC: %w", err)
	}
	env.grpcClient = pb.NewUsageServiceClient(env.grpcConn)
	env.httpClient = &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{MaxIdleConns: opts.concurrency * 2, MaxIdleConnsPerHost: opts.concurrency * 2}}

	if err := env.setup(ctx); err != nil {
		return nil, err
	}
	return env, nil
}

func (e *environment) close() {
	if e.grpcConn != nil {
		_ = e.grpcConn.Close()
	}
	if e.grpcServer != nil {
		e.grpcServer.Stop()
	}
	if e.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = e.httpServer.Shutdown(ctx)
		cancel()
	}
	if e.app != nil {
		_ = e.app.Cleanup()
	}
	if e.metrics != nil {
		_ = e.metrics.Shutdown(context.Background())
	}
}

func (e *environment) setup(ctx context.Context) error {
	e.runID = strings.ToLower(fmt.Sprintf("%x", time.Now().UnixNano()))
	e.meter = "load_events_" + e.runID
	e.subject = "load_subject_" + e.runID
	email := "load+" + e.runID + "@example.com"
	if _, _, err := e.jsonRequest(ctx, http.MethodPost, "/v1/auth/users", map[string]any{"email": email, "password": "strong-password"}, nil, nil, http.StatusCreated); err != nil {
		return fmt.Errorf("register load user: %w", err)
	}
	login, _, err := e.jsonRequest(ctx, http.MethodPost, "/v1/auth/sessions", map[string]any{"email": email, "password": "strong-password"}, nil, nil, http.StatusCreated)
	if err != nil {
		return fmt.Errorf("login load user: %w", err)
	}
	cookies := login.Cookies()
	_, keyBody, err := e.jsonRequest(ctx, http.MethodPost, "/v1/auth/api-keys", map[string]any{
		"name": "load-test", "scopes": []string{"usage:write", "usage:read", "meters:write", "meters:read"},
	}, nil, cookies, http.StatusCreated)
	if err != nil {
		return fmt.Errorf("create load API key: %w", err)
	}
	var key struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(keyBody, &key); err != nil || key.Key == "" {
		return errors.New("decode load API key")
	}
	e.apiKey = key.Key
	_, _, err = e.jsonRequest(ctx, http.MethodPost, "/v1/meters", map[string]any{
		"name": e.meter, "description": "Load regression events", "unit": "event", "aggregation": "sum",
		"dimensions": []map[string]any{{"name": "protocol", "type": "string", "required": true}},
	}, e.authHeader(), nil, http.StatusCreated)
	if err != nil {
		return fmt.Errorf("create load meter: %w", err)
	}
	return nil
}

func buildOperations(env *environment, opts options) ([]operation, map[string]struct{}, func(context.Context) error, error) {
	counts := split(opts.events, 5)
	expected := make(map[string]struct{}, opts.events)
	index := 0
	newEvent := func(protocol string) loadEvent {
		index++
		key := fmt.Sprintf("load-%s-%06d", env.runID, index)
		expected[key] = struct{}{}
		return loadEvent{IdempotencyKey: key, Subject: env.subject, Meter: env.meter, Quantity: 1, Timestamp: time.Now().UTC().Format(time.RFC3339Nano), Metadata: map[string]any{"protocol": protocol}}
	}

	operations := make([]operation, 0, opts.events)
	restSingles := make([]loadEvent, 0, counts[0])
	for range counts[0] {
		event := newEvent("rest-single")
		restSingles = append(restSingles, event)
		operations = append(operations, operation{scenario: "rest-single", events: 1, run: func(ctx context.Context) error {
			_, _, err := env.jsonRequest(ctx, http.MethodPost, "/v1/usages", event, env.authHeader(), nil, http.StatusCreated)
			return err
		}})
	}

	restBatches := chunkEvents(counts[1], opts.bulkSize, func() loadEvent { return newEvent("rest-bulk") })
	for batchIndex, batch := range restBatches {
		batchKey := fmt.Sprintf("rest-batch-%s-%d", env.runID, batchIndex)
		operations = append(operations, operation{scenario: "rest-bulk", events: len(batch), run: func(ctx context.Context) error {
			headers := env.authHeader()
			headers.Set("Idempotency-Key", batchKey)
			_, body, err := env.jsonRequest(ctx, http.MethodPost, "/v1/usages/bulk", batch, headers, nil, http.StatusCreated)
			if err != nil {
				return err
			}
			return requireBulkCounts(body, len(batch), 0, 0)
		}})
	}

	grpcSingles := make([]loadEvent, 0, counts[2])
	for range counts[2] {
		event := newEvent("grpc-unary")
		grpcSingles = append(grpcSingles, event)
		operations = append(operations, operation{scenario: "grpc-unary", events: 1, run: func(ctx context.Context) error {
			_, err := env.grpcClient.CreateUsage(env.grpcContext(ctx), &pb.CreateUsageRequest{Event: protoEvent(event)})
			return err
		}})
	}

	grpcBatches := chunkEvents(counts[3], opts.bulkSize, func() loadEvent { return newEvent("grpc-bulk") })
	for batchIndex, batch := range grpcBatches {
		batchKey := fmt.Sprintf("grpc-batch-%s-%d", env.runID, batchIndex)
		operations = append(operations, operation{scenario: "grpc-bulk", events: len(batch), run: func(ctx context.Context) error {
			result, err := env.grpcClient.CreateUsageBulk(env.grpcContext(ctx), &pb.CreateUsageBulkRequest{IdempotencyKey: batchKey, Events: protoEvents(batch)})
			if err != nil {
				return err
			}
			if int(result.GetAcceptedCount()) != len(batch) || result.GetDuplicateCount() != 0 || result.GetFailedCount() != 0 {
				return fmt.Errorf("counts accepted=%d duplicate=%d failed=%d", result.GetAcceptedCount(), result.GetDuplicateCount(), result.GetFailedCount())
			}
			return nil
		}})
	}

	streamBatches := chunkEvents(counts[4], opts.bulkSize, func() loadEvent { return newEvent("grpc-stream") })
	for batchIndex, batch := range streamBatches {
		batchKey := fmt.Sprintf("grpc-stream-%s-%d", env.runID, batchIndex)
		operations = append(operations, operation{scenario: "grpc-stream", events: len(batch), run: func(ctx context.Context) error {
			stream, err := env.grpcClient.StreamUsage(metadata.AppendToOutgoingContext(env.grpcContext(ctx), "idempotency-key", batchKey))
			if err != nil {
				return err
			}
			for _, event := range batch {
				if err := stream.Send(&pb.StreamUsageRequest{Event: protoEvent(event)}); err != nil {
					return err
				}
			}
			result, err := stream.CloseAndRecv()
			if err != nil {
				return err
			}
			if int(result.GetAcceptedCount()) != len(batch) || result.GetDuplicateCount() != 0 || result.GetFailedCount() != 0 {
				return fmt.Errorf("counts accepted=%d duplicate=%d failed=%d", result.GetAcceptedCount(), result.GetDuplicateCount(), result.GetFailedCount())
			}
			return nil
		}})
	}
	if len(restSingles) == 0 || len(restBatches) == 0 || len(grpcSingles) == 0 || len(grpcBatches) == 0 || len(streamBatches) == 0 {
		return nil, nil, nil, errors.New("load allocation did not cover every protocol")
	}

	replay := func(ctx context.Context) error {
		if _, _, err := env.jsonRequest(ctx, http.MethodPost, "/v1/usages", restSingles[0], env.authHeader(), nil, http.StatusCreated); err != nil {
			return err
		}
		headers := env.authHeader()
		headers.Set("Idempotency-Key", fmt.Sprintf("rest-batch-%s-%d", env.runID, 0))
		if _, _, err := env.jsonRequest(ctx, http.MethodPost, "/v1/usages/bulk", restBatches[0], headers, nil, http.StatusCreated); err != nil {
			return err
		}
		if _, err := env.grpcClient.CreateUsage(env.grpcContext(ctx), &pb.CreateUsageRequest{Event: protoEvent(grpcSingles[0])}); err != nil {
			return err
		}
		if _, err := env.grpcClient.CreateUsageBulk(env.grpcContext(ctx), &pb.CreateUsageBulkRequest{IdempotencyKey: fmt.Sprintf("grpc-batch-%s-%d", env.runID, 0), Events: protoEvents(grpcBatches[0])}); err != nil {
			return err
		}
		result, err := env.grpcClient.CreateUsageBulk(env.grpcContext(ctx), &pb.CreateUsageBulkRequest{IdempotencyKey: "stream-recovery-" + env.runID, Events: protoEvents(streamBatches[0])})
		if err != nil {
			return err
		}
		if int(result.GetDuplicateCount()) != len(streamBatches[0]) || result.GetAcceptedCount() != 0 || result.GetFailedCount() != 0 {
			return fmt.Errorf("stream recovery counts accepted=%d duplicate=%d failed=%d", result.GetAcceptedCount(), result.GetDuplicateCount(), result.GetFailedCount())
		}
		return nil
	}
	return operations, expected, replay, nil
}

func execute(ctx context.Context, operations []operation, concurrency int) []measurement {
	jobs := make(chan operation)
	results := make(chan measurement, len(operations))
	var workers sync.WaitGroup
	for range concurrency {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for op := range jobs {
				started := time.Now()
				err := op.run(ctx)
				results <- measurement{scenario: op.scenario, duration: time.Since(started), events: op.events, err: err}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, op := range operations {
			select {
			case jobs <- op:
			case <-ctx.Done():
				return
			}
		}
	}()
	workers.Wait()
	close(results)
	measurements := make([]measurement, 0, len(operations))
	for result := range results {
		measurements = append(measurements, result)
	}
	return measurements
}

func summarize(measurements []measurement) (map[string]*scenarioSummary, []string) {
	summary := map[string]*scenarioSummary{"all": {}}
	failures := []string{}
	for _, result := range measurements {
		for _, name := range []string{"all", result.scenario} {
			item := summary[name]
			if item == nil {
				item = &scenarioSummary{}
				summary[name] = item
			}
			item.operations++
			item.events += result.events
			item.durations = append(item.durations, result.duration)
			if result.err != nil {
				item.errors++
			}
		}
		if result.err != nil {
			failures = append(failures, result.scenario+": "+result.err.Error())
		}
	}
	return summary, failures
}

func printSummary(opts options, summary map[string]*scenarioSummary, elapsed time.Duration) {
	fmt.Printf("events=%d concurrency=%d bulk_size=%d elapsed=%s throughput=%.1f events/s\n", opts.events, opts.concurrency, opts.bulkSize, elapsed.Round(time.Millisecond), float64(opts.events)/elapsed.Seconds())
	fmt.Println("scenario       ops  events  errors  p50       p95       p99")
	for _, name := range []string{"rest-single", "rest-bulk", "grpc-unary", "grpc-bulk", "grpc-stream", "all"} {
		item := summary[name]
		if item == nil {
			continue
		}
		fmt.Printf("%-14s %4d %7d %7d  %-9s %-9s %s\n", name, item.operations, item.events, item.errors, percentile(item.durations, .50).Round(time.Millisecond), percentile(item.durations, .95).Round(time.Millisecond), percentile(item.durations, .99).Round(time.Millisecond))
	}
}

func verifyEvents(ctx context.Context, env *environment, expected map[string]struct{}) error {
	found := make(map[string]struct{}, len(expected))
	cursor := ""
	for {
		query := url.Values{"meter": {env.meter}, "subject": {env.subject}, "limit": {"1000"}}
		if cursor != "" {
			query.Set("cursor", cursor)
		}
		_, body, err := env.jsonRequest(ctx, http.MethodGet, "/v1/usageevents?"+query.Encode(), nil, env.authHeader(), nil, http.StatusOK)
		if err != nil {
			return fmt.Errorf("list persisted events: %w", err)
		}
		var page struct {
			Items []struct {
				IdempotencyKey string `json:"idempotency_key"`
			} `json:"items"`
			NextCursor string `json:"next_cursor"`
		}
		if err := json.Unmarshal(body, &page); err != nil {
			return fmt.Errorf("decode persisted events: %w", err)
		}
		for _, event := range page.Items {
			if _, duplicate := found[event.IdempotencyKey]; duplicate {
				return fmt.Errorf("duplicate persisted idempotency key %q", event.IdempotencyKey)
			}
			found[event.IdempotencyKey] = struct{}{}
		}
		cursor = page.NextCursor
		if cursor == "" {
			break
		}
	}
	if len(found) != len(expected) {
		return fmt.Errorf("persisted event count=%d, want=%d", len(found), len(expected))
	}
	for key := range expected {
		if _, ok := found[key]; !ok {
			return fmt.Errorf("persisted events missing %q", key)
		}
	}
	return nil
}

func verifyMetrics(ctx context.Context, env *environment) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, env.httpURL+"/metrics", nil)
	if err != nil {
		return err
	}
	response, err := env.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("scrape metrics: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("metrics status=%d", response.StatusCode)
	}
	text := string(body)
	for _, family := range []string{"open_spanner_http_server_requests_total", "open_spanner_grpc_server_requests_total", "open_spanner_ingestion_events_total", "open_spanner_db_client_connections"} {
		if !strings.Contains(text, family) {
			return fmt.Errorf("metrics missing %s", family)
		}
	}
	if strings.Contains(text, env.runID) || strings.Contains(text, env.subject) || strings.Contains(text, env.meter) {
		return errors.New("metrics exposed a high-cardinality load identifier")
	}
	return nil
}

func (e *environment) jsonRequest(ctx context.Context, method, path string, body any, headers http.Header, cookies []*http.Cookie, expectedStatus int) (*http.Response, []byte, error) {
	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, nil, err
		}
		payload = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, e.httpURL+path, payload)
	if err != nil {
		return nil, nil, err
	}
	for name, values := range headers {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	response, err := e.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return response, nil, err
	}
	if response.StatusCode != expectedStatus {
		return response, responseBody, fmt.Errorf("%s %s status=%d want=%d body=%s", method, path, response.StatusCode, expectedStatus, strings.TrimSpace(string(responseBody)))
	}
	return response, responseBody, nil
}

func (e *environment) authHeader() http.Header {
	return http.Header{"Authorization": {"Bearer " + e.apiKey}}
}

func (e *environment) grpcContext(ctx context.Context) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+e.apiKey)
}

func protoEvent(event loadEvent) *pb.UsageEventInput {
	timestamp, _ := time.Parse(time.RFC3339Nano, event.Timestamp)
	return &pb.UsageEventInput{
		IdempotencyKey: event.IdempotencyKey, Subject: event.Subject, Meter: event.Meter,
		Quantity: event.Quantity, Timestamp: timestamppb.New(timestamp),
		Metadata: map[string]*structpb.Value{"protocol": structpb.NewStringValue(event.Metadata["protocol"].(string))},
	}
}

func protoEvents(events []loadEvent) []*pb.UsageEventInput {
	result := make([]*pb.UsageEventInput, 0, len(events))
	for _, event := range events {
		result = append(result, protoEvent(event))
	}
	return result
}

func requireBulkCounts(body []byte, accepted, duplicates, failed int) error {
	var result struct {
		Accepted   int `json:"accepted"`
		Duplicates int `json:"duplicates"`
		Failed     int `json:"failed"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return err
	}
	if result.Accepted != accepted || result.Duplicates != duplicates || result.Failed != failed {
		return fmt.Errorf("counts accepted=%d duplicate=%d failed=%d", result.Accepted, result.Duplicates, result.Failed)
	}
	return nil
}

func split(total, parts int) []int {
	result := make([]int, parts)
	for i := range result {
		result[i] = total / parts
	}
	for i := 0; i < total%parts; i++ {
		result[i]++
	}
	return result
}

func chunkEvents(count, size int, create func() loadEvent) [][]loadEvent {
	result := make([][]loadEvent, 0, int(math.Ceil(float64(count)/float64(size))))
	for remaining := count; remaining > 0; {
		length := min(size, remaining)
		batch := make([]loadEvent, 0, length)
		for range length {
			batch = append(batch, create())
		}
		result = append(result, batch)
		remaining -= length
	}
	return result
}

func percentile(values []time.Duration, quantile float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	ordered := append([]time.Duration(nil), values...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	index := int(math.Ceil(quantile*float64(len(ordered)))) - 1
	return ordered[max(0, min(index, len(ordered)-1))]
}
