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
	meterName := fmt.Sprintf("sdk_go_tokens_used_%d", runID)

	_, err = api.Meters.CreateMeter(meters.NewCreateMeterParams().WithRequest(&models.MeterCreateRequest{
		Name:               meterName,
		Description:        "Track model token consumption by provider, model, operation, and cache path",
		Unit:               "token",
		Aggregation:        "sum",
		EventRetentionDays: 90,
		Dimensions: []*models.MeterDimensionRequest{
			{Name: "model", DisplayName: "Model", Description: "Model identifier", Type: "string", Required: true},
			{Name: "provider", DisplayName: "Provider", Description: "AI provider", Type: "string", Required: true},
			{Name: "operation", DisplayName: "Operation", Description: "Completion, embedding, or rerank", Type: "string", Required: true},
			{Name: "cached", DisplayName: "Cached", Description: "Whether cached context was used", Type: "boolean", Required: false},
		},
	}))
	if err != nil {
		panic(err)
	}

	events := []usageEvent{
		{"org_acme", 24800, map[string]any{"model": "gpt-4.1", "provider": "openai", "operation": "completion", "cached": false}},
		{"org_acme", 13200, map[string]any{"model": "text-embedding-3-large", "provider": "openai", "operation": "embedding", "cached": true}},
		{"org_globex", 4100, map[string]any{"model": "claude-3-5-sonnet", "provider": "anthropic", "operation": "completion", "cached": false}},
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

	fmt.Printf("seeded AI token meter %s with %d events\n", meterName, len(events))
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
