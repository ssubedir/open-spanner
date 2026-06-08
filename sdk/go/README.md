# Open Spanner Go SDK

Generated Go client for the Open Spanner API.

Regenerate from the repository root:

```powershell
task sdk:go
```

Run SDK tests:

```powershell
task sdk:go:test
```

Example:

```go
package main

import (
	"fmt"
	"time"

	"github.com/ssubedir/open-spanner/sdk/go/client"
	"github.com/ssubedir/open-spanner/sdk/go/client/meters"
	"github.com/ssubedir/open-spanner/sdk/go/client/usages"
	"github.com/ssubedir/open-spanner/sdk/go/models"
)

func main() {
	cfg := client.DefaultTransportConfig().
		WithHost("api.example.com").
		WithSchemes([]string{"https"})

	api := client.NewHTTPClientWithConfig(nil, cfg)

	meter, err := api.Meters.CreateMeter(meters.NewCreateMeterParams().WithRequest(&models.MeterCreateRequest{
		Name:               "api_requests",
		Description:        "API request counter",
		Unit:               "request",
		Aggregation:        "sum",
		EventRetentionDays: 30,
	}))
	if err != nil {
		panic(err)
	}

	usage, err := api.Usages.CreateUsage(usages.NewCreateUsageParams().WithRequest(&models.UsageCreateRequest{
		IdempotencyKey: fmt.Sprintf("api_requests-%d", time.Now().UnixNano()),
		Subject:        "org_123",
		Meter:          meter.Payload.Name,
		Quantity:       1,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
	}))
	if err != nil {
		panic(err)
	}

	fmt.Println(meter.Payload.ID, usage.Payload.ID)
}
```
