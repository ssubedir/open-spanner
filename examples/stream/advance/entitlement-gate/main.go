package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"

	restclient "github.com/ssubedir/open-spanner/sdk/go/client"
	"github.com/ssubedir/open-spanner/sdk/go/client/plans"
	"github.com/ssubedir/open-spanner/sdk/go/models"
	"github.com/ssubedir/open-spanner/sdk/go/stream"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	apiKey := mustEnv("OPEN_SPANNER_API_KEY")
	meterName := env("OPEN_SPANNER_GRPC_METER", "stream_api_calls")
	subject := env("OPEN_SPANNER_SUBJECT", "org_123")
	quantity := envFloat("OPEN_SPANNER_QUANTITY", 1)

	api := newRESTClient(env("OPEN_SPANNER_BASE_URL", "http://localhost:18081"), apiKey)
	check, err := api.Plans.CheckEntitlement(plans.NewCheckEntitlementParams().WithRequest(&models.EntitlementCheckRequest{
		Subject:  subject,
		Meter:    meterName,
		Quantity: quantity,
	}))
	if err != nil {
		panic(err)
	}

	entitlement := check.Payload
	fmt.Printf("%s on %s: allowed=%t state=%s remaining=%.0f\n", entitlement.Subject, entitlement.PlanName, entitlement.Allowed, entitlement.State, entitlement.Remaining)
	if !entitlement.Allowed {
		return
	}

	streamClient, err := stream.NewClient(env("OPEN_SPANNER_GRPC_ADDR", "localhost:18090"), apiKey)
	if err != nil {
		panic(err)
	}
	defer streamClient.Close()

	now := time.Now().UTC()
	recorded, err := streamClient.Track(ctx, stream.Event{
		IdempotencyKey: fmt.Sprintf("entitlement-gate-%s-%d", subject, now.UnixNano()),
		Subject:        subject,
		Meter:          meterName,
		Quantity:       quantity,
		Timestamp:      now,
		Metadata: map[string]any{
			"endpoint": "/v1/messages",
			"region":   "us-east",
			"source":   "entitlement-gate-example",
		},
	})
	if err != nil {
		panic(err)
	}

	fmt.Printf("meter=%s recorded_event=%s\n", meterName, recorded.ID)
}

func newRESTClient(baseURL string, apiKey string) *restclient.OpenSpanner {
	host, schemes, err := transportParts(baseURL)
	if err != nil {
		panic(err)
	}
	transport := httptransport.New(host, restclient.DefaultBasePath, schemes)
	transport.DefaultAuthentication = httptransport.BearerToken(apiKey)
	return restclient.New(transport, strfmt.Default)
}

func env(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func mustEnv(key string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		panic(key + " is required")
	}
	return value
}

func envFloat(key string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
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
