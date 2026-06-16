package bootstrap

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ssubedir/open-spanner/internal/config"
)

func TestIntegrationAuthGuardsSDKAndDashboardRoutes(t *testing.T) {
	ctx := context.Background()
	router := chi.NewRouter()
	app, err := RegisterRoutes(ctx, router, config.Config{
		DBDriver:   "sqlite",
		SQLitePath: ":memory:",
		DBPool:     config.DBPoolConfig{MaxOpenConns: 1},
	})
	if err != nil {
		t.Fatalf("register routes: %v", err)
	}
	t.Cleanup(func() {
		if err := app.Cleanup(); err != nil {
			t.Fatalf("cleanup: %v", err)
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/meters", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("meters status = %d, want %d: %s", res.Code, http.StatusUnauthorized, res.Body.String())
	}

	createUser := requestJSON(t, router, http.MethodPost, "/v1/auth/users", map[string]any{
		"email":    "admin@example.com",
		"password": "strong-password",
	}, nil)
	if createUser.Code != http.StatusCreated {
		t.Fatalf("create user status = %d, want %d: %s", createUser.Code, http.StatusCreated, createUser.Body.String())
	}

	login := requestJSON(t, router, http.MethodPost, "/v1/auth/sessions", map[string]any{
		"email":    "admin@example.com",
		"password": "strong-password",
	}, nil)
	if login.Code != http.StatusCreated {
		t.Fatalf("login status = %d, want %d: %s", login.Code, http.StatusCreated, login.Body.String())
	}

	keyRes := requestJSON(t, router, http.MethodPost, "/v1/auth/api-keys", map[string]any{
		"name": "sdk",
	}, login.Result().Cookies())
	if keyRes.Code != http.StatusCreated {
		t.Fatalf("create api key status = %d, want %d: %s", keyRes.Code, http.StatusCreated, keyRes.Body.String())
	}

	var key struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(keyRes.Body).Decode(&key); err != nil {
		t.Fatalf("decode api key: %v", err)
	}
	if key.Key == "" {
		t.Fatal("api key is empty")
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/meters", nil)
	req.Header.Set("Authorization", "Bearer "+key.Key)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("api-key meters status = %d, want %d: %s", res.Code, http.StatusOK, res.Body.String())
	}

	savedQueryPayload := map[string]any{
		"name":        "API usage by endpoint",
		"query":       map[string]any{"combinator": "and", "rules": []any{}},
		"group_by":    []string{"endpoint"},
		"bucket_size": "day",
		"limit":       500,
	}

	apiKeySavedQuery := requestJSONWithHeaders(t, router, http.MethodPost, "/v1/usage/saved-queries", savedQueryPayload, map[string]string{
		"Authorization": "Bearer " + key.Key,
	}, nil)
	if apiKeySavedQuery.Code != http.StatusUnauthorized {
		t.Fatalf("api-key saved query status = %d, want %d: %s", apiKeySavedQuery.Code, http.StatusUnauthorized, apiKeySavedQuery.Body.String())
	}

	sessionSavedQuery := requestJSON(t, router, http.MethodPost, "/v1/usage/saved-queries", savedQueryPayload, login.Result().Cookies())
	if sessionSavedQuery.Code != http.StatusCreated {
		t.Fatalf("session saved query status = %d, want %d: %s", sessionSavedQuery.Code, http.StatusCreated, sessionSavedQuery.Body.String())
	}
}

func TestIntegrationSQLiteSDKUsageFlow(t *testing.T) {
	runIntegrationSDKUsageFlow(t, config.Config{
		DBDriver:   "sqlite",
		SQLitePath: ":memory:",
		DBPool:     config.DBPoolConfig{MaxOpenConns: 1},
	}, "sqlite")
}

func TestIntegrationPostgresSDKUsageFlow(t *testing.T) {
	dsn := os.Getenv("OPEN_SPANNER_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set OPEN_SPANNER_TEST_POSTGRES_DSN to run Postgres bootstrap integration tests")
	}

	runIntegrationSDKUsageFlow(t, config.Config{
		DBDriver:    "postgres",
		PostgresDSN: dsn,
		DBPool:      config.DBPoolConfig{MaxOpenConns: 1},
	}, "postgres")
}

func runIntegrationSDKUsageFlow(t *testing.T, cfg config.Config, namespace string) {
	t.Helper()

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

	suffix := namespace + "_" + strconv.FormatInt(time.Now().UTC().UnixNano(), 36)
	meterName := "api_calls_" + suffix
	subjectOne := "org_123_" + suffix
	subjectTwo := "org_456_" + suffix

	apiKey := createTestDashboardAPIKey(t, router, "admin+"+suffix+"@example.com")
	authHeaders := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}

	apiKeyCreateKey := requestJSONWithHeaders(t, router, http.MethodPost, "/v1/auth/api-keys", map[string]any{
		"name": "nested-sdk",
	}, authHeaders, nil)
	if apiKeyCreateKey.Code != http.StatusUnauthorized {
		t.Fatalf("api-key create api key status = %d, want %d: %s", apiKeyCreateKey.Code, http.StatusUnauthorized, apiKeyCreateKey.Body.String())
	}

	createMeter := requestJSONWithHeaders(t, router, http.MethodPost, "/v1/meters", map[string]any{
		"name":            meterName,
		"description":     "API calls",
		"unit":            "call",
		"aggregation":     "sum",
		"metadata_schema": map[string]string{"endpoint": "string", "status": "number"},
	}, authHeaders, nil)
	if createMeter.Code != http.StatusCreated {
		t.Fatalf("create meter status = %d, want %d: %s", createMeter.Code, http.StatusCreated, createMeter.Body.String())
	}

	for _, event := range []map[string]any{
		{
			"idempotency_key": "sdk-flow-" + suffix + "-1",
			"subject":         subjectOne,
			"meter":           meterName,
			"quantity":        2,
			"timestamp":       "2026-06-08T10:00:00Z",
			"metadata":        map[string]any{"endpoint": "/orders", "status": 200},
		},
		{
			"idempotency_key": "sdk-flow-" + suffix + "-2",
			"subject":         subjectOne,
			"meter":           meterName,
			"quantity":        3,
			"timestamp":       "2026-06-08T11:00:00Z",
			"metadata":        map[string]any{"endpoint": "/users", "status": 201},
		},
		{
			"idempotency_key": "sdk-flow-" + suffix + "-3",
			"subject":         subjectTwo,
			"meter":           meterName,
			"quantity":        7,
			"timestamp":       "2026-06-08T12:00:00Z",
			"metadata":        map[string]any{"endpoint": "/orders", "status": 200},
		},
	} {
		createUsage := requestJSONWithHeaders(t, router, http.MethodPost, "/v1/usages", event, authHeaders, nil)
		if createUsage.Code != http.StatusCreated {
			t.Fatalf("create usage status = %d, want %d: %s", createUsage.Code, http.StatusCreated, createUsage.Body.String())
		}
	}

	query := url.Values{}
	query.Set("subject", subjectOne)
	query.Set("meter", meterName)
	query.Set("from", "2026-06-08T00:00:00Z")
	query.Set("to", "2026-06-09T00:00:00Z")
	query.Set("bucket_size", "day")
	query.Set("metadata.endpoint", "/orders")
	query.Set("limit", "10")
	bucketsRes := requestJSONWithHeaders(t, router, http.MethodGet, "/v1/usages?"+query.Encode(), nil, authHeaders, nil)
	if bucketsRes.Code != http.StatusOK {
		t.Fatalf("list usages status = %d, want %d: %s", bucketsRes.Code, http.StatusOK, bucketsRes.Body.String())
	}

	var buckets []usageBucketResponse
	decodeJSON(t, bucketsRes, &buckets)
	if len(buckets) != 1 || buckets[0].Quantity != 2 || buckets[0].BucketStart != "2026-06-08T00:00:00Z" {
		t.Fatalf("usage buckets = %#v, want one /orders bucket with quantity 2", buckets)
	}

	searchRes := requestJSONWithHeaders(t, router, http.MethodPost, "/v1/usages/search", map[string]any{
		"meter":       meterName,
		"from":        "2026-06-08T00:00:00Z",
		"to":          "2026-06-09T00:00:00Z",
		"bucket_size": "day",
		"group_by":    []string{"endpoint"},
		"limit":       10,
		"filter": map[string]any{
			"type":  "condition",
			"field": "metadata.status",
			"op":    "eq",
			"value": 200,
		},
	}, authHeaders, nil)
	if searchRes.Code != http.StatusOK {
		t.Fatalf("search usages status = %d, want %d: %s", searchRes.Code, http.StatusOK, searchRes.Body.String())
	}

	var searchedBuckets []usageBucketResponse
	decodeJSON(t, searchRes, &searchedBuckets)
	if len(searchedBuckets) != 1 || searchedBuckets[0].Quantity != 9 || searchedBuckets[0].Group["endpoint"] != "/orders" {
		t.Fatalf("searched usage buckets = %#v, want one /orders bucket with quantity 9", searchedBuckets)
	}

	breakdownRes := requestJSONWithHeaders(t, router, http.MethodPost, "/v1/usages/breakdowns/search", map[string]any{
		"meter": meterName,
		"field": "metadata.endpoint",
		"from":  "2026-06-08T00:00:00Z",
		"to":    "2026-06-09T00:00:00Z",
		"limit": 10,
	}, authHeaders, nil)
	if breakdownRes.Code != http.StatusOK {
		t.Fatalf("breakdown status = %d, want %d: %s", breakdownRes.Code, http.StatusOK, breakdownRes.Body.String())
	}

	var breakdown usageBreakdownListResponse
	decodeJSON(t, breakdownRes, &breakdown)
	if len(breakdown.Items) != 2 || breakdown.Items[0].Value != "/orders" || breakdown.Items[0].Quantity != 9 || breakdown.Items[0].UsageEvents != 2 {
		t.Fatalf("breakdown = %#v, want /orders first with quantity 9", breakdown)
	}

	dimensionsQuery := url.Values{}
	dimensionsQuery.Set("meter", meterName)
	dimensionsQuery.Set("field", "endpoint")
	dimensionsQuery.Set("limit", "10")
	dimensionsRes := requestJSONWithHeaders(t, router, http.MethodGet, "/v1/usages/dimensions?"+dimensionsQuery.Encode(), nil, authHeaders, nil)
	if dimensionsRes.Code != http.StatusOK {
		t.Fatalf("dimension values status = %d, want %d: %s", dimensionsRes.Code, http.StatusOK, dimensionsRes.Body.String())
	}

	var dimensions usageDimensionValueListResponse
	decodeJSON(t, dimensionsRes, &dimensions)
	if len(dimensions.Items) != 2 || dimensions.Items[0].Value == "" {
		t.Fatalf("dimension values = %#v, want discovered endpoint values", dimensions)
	}

	eventsRes := requestJSONWithHeaders(t, router, http.MethodPost, "/v1/usageevents/search", map[string]any{
		"meter": meterName,
		"from":  "2026-06-08T00:00:00Z",
		"to":    "2026-06-09T00:00:00Z",
		"limit": 10,
		"filter": map[string]any{
			"type":  "condition",
			"field": "quantity",
			"op":    "gte",
			"value": 3,
		},
	}, authHeaders, nil)
	if eventsRes.Code != http.StatusOK {
		t.Fatalf("search events status = %d, want %d: %s", eventsRes.Code, http.StatusOK, eventsRes.Body.String())
	}

	var events usageEventListResponse
	decodeJSON(t, eventsRes, &events)
	if len(events.Items) != 2 || events.Items[0].Quantity != 7 || events.Items[1].Quantity != 3 {
		t.Fatalf("searched usage events = %#v, want two events ordered newest first", events)
	}

	ingestionsRes := requestJSONWithHeaders(t, router, http.MethodGet, "/v1/usageingestions?limit=10", nil, authHeaders, nil)
	if ingestionsRes.Code != http.StatusOK {
		t.Fatalf("list ingestions status = %d, want %d: %s", ingestionsRes.Code, http.StatusOK, ingestionsRes.Body.String())
	}

	var ingestions usageIngestionListResponse
	decodeJSON(t, ingestionsRes, &ingestions)
	if len(ingestions.Items) < 3 {
		t.Fatalf("ingestions = %#v, want at least one run per single usage create", ingestions)
	}

	runIntegrationHyphenatedDimensionFlow(t, router, authHeaders, suffix)
	runIntegrationFirstAggregationFlow(t, router, authHeaders, suffix)
	runIntegrationLastAggregationFlow(t, router, authHeaders, suffix)
}

func runIntegrationHyphenatedDimensionFlow(t *testing.T, router http.Handler, authHeaders map[string]string, suffix string) {
	t.Helper()

	const dimensionField = "region-name"
	meterName := "hyphen_dimensions_" + suffix
	subject := "org_hyphen_" + suffix

	createMeter := requestJSONWithHeaders(t, router, http.MethodPost, "/v1/meters", map[string]any{
		"name":            meterName,
		"description":     "Hyphenated dimension keys",
		"unit":            "event",
		"aggregation":     "sum",
		"metadata_schema": map[string]string{dimensionField: "string"},
	}, authHeaders, nil)
	if createMeter.Code != http.StatusCreated {
		t.Fatalf("create hyphen-dimension meter status = %d, want %d: %s", createMeter.Code, http.StatusCreated, createMeter.Body.String())
	}

	createUsage := requestJSONWithHeaders(t, router, http.MethodPost, "/v1/usages", map[string]any{
		"idempotency_key": "hyphen-dimension-" + suffix,
		"subject":         subject,
		"meter":           meterName,
		"quantity":        1,
		"timestamp":       "2026-06-08T10:00:00Z",
		"metadata":        map[string]any{dimensionField: "us-east-1"},
	}, authHeaders, nil)
	if createUsage.Code != http.StatusCreated {
		t.Fatalf("create hyphen-dimension usage status = %d, want %d: %s", createUsage.Code, http.StatusCreated, createUsage.Body.String())
	}

	listQuery := url.Values{}
	listQuery.Set("subject", subject)
	listQuery.Set("meter", meterName)
	listQuery.Set("from", "2026-06-08T00:00:00Z")
	listQuery.Set("to", "2026-06-09T00:00:00Z")
	listQuery.Set("bucket_size", "day")
	listQuery.Set("metadata."+dimensionField, "us-east-1")
	listQuery.Set("limit", "10")
	listRes := requestJSONWithHeaders(t, router, http.MethodGet, "/v1/usages?"+listQuery.Encode(), nil, authHeaders, nil)
	if listRes.Code != http.StatusOK {
		t.Fatalf("list hyphen dimension usage status = %d, want %d: %s", listRes.Code, http.StatusOK, listRes.Body.String())
	}
	var listBuckets []usageBucketResponse
	decodeJSON(t, listRes, &listBuckets)
	if len(listBuckets) != 1 || listBuckets[0].Quantity != 1 {
		t.Fatalf("hyphen dimension list buckets = %#v, want one bucket", listBuckets)
	}

	searchRes := requestJSONWithHeaders(t, router, http.MethodPost, "/v1/usages/search", map[string]any{
		"subject":     subject,
		"meter":       meterName,
		"from":        "2026-06-08T00:00:00Z",
		"to":          "2026-06-09T00:00:00Z",
		"bucket_size": "day",
		"group_by":    []string{dimensionField},
		"limit":       10,
	}, authHeaders, nil)
	if searchRes.Code != http.StatusOK {
		t.Fatalf("search hyphen dimension usage status = %d, want %d: %s", searchRes.Code, http.StatusOK, searchRes.Body.String())
	}
	var groupedBuckets []usageBucketResponse
	decodeJSON(t, searchRes, &groupedBuckets)
	if len(groupedBuckets) != 1 || groupedBuckets[0].Group[dimensionField] != "us-east-1" {
		t.Fatalf("hyphen dimension grouped buckets = %#v, want us-east-1 group", groupedBuckets)
	}

	breakdownRes := requestJSONWithHeaders(t, router, http.MethodPost, "/v1/usages/breakdowns/search", map[string]any{
		"subject": subject,
		"meter":   meterName,
		"field":   "metadata." + dimensionField,
		"from":    "2026-06-08T00:00:00Z",
		"to":      "2026-06-09T00:00:00Z",
		"limit":   10,
	}, authHeaders, nil)
	if breakdownRes.Code != http.StatusOK {
		t.Fatalf("breakdown hyphen dimension status = %d, want %d: %s", breakdownRes.Code, http.StatusOK, breakdownRes.Body.String())
	}
	var breakdown usageBreakdownListResponse
	decodeJSON(t, breakdownRes, &breakdown)
	if len(breakdown.Items) != 1 || breakdown.Items[0].Value != "us-east-1" || breakdown.Items[0].Quantity != 1 {
		t.Fatalf("hyphen dimension breakdown = %#v, want us-east-1 item", breakdown)
	}

	dimensionsQuery := url.Values{}
	dimensionsQuery.Set("subject", subject)
	dimensionsQuery.Set("meter", meterName)
	dimensionsQuery.Set("field", dimensionField)
	dimensionsQuery.Set("limit", "10")
	dimensionsRes := requestJSONWithHeaders(t, router, http.MethodGet, "/v1/usages/dimensions?"+dimensionsQuery.Encode(), nil, authHeaders, nil)
	if dimensionsRes.Code != http.StatusOK {
		t.Fatalf("list hyphen dimension values status = %d, want %d: %s", dimensionsRes.Code, http.StatusOK, dimensionsRes.Body.String())
	}
	var dimensions usageDimensionValueListResponse
	decodeJSON(t, dimensionsRes, &dimensions)
	if len(dimensions.Items) != 1 || dimensions.Items[0].Value != "us-east-1" {
		t.Fatalf("hyphen dimension values = %#v, want us-east-1", dimensions)
	}

	eventsRes := requestJSONWithHeaders(t, router, http.MethodPost, "/v1/usageevents/search", map[string]any{
		"subject": subject,
		"meter":   meterName,
		"from":    "2026-06-08T00:00:00Z",
		"to":      "2026-06-09T00:00:00Z",
		"limit":   10,
		"filter": map[string]any{
			"type":  "condition",
			"field": "metadata." + dimensionField,
			"op":    "contains",
			"value": "us",
		},
	}, authHeaders, nil)
	if eventsRes.Code != http.StatusOK {
		t.Fatalf("search hyphen dimension events status = %d, want %d: %s", eventsRes.Code, http.StatusOK, eventsRes.Body.String())
	}
	var events usageEventListResponse
	decodeJSON(t, eventsRes, &events)
	if len(events.Items) != 1 || events.Items[0].Metadata[dimensionField] != "us-east-1" {
		t.Fatalf("hyphen dimension events = %#v, want matching event", events)
	}
}

func runIntegrationFirstAggregationFlow(t *testing.T, router http.Handler, authHeaders map[string]string, suffix string) {
	t.Helper()

	meterName := "first_aggregation_" + suffix
	subject := "org_first_" + suffix

	createMeter := requestJSONWithHeaders(t, router, http.MethodPost, "/v1/meters", map[string]any{
		"name":            meterName,
		"description":     "First value aggregation",
		"unit":            "event",
		"aggregation":     "first",
		"metadata_schema": map[string]string{"endpoint": "string"},
	}, authHeaders, nil)
	if createMeter.Code != http.StatusCreated {
		t.Fatalf("create first-aggregation meter status = %d, want %d: %s", createMeter.Code, http.StatusCreated, createMeter.Body.String())
	}

	for _, event := range []map[string]any{
		{
			"idempotency_key": "first-aggregation-" + suffix + "-later",
			"subject":         subject,
			"meter":           meterName,
			"quantity":        12,
			"timestamp":       "2026-06-08T11:00:00Z",
			"metadata":        map[string]any{"endpoint": "/orders"},
		},
		{
			"idempotency_key": "first-aggregation-" + suffix + "-earlier",
			"subject":         subject,
			"meter":           meterName,
			"quantity":        7,
			"timestamp":       "2026-06-08T09:00:00Z",
			"metadata":        map[string]any{"endpoint": "/orders"},
		},
		{
			"idempotency_key": "first-aggregation-" + suffix + "-users",
			"subject":         subject,
			"meter":           meterName,
			"quantity":        4,
			"timestamp":       "2026-06-08T10:00:00Z",
			"metadata":        map[string]any{"endpoint": "/users"},
		},
	} {
		createUsage := requestJSONWithHeaders(t, router, http.MethodPost, "/v1/usages", event, authHeaders, nil)
		if createUsage.Code != http.StatusCreated {
			t.Fatalf("create first-aggregation usage status = %d, want %d: %s", createUsage.Code, http.StatusCreated, createUsage.Body.String())
		}
	}

	searchRes := requestJSONWithHeaders(t, router, http.MethodPost, "/v1/usages/search", map[string]any{
		"subject":     subject,
		"meter":       meterName,
		"from":        "2026-06-08T00:00:00Z",
		"to":          "2026-06-09T00:00:00Z",
		"bucket_size": "day",
		"group_by":    []string{"endpoint"},
		"limit":       10,
	}, authHeaders, nil)
	if searchRes.Code != http.StatusOK {
		t.Fatalf("search first-aggregation usage status = %d, want %d: %s", searchRes.Code, http.StatusOK, searchRes.Body.String())
	}
	var buckets []usageBucketResponse
	decodeJSON(t, searchRes, &buckets)
	if len(buckets) != 2 {
		t.Fatalf("first-aggregation buckets = %#v, want two endpoint groups", buckets)
	}
	if buckets[0].Group["endpoint"] != "/orders" || buckets[0].Quantity != 7 {
		t.Fatalf("first-aggregation /orders bucket = %#v, want earliest quantity 7", buckets[0])
	}
	if buckets[1].Group["endpoint"] != "/users" || buckets[1].Quantity != 4 {
		t.Fatalf("first-aggregation /users bucket = %#v, want quantity 4", buckets[1])
	}
}

func runIntegrationLastAggregationFlow(t *testing.T, router http.Handler, authHeaders map[string]string, suffix string) {
	t.Helper()

	meterName := "last_aggregation_" + suffix
	subject := "org_last_" + suffix

	createMeter := requestJSONWithHeaders(t, router, http.MethodPost, "/v1/meters", map[string]any{
		"name":            meterName,
		"description":     "Last value aggregation",
		"unit":            "event",
		"aggregation":     "last",
		"metadata_schema": map[string]string{"endpoint": "string"},
	}, authHeaders, nil)
	if createMeter.Code != http.StatusCreated {
		t.Fatalf("create last-aggregation meter status = %d, want %d: %s", createMeter.Code, http.StatusCreated, createMeter.Body.String())
	}

	for _, event := range []map[string]any{
		{
			"idempotency_key": "last-aggregation-" + suffix + "-earlier",
			"subject":         subject,
			"meter":           meterName,
			"quantity":        7,
			"timestamp":       "2026-06-08T09:00:00Z",
			"metadata":        map[string]any{"endpoint": "/orders"},
		},
		{
			"idempotency_key": "last-aggregation-" + suffix + "-users",
			"subject":         subject,
			"meter":           meterName,
			"quantity":        4,
			"timestamp":       "2026-06-08T10:00:00Z",
			"metadata":        map[string]any{"endpoint": "/users"},
		},
		{
			"idempotency_key": "last-aggregation-" + suffix + "-later",
			"subject":         subject,
			"meter":           meterName,
			"quantity":        12,
			"timestamp":       "2026-06-08T11:00:00Z",
			"metadata":        map[string]any{"endpoint": "/orders"},
		},
	} {
		createUsage := requestJSONWithHeaders(t, router, http.MethodPost, "/v1/usages", event, authHeaders, nil)
		if createUsage.Code != http.StatusCreated {
			t.Fatalf("create last-aggregation usage status = %d, want %d: %s", createUsage.Code, http.StatusCreated, createUsage.Body.String())
		}
	}

	searchRes := requestJSONWithHeaders(t, router, http.MethodPost, "/v1/usages/search", map[string]any{
		"subject":     subject,
		"meter":       meterName,
		"from":        "2026-06-08T00:00:00Z",
		"to":          "2026-06-09T00:00:00Z",
		"bucket_size": "day",
		"group_by":    []string{"endpoint"},
		"limit":       10,
	}, authHeaders, nil)
	if searchRes.Code != http.StatusOK {
		t.Fatalf("search last-aggregation usage status = %d, want %d: %s", searchRes.Code, http.StatusOK, searchRes.Body.String())
	}
	var buckets []usageBucketResponse
	decodeJSON(t, searchRes, &buckets)
	if len(buckets) != 2 {
		t.Fatalf("last-aggregation buckets = %#v, want two endpoint groups", buckets)
	}
	if buckets[0].Group["endpoint"] != "/orders" || buckets[0].Quantity != 12 {
		t.Fatalf("last-aggregation /orders bucket = %#v, want latest quantity 12", buckets[0])
	}
	if buckets[1].Group["endpoint"] != "/users" || buckets[1].Quantity != 4 {
		t.Fatalf("last-aggregation /users bucket = %#v, want quantity 4", buckets[1])
	}
}

func requestJSON(t *testing.T, handler http.Handler, method string, path string, body any, cookies []*http.Cookie) *httptest.ResponseRecorder {
	return requestJSONWithHeaders(t, handler, method, path, body, nil, cookies)
}

func requestJSONWithHeaders(t *testing.T, handler http.Handler, method string, path string, body any, headers map[string]string, cookies []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	var payload bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&payload).Encode(body); err != nil {
			t.Fatalf("encode request body: %v", err)
		}
	}

	req := httptest.NewRequest(method, path, &payload)
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	return res
}

func createTestDashboardAPIKey(t *testing.T, handler http.Handler, email string) string {
	t.Helper()

	createUser := requestJSON(t, handler, http.MethodPost, "/v1/auth/users", map[string]any{
		"email":    email,
		"password": "strong-password",
	}, nil)
	if createUser.Code != http.StatusCreated {
		t.Fatalf("create user status = %d, want %d: %s", createUser.Code, http.StatusCreated, createUser.Body.String())
	}

	login := requestJSON(t, handler, http.MethodPost, "/v1/auth/sessions", map[string]any{
		"email":    email,
		"password": "strong-password",
	}, nil)
	if login.Code != http.StatusCreated {
		t.Fatalf("login status = %d, want %d: %s", login.Code, http.StatusCreated, login.Body.String())
	}

	createKey := requestJSON(t, handler, http.MethodPost, "/v1/auth/api-keys", map[string]any{
		"name": "integration-sdk",
	}, login.Result().Cookies())
	if createKey.Code != http.StatusCreated {
		t.Fatalf("create api key status = %d, want %d: %s", createKey.Code, http.StatusCreated, createKey.Body.String())
	}

	var key struct {
		Key string `json:"key"`
	}
	decodeJSON(t, createKey, &key)
	if key.Key == "" {
		t.Fatal("api key is empty")
	}
	return key.Key
}

func decodeJSON(t *testing.T, res *httptest.ResponseRecorder, target any) {
	t.Helper()

	if err := json.NewDecoder(res.Body).Decode(target); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
}

type usageBucketResponse struct {
	Subject     string            `json:"subject"`
	Meter       string            `json:"meter"`
	BucketSize  string            `json:"bucket_size"`
	BucketStart string            `json:"bucket_start"`
	Aggregation string            `json:"aggregation"`
	Unit        string            `json:"unit"`
	Quantity    float64           `json:"quantity"`
	Group       map[string]string `json:"group"`
}

type usageBreakdownResponse struct {
	Field       string  `json:"field"`
	Value       string  `json:"value"`
	Quantity    float64 `json:"quantity"`
	UsageEvents int     `json:"events"`
	Aggregation string  `json:"aggregation"`
	Unit        string  `json:"unit"`
}

type usageBreakdownListResponse struct {
	Items []usageBreakdownResponse `json:"items"`
}

type usageDimensionValueResponse struct {
	Field       string `json:"field"`
	Value       string `json:"value"`
	UsageEvents int    `json:"events"`
}

type usageDimensionValueListResponse struct {
	Items []usageDimensionValueResponse `json:"items"`
}

type usageEventResponse struct {
	ID             string         `json:"id"`
	IdempotencyKey string         `json:"idempotency_key"`
	Subject        string         `json:"subject"`
	Meter          string         `json:"meter"`
	Quantity       float64        `json:"quantity"`
	Timestamp      string         `json:"timestamp"`
	ReceivedAt     string         `json:"received_at"`
	Metadata       map[string]any `json:"metadata"`
}

type usageEventListResponse struct {
	Items      []usageEventResponse `json:"items"`
	NextCursor string               `json:"next_cursor"`
}

type usageIngestionResponse struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Accepted   int    `json:"accepted"`
	Duplicates int    `json:"duplicates"`
	Failed     int    `json:"failed"`
	CreatedAt  string `json:"created_at"`
}

type usageIngestionListResponse struct {
	Items      []usageIngestionResponse `json:"items"`
	NextCursor string                   `json:"next_cursor"`
}
