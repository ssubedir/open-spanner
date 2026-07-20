# Open Spanner Go SDK

Generated Go client for the Open Spanner API.

Install:

```sh
go get github.com/ssubedir/open-spanner/sdk/go
```

Record usage for a meter that already exists:

```go
package main

import (
	"context"
	"fmt"
	"time"

	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"

	"github.com/ssubedir/open-spanner/sdk/go/client"
	"github.com/ssubedir/open-spanner/sdk/go/client/usages"
	"github.com/ssubedir/open-spanner/sdk/go/models"
	"github.com/ssubedir/open-spanner/sdk/go/retry"
)

func main() {
	apiKey := "..."

	cfg := client.DefaultTransportConfig().
		WithHost("api.example.com").
		WithSchemes([]string{"https"})

	transport := httptransport.New(cfg.Host, cfg.BasePath, cfg.Schemes)
	transport.DefaultAuthentication = httptransport.BearerToken(apiKey)
	api := client.New(transport, strfmt.Default)

	request := usages.NewCreateUsageParams().WithRequest(&models.UsageCreateRequest{
		IdempotencyKey: fmt.Sprintf("api_requests-%d", time.Now().UnixNano()), Subject: "org_123",
		Meter: "api_requests", Quantity: 1, Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
	usage, err := retry.Do(context.Background(), retry.DefaultPolicy(), func() (*usages.CreateUsageCreated, error) {
		return api.Usages.CreateUsage(request)
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(usage.Payload.ID)
}
```

Types and clients are generated from `../../openapi/sdk-swagger.json` with `go-swagger`.

## gRPC Ingestion

The SDK also includes a small gRPC ingestion client for backend services that continuously emit usage:

Runnable example: [`examples/stream/basic/go`](../../examples/stream/basic/go).

```go
package main

import (
	"context"
	"time"

	"github.com/ssubedir/open-spanner/sdk/go/stream"
)

func main() {
	client, err := stream.NewClient(
		"localhost:18090",
		"osp_...",
		stream.WithRetryPolicy(stream.DefaultRetryPolicy()),
	)
	if err != nil {
		panic(err)
	}
	defer client.Close()

	result, err := client.TrackBulk(context.Background(), "batch-1", []stream.Event{
		{
			IdempotencyKey: "usage-1",
			Subject:        "org_123",
			Meter:          "api_requests",
			Quantity:       1,
			Timestamp:      time.Now().UTC(),
			Metadata:       map[string]any{"endpoint": "/v1/orders", "status": 200},
		},
	})
	if err != nil {
		panic(err)
	}

	_ = result.AcceptedCount
}
```

Use `stream.WithTransportCredentials(...)` when connecting to a TLS-enabled gRPC endpoint.

The retry policy applies only to unary `Track` and `TrackBulk` calls. It retries overload, temporary unavailability, and deadline failures, honors `google.rpc.RetryInfo`, preserves the original request, and reports each retry through `RetryPolicy.OnRetry`. Client streams are not replayed automatically because the server may have accepted an unknown prefix; retry those events with their original idempotency keys through `TrackBulk`.
