package main

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"

	"github.com/ssubedir/open-spanner/sdk/go/client"
	"github.com/ssubedir/open-spanner/sdk/go/client/meters"
	"github.com/ssubedir/open-spanner/sdk/go/client/usages"
	"github.com/ssubedir/open-spanner/sdk/go/models"
)

type usageEvent struct {
	subject  string
	quantity float64
	metadata map[string]any
}

func main() {
	baseURL := env("OPEN_SPANNER_BASE_URL", "http://localhost:18081")
	host, schemes, err := transportParts(baseURL)
	if err != nil {
		panic(err)
	}
	apiKey := env("OPEN_SPANNER_API_KEY", "osp_...")
	transport := httptransport.New(host, client.DefaultBasePath, schemes)
	transport.DefaultAuthentication = httptransport.BearerToken(apiKey)
	api := client.New(transport, strfmt.Default)

	now := time.Now().UTC()
	runID := now.UnixNano() / int64(time.Millisecond)
	meterName := fmt.Sprintf("sdk_go_api_requests_%d", runID)

	_, err = api.Meters.CreateMeter(meters.NewCreateMeterParams().WithRequest(&models.MeterCreateRequest{
		Name:               meterName,
		Description:        "Track request volume by endpoint, method, status, region, and service tier",
		Unit:               "request",
		Aggregation:        "sum",
		EventRetentionDays: 90,
		Dimensions: []*models.MeterDimensionRequest{
			{Name: "endpoint", DisplayName: "Endpoint", Description: "Route or operation", Type: "string", Required: true},
			{Name: "method", DisplayName: "Method", Description: "HTTP method", Type: "string", Required: true},
			{Name: "status_code", DisplayName: "Status code", Description: "HTTP status code", Type: "number", Required: true},
			{Name: "region-name", DisplayName: "Region", Description: "Serving region", Type: "string", Required: false},
			{Name: "service.tier", DisplayName: "Service tier", Description: "Backend service tier", Type: "string", Required: true},
		},
	}))
	if err != nil {
		panic(err)
	}

	events := []usageEvent{
		{"org_acme", 38, map[string]any{"endpoint": "/v1/orders", "method": "POST", "status_code": 201, "region-name": "us-east", "service": map[string]any{"tier": "gold"}}},
		{"org_acme", 91, map[string]any{"endpoint": "/v1/orders", "method": "GET", "status_code": 200, "region-name": "us-east", "service": map[string]any{"tier": "gold"}}},
		{"org_globex", 14, map[string]any{"endpoint": "/v1/invoices", "method": "GET", "status_code": 200, "region-name": "eu-west", "service": map[string]any{"tier": "silver"}}},
	}

	for index, event := range events {
		_, err := api.Usages.CreateUsage(usages.NewCreateUsageParams().WithRequest(&models.UsageCreateRequest{
			IdempotencyKey: fmt.Sprintf("%s-%d-%d", meterName, index, runID),
			Subject:        event.subject,
			Meter:          meterName,
			Quantity:       event.quantity,
			Timestamp:      now.Add(time.Duration(index) * time.Minute).Format(time.RFC3339),
			Metadata:       event.metadata,
		}))
		if err != nil {
			panic(err)
		}
	}

	fmt.Printf("seeded API request meter %s with %d events\n", meterName, len(events))
}

func env(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func transportParts(baseURL string) (string, []string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", nil, err
	}
	scheme := parsed.Scheme
	if scheme == "" {
		scheme = "http"
	}
	host := parsed.Host
	if host == "" {
		host = strings.TrimPrefix(strings.TrimPrefix(baseURL, "http://"), "https://")
	}
	return host, []string{scheme}, nil
}
