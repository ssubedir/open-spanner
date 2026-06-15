# Open Spanner

Open Spanner is an open source metering service for tracking product usage:

- **Who** used something
- **What** meter was used
- **When** it happened
- **How much** was used
- **Where / in what context** through typed metadata

It is API-first and intentionally small. Sign in to the dashboard, create API keys for server-side clients, define meters, ingest usage events, query usage buckets, export data, and inspect activity from the embedded dashboard.

## Features

- Meter definitions with units, aggregation mode, retention policy, and metadata schema
- Single and bulk usage ingestion with idempotency
- Bucketed usage queries with filtering, grouping, and CSV export
- Dashboard registration, cookie sessions, and API key management
- Raw usage event search, pagination, CSV export, and retention pruning in the service API
- SQLite and Postgres storage
- Embedded React dashboard
- Swagger/OpenAPI docs
- Generated SDKs for Go, Python, TypeScript, and C#

## Quick Start

Install Task:

```sh
winget install Task.Task
# or
brew install go-task/tap/go-task
# or
npm install -g @go-task/cli
```

Run the API with SQLite storage:

```sh
task run:sqlite
```

Open:

```text
Dashboard: http://localhost:18081/login
API docs:  http://localhost:18081/docs
Health:    http://localhost:18081/health
Ready:     http://localhost:18081/ready
```

## Basic Flow

Register or log in through the dashboard, then create an API key from the API Keys page. Copy the key when it is created; only its prefix is shown after that.

Use the API key from SDKs or direct API calls:

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
    "description": "API requests accepted by the service",
    "unit": "request",
    "aggregation": "sum",
    "event_retention_days": 90,
    "metadata_schema": {
      "region": "string",
      "service": "string"
    }
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
    "timestamp": "2026-06-09T12:00:00Z",
    "metadata": {
      "region": "us-east-1",
      "service": "api"
    }
  }'
```

Query usage:

```sh
curl "http://localhost:18081/v1/usages?subject=org_123&meter=api_requests&from=2026-06-01T00:00:00Z&to=2026-07-01T00:00:00Z&bucket_size=day&limit=100" \
  -H "Authorization: Bearer $API_KEY"
```

## Concepts

A **meter** defines what can be measured, such as `api_requests`, `storage_bytes`, `seats`, or `tokens`.

A **usage event** records one measurement for one subject at one time. It includes a subject, meter, quantity, timestamp, optional metadata, and an idempotency key.

Meters support these aggregation modes:

```text
sum, count, avg, min, max, first, last, rate
```

Metadata schemas support:

```text
string, number, boolean
```

## API

The full API reference is available in Swagger UI when the server is running:

```text
http://localhost:18081/docs
```

Dashboard access uses HttpOnly cookies. SDKs and service-to-service clients use API keys in the `Authorization: Bearer <key>` header. API keys are created and deleted from the dashboard.

## SDKs

| Language | Package | Install | Example |
| --- | --- | --- | --- |
| Go | [`sdk/go`](sdk/go) | `go get github.com/ssubedir/open-spanner/sdk/go` | [`examples/basic/go`](examples/basic/go) |
| Python | [`open-spanner`](https://pypi.org/project/open-spanner/) | `pip install open-spanner` | [`examples/basic/python`](examples/basic/python) |
| TypeScript | [`@ssubedir/open-spanner`](https://www.npmjs.com/package/@ssubedir/open-spanner) | `npm install @ssubedir/open-spanner` | [`examples/basic/typescript`](examples/basic/typescript) |
| C# | [`OpenSpanner`](https://www.nuget.org/packages/OpenSpanner/) | `dotnet add package OpenSpanner` | [`examples/basic/csharp`](examples/basic/csharp) |

Regenerate SDKs:

```sh
task sdk:go
task sdk:python
task sdk:typescript
task sdk:csharp
```

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `OPEN_SPANNER_HTTP_ADDR` | `:18081` | API listen address |
| `OPEN_SPANNER_DB_DRIVER` | `sqlite` | Storage driver: `sqlite` or `postgres` |
| `OPEN_SPANNER_SQLITE_PATH` | `open-spanner.db` | SQLite database path |
| `OPEN_SPANNER_POSTGRES_DSN` | | Postgres connection string when `OPEN_SPANNER_DB_DRIVER=postgres` |
| `OPEN_SPANNER_DB_MAX_OPEN_CONNS` | `0` | Maximum open SQL connections; `0` keeps Go's default |
| `OPEN_SPANNER_DB_MAX_IDLE_CONNS` | `0` | Maximum idle SQL connections; `0` keeps Go's default |
| `OPEN_SPANNER_DB_CONN_MAX_LIFETIME` | `0` | Maximum SQL connection lifetime; `0` disables recycling |
| `OPEN_SPANNER_DB_CONN_MAX_IDLE_TIME` | `0` | Maximum SQL connection idle time; `0` disables idle-time recycling |
| `OPEN_SPANNER_RETENTION_PRUNE_ENABLED` | `false` | Enable automatic retention pruning |
| `OPEN_SPANNER_RETENTION_PRUNE_INTERVAL` | `1h` | Background prune interval |
| `OPEN_SPANNER_RETENTION_PRUNE_TIMEOUT` | `30m` | Maximum duration for one background prune run |

Run with Postgres storage:

```sh
task postgres:up
task run:postgres
```

Run Postgres integration tests:

```sh
task test:postgres
```

## Development

```sh
task test
task vet
task openapi
task openapi:convert
task openapi:sdk
task docs:build
task admin:build
```

Run the React dashboard dev server:

```sh
task admin:dev
```

## Project Structure

```text
cmd/api                 API entrypoint
internal/config         Environment configuration
internal/server/http    HTTP server wiring
internal/ui             Embedded dashboard routes/assets
internal/metering       Domain, app services, adapters, workers
web                     React dashboard source
docs                    Fumadocs documentation site
openapi                 Generated Swagger/OpenAPI artifacts
sdk                     Generated SDKs
examples                SDK examples
```

## License

Open Spanner is licensed under the MIT License. See [LICENSE](LICENSE).
