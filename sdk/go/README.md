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
	"fmt"
	"time"

	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"

	"github.com/ssubedir/open-spanner/sdk/go/client"
	"github.com/ssubedir/open-spanner/sdk/go/client/usages"
	"github.com/ssubedir/open-spanner/sdk/go/models"
)

func main() {
	apiKey := "..."

	cfg := client.DefaultTransportConfig().
		WithHost("api.example.com").
		WithSchemes([]string{"https"})

	transport := httptransport.New(cfg.Host, cfg.BasePath, cfg.Schemes)
	transport.DefaultAuthentication = httptransport.BearerToken(apiKey)
	api := client.New(transport, strfmt.Default)

	usage, err := api.Usages.CreateUsage(usages.NewCreateUsageParams().WithRequest(&models.UsageCreateRequest{
		IdempotencyKey: fmt.Sprintf("api_requests-%d", time.Now().UnixNano()),
		Subject:        "org_123",
		Meter:          "api_requests",
		Quantity:       1,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
	}))
	if err != nil {
		panic(err)
	}

	fmt.Println(usage.Payload.ID)
}
```

Types and clients are generated from `../../openapi/sdk-swagger.json` with `go-swagger`.
