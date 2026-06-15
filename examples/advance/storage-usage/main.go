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
	meterName := fmt.Sprintf("sdk_go_storage_gb_hours_%d", runID)

	_, err = api.Meters.CreateMeter(meters.NewCreateMeterParams().WithRequest(&models.MeterCreateRequest{
		Name:               meterName,
		Description:        "Track storage consumption by tier, region, and resource type",
		Unit:               "gb_hour",
		Aggregation:        "sum",
		EventRetentionDays: 90,
		Dimensions: []*models.MeterDimensionRequest{
			{Name: "tier", DisplayName: "Tier", Description: "Storage tier", Type: "string", Required: true},
			{Name: "region", DisplayName: "Region", Description: "Storage region", Type: "string", Required: true},
			{Name: "resource_type", DisplayName: "Resource type", Description: "Stored resource type", Type: "string", Required: true},
		},
	}))
	if err != nil {
		panic(err)
	}

	events := []usageEvent{
		{"org_acme", 512.5, map[string]any{"tier": "hot", "region": "us-east", "resource_type": "object"}},
		{"org_acme", 128, map[string]any{"tier": "archive", "region": "us-east", "resource_type": "backup"}},
		{"org_globex", 74.25, map[string]any{"tier": "hot", "region": "eu-west", "resource_type": "object"}},
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

	fmt.Printf("seeded storage usage meter %s with %d events\n", meterName, len(events))
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
