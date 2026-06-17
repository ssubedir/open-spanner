# Advanced Metering Examples

Each folder is an isolated use-case project. Open a scenario folder, pick an SDK,
and run that SDK's example without seeding unrelated meters.

| Scenario | What it shows |
| --- | --- |
| `api-requests` | Request volume by endpoint, method, status, region, and service tier |
| `active-users` | SaaS seat or active-user metering by plan and workspace |
| `ai-tokens` | AI token consumption by model, provider, operation, and cache path |
| `storage-usage` | Capacity usage by tier, region, and resource type |
| `background-jobs` | Asynchronous work by queue, status, and worker region |
| `feature-usage` | Product feature and entitlement usage by plan and source |
| `historical-backfill` | Importing older usage with stable idempotency keys |

Inside each scenario:

| SDK | Command |
| --- | --- |
| TypeScript | `npm install && npm run start` |
| Python | `uv run python python.py` |
| C# | `dotnet run --project OpenSpanner.<Scenario>.csproj` |
| Go | `go run .` |

Create an API key in the dashboard first. Each example reads
`OPEN_SPANNER_API_KEY` and `OPEN_SPANNER_BASE_URL`; if those are not set, replace
the inline fallback values in the file you are running.
