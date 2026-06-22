package main

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"

	"github.com/ssubedir/open-spanner/sdk/go/client"
	"github.com/ssubedir/open-spanner/sdk/go/client/plans"
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

	meterName := env("OPEN_SPANNER_METER", "api_calls")
	subject := env("OPEN_SPANNER_SUBJECT", "org_123")
	quantity := envFloat("OPEN_SPANNER_QUANTITY", 1)

	check, err := api.Plans.CheckEntitlement(plans.NewCheckEntitlementParams().WithRequest(&models.EntitlementCheckRequest{
		Subject:  subject,
		Meter:    meterName,
		Quantity: quantity,
	}))
	if err != nil {
		panic(err)
	}

	result := check.Payload
	fmt.Printf("%s on %s: allowed=%t state=%s remaining=%.0f\n", result.Subject, result.PlanName, result.Allowed, result.State, result.Remaining)
	if !result.Allowed {
		return
	}

	_, err = api.Usages.CreateUsage(usages.NewCreateUsageParams().WithRequest(&models.UsageCreateRequest{
		IdempotencyKey: fmt.Sprintf("entitlement-check-%s-%d", subject, time.Now().UnixNano()),
		Subject:        subject,
		Meter:          meterName,
		Quantity:       quantity,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		Metadata:       map[string]any{"source": "entitlement-check-example"},
	}))
	if err != nil {
		panic(err)
	}

	fmt.Println("usage accepted")
}

func env(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func envFloat(key string, fallback float64) float64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		panic(err)
	}
	return parsed
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

