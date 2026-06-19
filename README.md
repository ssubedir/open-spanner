# Open Spanner

Open Spanner is an open-source metering service for usage-based products. It records usage from your backend services, validates that usage against meter definitions, and turns raw events into queryable buckets for billing, limits, reporting, operations, and audits.

Use it when you need to answer questions like:

- How many API requests did a customer make this month?
- Which model, region, or plan produced the most usage?
- Did a customer cross a usage threshold?
- What usage should be exported into billing, finance, or analytics?
- Can we replay or retry usage writes without double-counting?

Open Spanner is not a payment processor, invoice generator, entitlement system, or customer identity provider. It gives those systems a clean usage record to work with.

## Product Surfaces

| Surface | Use it for |
| --- | --- |
| Dashboard | Sign in, define meters, inspect usage, create API keys, manage exports, and view alert activity. |
| REST API | Meter management, usage writes, usage queries, exports, and operational endpoints. |
| Official SDKs | Typed backend clients for REST operations in Go, TypeScript, Python, and C#. |
| gRPC streaming | High-throughput usage ingestion from trusted backend services. |
| Workers | Queued CSV export processing and alert threshold evaluation. |
| Storage | SQLite for local/single-node use, Postgres for production deployments. |

Read the hosted docs at [ssubedir.github.io/open-spanner/docs](https://ssubedir.github.io/open-spanner/docs).

## Features

- Meter definitions with units, aggregation mode, retention policy, and typed dimensions.
- Idempotent single and bulk usage ingestion.
- gRPC stream ingestion for backend service-to-service usage pipelines.
- Bucketed usage queries with filters, breakdowns, dimensions, and pagination.
- Direct CSV exports for focused requests and queued export jobs for larger files.
- Alert rules that watch usage windows and deliver webhook notifications.
- Dashboard auth with HttpOnly cookies and API keys for service clients.
- SQLite and Postgres storage, including Postgres JSONB metadata filtering.
- Embedded React dashboard and Swagger UI.
- Generated REST SDKs for Go, TypeScript, Python, and C#.
- Go stream SDK for gRPC usage ingestion.

## Quick Start

The fastest full stack is Docker Compose. It starts the API, dashboard, export worker, alert worker, Postgres, and shared export storage.

```sh
git clone https://github.com/ssubedir/open-spanner.git
cd open-spanner
docker compose -f docker-compose.app.yml up -d --build
```

Open the dashboard:

```text
http://localhost:18081/register
```

Useful local endpoints:

| Endpoint | Purpose |
| --- | --- |
| `http://localhost:18081/login` | Dashboard login |
| `http://localhost:18081/docs` | Swagger UI |
| `http://localhost:18081/health` | Liveness |
| `http://localhost:18081/ready` | Readiness |
| `localhost:18090` | gRPC usage ingestion |

Stop the stack:

```sh
docker compose -f docker-compose.app.yml down
```

Remove Postgres data too:

```sh
docker compose -f docker-compose.app.yml down -v
```

## Docker Image

Release images are published to Docker Hub:

```sh
docker pull ssubedir/open-spanner:latest
```

Use `latest` for trials. Pin a version tag for production:

```sh
docker pull ssubedir/open-spanner:0.1.8
```

The image includes the API and worker binaries:

```text
/usr/local/bin/open-spanner
/usr/local/bin/open-spanner-export-worker
/usr/local/bin/open-spanner-alert-worker
```

For a small SQLite-backed trial:

```sh
docker volume create open-spanner-data

docker run --detach \
  --name open-spanner \
  --publish 18081:18081 \
  --publish 18090:18090 \
  --volume open-spanner-data:/data \
  ssubedir/open-spanner:latest
```

## From Source

Install [Task](https://taskfile.dev/) and run the API with SQLite:

```sh
task run:sqlite
```

Run workers in separate terminals when you want queued exports and alerts processed:

```sh
task run:export-worker
task run:alert-worker
```

Run with Postgres:

```sh
task postgres:up
task run:postgres
task run:export-worker:postgres
task run:alert-worker:postgres
```

## First Usage Flow

Create a dashboard user, then create an API key from the API Keys page. Copy the key when it is created; the full key is not shown again.

```sh
API_KEY="osp_..."
```

Create a meter:

```sh
curl -X POST http://localhost:18081/v1/meters \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "api_requests",
    "description": "API requests served by the product API",
    "unit": "request",
    "aggregation": "sum",
    "event_retention_days": 90,
    "dimensions": [
      { "name": "endpoint", "type": "string", "required": true },
      { "name": "status", "type": "number", "required": true },
      { "name": "region", "type": "string" }
    ]
  }'
```

Record usage:

```sh
curl -X POST http://localhost:18081/v1/usages \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "idempotency_key": "usage_001",
    "subject": "org_123",
    "meter": "api_requests",
    "quantity": 1,
    "metadata": {
      "endpoint": "/checkout",
      "status": 200,
      "region": "us-east"
    }
  }'
```

Query usage buckets:

```sh
curl "http://localhost:18081/v1/usages?subject=org_123&meter=api_requests&bucket_size=day&metadata.endpoint=/checkout" \
  -H "Authorization: Bearer $API_KEY"
```

## gRPC Streaming

Use REST or the dashboard for setup operations such as API keys and meters. Use gRPC streaming when a trusted backend service continuously emits usage.

```go
package main

import (
	"context"
	"time"

	"github.com/ssubedir/open-spanner/sdk/go/stream"
)

func main() {
	client, err := stream.NewClient("localhost:18090", "osp_...")
	if err != nil {
		panic(err)
	}
	defer client.Close()

	_, err = client.Track(context.Background(), stream.Event{
		IdempotencyKey: "stream_usage_001",
		Subject:        "org_123",
		Meter:          "api_requests",
		Quantity:       1,
		Timestamp:      time.Now().UTC(),
		Metadata: map[string]any{
			"endpoint": "/checkout",
			"status":   200,
			"region":   "us-east",
		},
	})
	if err != nil {
		panic(err)
	}
}
```

Stream examples:

| Scenario | Path |
| --- | --- |
| Basic Go stream client | [`examples/stream/basic/go`](examples/stream/basic/go) |
| Device telemetry | [`examples/stream/advance/device-telemetry`](examples/stream/advance/device-telemetry) |
| WebSocket sessions | [`examples/stream/advance/websocket-sessions`](examples/stream/advance/websocket-sessions) |
| Queue consumers | [`examples/stream/advance/queue-consumer`](examples/stream/advance/queue-consumer) |

## Use Cases

| Use case | What you can meter |
| --- | --- |
| [API request metering](docs/content/docs/use-cases/api-requests.mdx) | Request volume by endpoint, method, status, region, and service tier. |
| [AI token usage](docs/content/docs/use-cases/ai-tokens.mdx) | Tokens by model, provider, operation, and cache path. |
| [Storage usage](docs/content/docs/use-cases/storage-usage.mdx) | Capacity by tier, region, and resource type. |
| [Active users](docs/content/docs/use-cases/active-users.mdx) | Seats, workspaces, roles, plans, and active accounts. |
| [Background jobs](docs/content/docs/use-cases/background-jobs.mdx) | Queue throughput, job outcomes, and worker regions. |
| [Feature usage](docs/content/docs/use-cases/feature-usage.mdx) | Product adoption, entitlement usage, and plan-level behavior. |
| [Historical backfill](docs/content/docs/use-cases/historical-backfill.mdx) | Older usage imported with stable idempotency keys. |

Each REST use case has runnable Go, TypeScript, Python, and C# examples under [`examples/rest/advance`](examples/rest/advance). Stream-native examples live under [`examples/stream`](examples/stream).

## SDKs

| Language | Package | Install | Example |
| --- | --- | --- | --- |
| Go REST | [`sdk/go`](sdk/go) | `go get github.com/ssubedir/open-spanner/sdk/go` | [`examples/rest/basic/go`](examples/rest/basic/go) |
| Go stream | [`sdk/go/stream`](sdk/go/stream) | `go get github.com/ssubedir/open-spanner/sdk/go` | [`examples/stream/basic/go`](examples/stream/basic/go) |
| TypeScript | [`@ssubedir/open-spanner`](https://www.npmjs.com/package/@ssubedir/open-spanner) | `npm install @ssubedir/open-spanner` | [`examples/rest/basic/typescript`](examples/rest/basic/typescript) |
| Python | [`open-spanner`](https://pypi.org/project/open-spanner/) | `pip install open-spanner` | [`examples/rest/basic/python`](examples/rest/basic/python) |
| C# | [`OpenSpanner`](https://www.nuget.org/packages/OpenSpanner/) | `dotnet add package OpenSpanner` | [`examples/rest/basic/csharp`](examples/rest/basic/csharp) |

SDKs are for trusted backend code. Do not put Open Spanner API keys in browser or mobile clients.

## Production Notes

For production, run Open Spanner with Postgres and separate API, export worker, and alert worker processes. The API and workers must share the same database. Queued exports also require shared export storage so workers can write files and the API can serve downloads.

Recommended production shape:

| Component | Recommendation |
| --- | --- |
| Database | Postgres with backups and normal database observability. |
| API | Run one or more API instances behind your ingress or load balancer. |
| Export worker | Run separately from the API when queued exports are enabled. |
| Alert worker | Run separately from the API when alert rules are enabled. |
| TLS | Terminate TLS at your ingress, load balancer, or reverse proxy. |
| gRPC | Expose only to trusted backend services that emit usage. |
| Secrets | Protect Postgres credentials, API keys, and webhook signing secrets. |

See [Production Deployment](docs/content/docs/configuration/deployment.mdx) for the checklist.

## Configuration

Common runtime variables:

| Variable | Default | Description |
| --- | --- | --- |
| `OPEN_SPANNER_HTTP_ADDR` | `:18081` | HTTP dashboard, REST API, Swagger UI, and health endpoints. |
| `OPEN_SPANNER_GRPC_ADDR` | `:18090` | gRPC usage ingestion listen address. |
| `OPEN_SPANNER_DB_DRIVER` | `sqlite` | Storage driver: `sqlite` or `postgres`. |
| `OPEN_SPANNER_SQLITE_PATH` | `open-spanner.db` | SQLite database path. |
| `OPEN_SPANNER_POSTGRES_DSN` | | Postgres connection string. |
| `OPEN_SPANNER_EXPORT_STORAGE_PATH` | `open-spanner-exports` | Shared path for generated export files. |
| `OPEN_SPANNER_EXPORT_WORKER_INTERVAL` | `5s` | Export worker polling interval. |
| `OPEN_SPANNER_ALERT_WORKER_INTERVAL` | `5s` | Alert worker polling interval. |
| `OPEN_SPANNER_RETENTION_PRUNE_ENABLED` | `false` | Enables automatic retention pruning. |

See [Environment Variables](docs/content/docs/configuration/environment-variables.mdx) for the full list.

## Examples And Tests

Verify local examples compile and build:

```sh
task test:examples
```

Run the Go stream SDK integration test:

```sh
task test:sdk
```

Run API tests:

```sh
task test
task test:postgres
```

Run dashboard E2E tests:

```sh
task test:e2e
```

## Development

Useful commands:

```sh
task test
task vet
task sqlc:check
task openapi:check
task docs:build
task admin:build
```

Regenerate SDKs:

```sh
task openapi:sdk
task sdk:go
task sdk:typescript
task sdk:python
task sdk:csharp
```

Run the dashboard dev server:

```sh
task admin:dev
```

## Project Structure

```text
cmd/api                 API entrypoint
cmd/export-worker       Queued export worker entrypoint
cmd/alert-worker        Alert evaluation worker entrypoint
internal/config         Runtime configuration
internal/server/http    HTTP server wiring
internal/ui             Embedded dashboard routes and assets
internal/metering       Domain, app services, adapters, and workers
web                     React dashboard source
docs                    Fumadocs documentation site
openapi                 Generated Swagger/OpenAPI artifacts
sdk                     Generated SDKs
sdk-test                SDK integration tests
examples                REST and gRPC stream examples
```

## License

Open Spanner is licensed under the MIT License. See [LICENSE](LICENSE).
