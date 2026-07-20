# Open Spanner

Open Spanner is open-source usage and metering infrastructure. It records what
customers, accounts, or systems consume and turns those facts into queryable
usage, quota state, alerts, and exports.

Use it between your product and billing, finance, analytics, support, or feature
gates. Open Spanner is not a payment processor, invoice generator, tax engine,
or customer identity provider; it gives those systems reliable usage facts and
quota decisions.

## Where It Fits

| Open Spanner owns | Downstream systems own |
| --- | --- |
| Meter definitions, dimensions, and retention | Customer identity, pricing, contracts, and checkout |
| Idempotent REST and gRPC usage ingestion | Payment methods, invoices, taxes, and collection |
| Usage queries, breakdowns, alerts, and CSV exports | Finance close and warehouse modeling |
| Plans, subject assignments, quota counters, and entitlement decisions | Customer-facing plan catalog and billing workflows |

## Core Capabilities

- Typed meters and dimensions with sum, count, average, min, max, first, last,
  and rate aggregation.
- Idempotent single, bulk, and client-streaming usage ingestion over REST and
  gRPC.
- Bucketed queries, filters, breakdowns, saved queries, raw events, and direct
  or queued CSV exports.
- Plans, scheduled assignments, quota progress, atomic consumption, immutable
  decision audits, reconciliation, and guarded counter repair.
- Threshold alerts with signed, durable webhook delivery and retry recovery.
- Retention rollups that preserve historical analytics after raw-event pruning.
- Shared workspaces with owner/admin/viewer roles, secure invitations, scoped
  API keys, expiration, rotation, and revocation history.
- Replica-safe workers, S3-compatible export storage, health probes, and
  Prometheus-compatible OpenTelemetry metrics.

## Product Surfaces

| Surface | Purpose |
| --- | --- |
| Control plane | Next.js UI for workspace access, meters, usage, plans, API keys, exports, alerts, and operations. |
| REST API | Configuration, usage ingestion and queries, entitlement decisions, exports, and operator endpoints. |
| gRPC API | High-throughput trusted-backend usage ingestion. |
| SDKs | REST and gRPC clients for Go, TypeScript, Python, and C#. |
| Workers | Durable usage fanout, exports, alerts, entitlement state, and maintenance. |
| Storage | SQLite for local or single-node use; Postgres for production. |

Read the [product documentation](https://ssubedir.github.io/open-spanner/docs).

## Quick Start

Docker Compose starts the control plane, API, usage, export, alert, and
entitlement workers, Postgres, and shared export storage:

```sh
git clone https://github.com/ssubedir/open-spanner.git
cd open-spanner
docker compose -f docker-compose.app.yml up -d --build
```

Open [http://localhost:18081/register](http://localhost:18081/register).

| Endpoint | Purpose |
| --- | --- |
| `http://localhost:18081` | Control plane and proxied REST API |
| `http://localhost:18081/ready` | End-to-end control-plane, API, and database readiness |
| `localhost:18090` | gRPC usage ingestion |

Stop the stack with `docker compose -f docker-compose.app.yml down`. Add `-v`
only when you also want to remove Postgres data.

## First Usage Flow

Register a user, create an API key in the control plane, and give it
`meters:write`, `meters:read`, `usage:write`, and `usage:read`. Copy the key when
it is created; the full secret is not shown again.

```sh
export BASE_URL="http://localhost:18081"
export API_KEY="osp_..."
```

Create a meter:

```sh
curl -X POST "$BASE_URL/v1/meters" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "api_requests",
    "description": "API requests served",
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

Record one event with a stable idempotency key:

```sh
curl -X POST "$BASE_URL/v1/usages" \
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

Query daily usage:

```sh
curl "$BASE_URL/v1/usages?subject=org_123&meter=api_requests&bucket_size=day" \
  -H "Authorization: Bearer $API_KEY"
```

Continue with [querying usage](docs/content/docs/getting-started/your-first-query.mdx),
[meter dimensions](docs/content/docs/concepts/meter-dimensions.mdx), or
[plans and entitlements](docs/content/docs/concepts/plans-entitlements.mdx).

## SDKs And gRPC

Every official SDK includes the application-facing REST API and a gRPC usage
client with unary, bulk, and client-streaming writes.

| Language | Package | Guide |
| --- | --- | --- |
| Go | `github.com/ssubedir/open-spanner/sdk/go` | [Go SDK](docs/content/docs/sdks/go.mdx) |
| TypeScript | `@ssubedir/open-spanner` | [TypeScript SDK](docs/content/docs/sdks/typescript.mdx) |
| Python | `open-spanner` | [Python SDK](docs/content/docs/sdks/python.mdx) |
| C# | `OpenSpanner` | [C# SDK](docs/content/docs/sdks/csharp.mdx) |

Unary gRPC writes support opt-in retry policies with backoff, jitter, and
server-provided `RetryInfo`. Client streams are not replayed automatically;
recover uncertain streams through bulk ingestion using the original event
idempotency keys.

Runnable REST examples live under [`examples/rest`](examples/rest). The Go
stream examples under [`examples/stream`](examples/stream) cover basic
ingestion, telemetry, WebSocket sessions, queue consumers, and
entitlement-gated writes.

SDKs are for trusted backend code. Never put Open Spanner API keys in browser or
mobile applications.

## Common Use Cases

| Use case | What to meter |
| --- | --- |
| [API requests](docs/content/docs/use-cases/api-requests.mdx) | Requests by endpoint, method, status, region, or service tier |
| [AI tokens](docs/content/docs/use-cases/ai-tokens.mdx) | Tokens by model, provider, operation, or cache path |
| [Storage](docs/content/docs/use-cases/storage-usage.mdx) | Capacity by tier, region, or resource type |
| [Active users](docs/content/docs/use-cases/active-users.mdx) | Seats, workspaces, plans, or active accounts |
| [Background jobs](docs/content/docs/use-cases/background-jobs.mdx) | Queue throughput, outcomes, and worker regions |
| [Feature usage](docs/content/docs/use-cases/feature-usage.mdx) | Product adoption and plan-level behavior |
| [Historical backfill](docs/content/docs/use-cases/historical-backfill.mdx) | Older usage with stable idempotency keys |

## Container Images

Releases publish separate API and control-plane images:

```sh
docker pull ssubedir/open-spanner:0.1.13
docker pull ssubedir/open-spanner-control-plane:0.1.13
```

The API image contains the API plus usage, export, alert, and entitlement worker
binaries. Run them as separate processes against the same Postgres database.
The control-plane image serves port `18081` and proxies `/v1` to the private API
configured by `OPEN_SPANNER_API_PROXY_URL`.

See [Production Deployment](docs/content/docs/configuration/deployment.mdx) for
container topology, probes, S3 storage, database pools, TLS, and upgrade steps.

## Run From Source

Install [Task](https://taskfile.dev/) and start the API and control plane in
separate terminals:

```sh
task run:sqlite
task control-plane:dev
```

Start workers separately when exercising asynchronous behavior:

```sh
task run:usage-worker
task run:export-worker
task run:alert-worker
task run:entitlement-worker
```

For Postgres, run `task postgres:up` and use the corresponding `*:postgres`
runtime tasks.

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `OPEN_SPANNER_HTTP_ADDR` | `:18080` | Private API, health, readiness, and metrics address |
| `OPEN_SPANNER_GRPC_ADDR` | `:18090` | gRPC ingestion address |
| `OPEN_SPANNER_API_PROXY_URL` | `http://127.0.0.1:18080` | Private API origin used by the control plane |
| `OPEN_SPANNER_DB_DRIVER` | `sqlite` | `sqlite` or `postgres` storage |
| `OPEN_SPANNER_POSTGRES_DSN` | empty | Postgres connection string |
| `OPEN_SPANNER_EXPORT_STORAGE_DRIVER` | `filesystem` | `filesystem` or `s3` export artifacts |
| `OPEN_SPANNER_REGISTRATION_ENABLED` | `true` | Allow new password and OAuth accounts |

See [Environment Variables](docs/content/docs/configuration/environment-variables.mdx)
for the complete runtime reference and `.env.example` for Compose settings.

## Development

```sh
task test
task test:postgres
task test:s3
task test:e2e
task docs:build
task control-plane:build
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for load testing, generated contracts,
SDK checks, and repository workflow.

## License

Open Spanner is licensed under the MIT License. See [LICENSE](LICENSE).
