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
	meterName := fmt.Sprintf("sdk_go_requests_%d", now.Unix())
	subject := "org_sdk_go"

	meterRes, err := api.Meters.CreateMeter(meters.NewCreateMeterParams().WithRequest(&models.MeterCreateRequest{
		Name:        meterName,
		Description: "Go SDK example request counter",
		Unit:        "request",
		Aggregation: "sum",
		Dimensions: []*models.MeterDimensionRequest{
			{
				Name:        "endpoint",
				DisplayName: "Endpoint",
				Description: "API route that handled the request",
				Type:        "string",
				Required:    true,
			},
			{
				Name:        "status",
				DisplayName: "HTTP status",
				Description: "Response status code",
				Type:        "number",
				Required:    true,
			},
			{
				Name:        "region",
				DisplayName: "Region",
				Description: "Serving region",
				Type:        "string",
				Required:    false,
			},
		},
		EventRetentionDays: 30,
	}))
	if err != nil {
		panic(err)
	}

	usageRes, err := api.Usages.CreateUsage(usages.NewCreateUsageParams().WithRequest(&models.UsageCreateRequest{
		IdempotencyKey: fmt.Sprintf("%s-%d", meterName, now.UnixNano()),
		Subject:        subject,
		Meter:          meterName,
		Quantity:       42,
		Timestamp:      now.Format(time.RFC3339),
		Metadata: map[string]any{
			"endpoint": "/v1/orders",
			"status":   200,
			"region":   "us-east",
			"trace_id": "trace-go-example",
		},
	}))
	if err != nil {
		panic(err)
	}

	_, validationErr := api.Usages.CreateUsage(usages.NewCreateUsageParams().WithRequest(&models.UsageCreateRequest{
		IdempotencyKey: fmt.Sprintf("%s-invalid-%d", meterName, now.UnixNano()),
		Subject:        subject,
		Meter:          meterName,
		Quantity:       1,
		Timestamp:      now.Format(time.RFC3339),
		Metadata: map[string]any{
			"endpoint": "/v1/orders",
			"status":   "200",
		},
	}))
	if validationErr == nil {
		panic("expected dimension validation error")
	}

	fmt.Printf("created meter: %s (%s)\n", meterRes.Payload.Name, meterRes.Payload.ID)
	fmt.Printf("recorded usage: %s quantity=%.2f\n", usageRes.Payload.ID, usageRes.Payload.Quantity)
	fmt.Printf("dimension validation rejected invalid usage: %v\n", validationErr)
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
