package main

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

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

	api := client.NewHTTPClientWithConfig(nil, client.DefaultTransportConfig().
		WithHost(host).
		WithSchemes(schemes))

	now := time.Now().UTC()
	meterName := fmt.Sprintf("sdk_go_requests_%d", now.Unix())
	subject := "org_sdk_go"

	schema := map[string]string{"plan": "string", "region": "string"}
	meterRes, err := api.Meters.CreateMeter(meters.NewCreateMeterParams().WithRequest(&models.MeterCreateRequest{
		Name:               meterName,
		Description:        "Go SDK example request counter",
		Unit:               "request",
		Aggregation:        "sum",
		MetadataSchema:     schema,
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
			"plan":   "pro",
			"region": "us-east",
		},
	}))
	if err != nil {
		panic(err)
	}

	fmt.Printf("created meter: %s (%s)\n", meterRes.Payload.Name, meterRes.Payload.ID)
	fmt.Printf("recorded usage: %s quantity=%.2f\n", usageRes.Payload.ID, usageRes.Payload.Quantity)
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
