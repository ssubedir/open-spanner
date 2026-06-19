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
	subject       string
	quantity      float64
	offsetMinutes int
	metadata      map[string]any
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
	meterName := fmt.Sprintf("sdk_go_billing_events_backfilled_%d", runID)

	_, err = api.Meters.CreateMeter(meters.NewCreateMeterParams().WithRequest(&models.MeterCreateRequest{
		Name:               meterName,
		Description:        "Import historical billing events with stable idempotency keys",
		Unit:               "event",
		Aggregation:        "sum",
		EventRetentionDays: 90,
		Dimensions: []*models.MeterDimensionRequest{
			{Name: "source", DisplayName: "Source", Description: "Imported source system", Type: "string", Required: true},
			{Name: "event_type", DisplayName: "Event type", Description: "Imported billing event type", Type: "string", Required: true},
			{Name: "import_batch", DisplayName: "Import batch", Description: "Backfill batch identifier", Type: "string", Required: true},
		},
	}))
	if err != nil {
		panic(err)
	}

	events := []usageEvent{
		{"org_acme", 340, -1440, map[string]any{"source": "legacy-billing", "event_type": "api_request", "import_batch": "batch-2026-06"}},
		{"org_globex", 112, -720, map[string]any{"source": "legacy-billing", "event_type": "storage", "import_batch": "batch-2026-06"}},
		{"org_initech", 64, -60, map[string]any{"source": "csv-import", "event_type": "feature_use", "import_batch": "batch-2026-06"}},
	}

	for index, event := range events {
		_, err := api.Usages.CreateUsage(usages.NewCreateUsageParams().WithRequest(&models.UsageCreateRequest{
			IdempotencyKey: fmt.Sprintf("%s-%v-%s-%d", meterName, event.metadata["import_batch"], event.subject, index),
			Subject:        event.subject,
			Meter:          meterName,
			Quantity:       event.quantity,
			Timestamp:      now.Add(time.Duration(event.offsetMinutes) * time.Minute).Format(time.RFC3339),
			Metadata:       event.metadata,
		}))
		if err != nil {
			panic(err)
		}
	}

	fmt.Printf("seeded historical backfill meter %s with %d events\n", meterName, len(events))
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
