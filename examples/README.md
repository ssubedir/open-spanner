# Open Spanner Examples

These examples show how to use Open Spanner from the official SDKs. Start with a
basic SDK example, then move into isolated product scenarios under
`examples/advance`.

## Basic Examples

The basic examples create one meter, write one usage event, and show dimension
validation rejecting invalid metadata.

| SDK | Command |
| --- | --- |
| TypeScript | `cd examples/basic/typescript && npm install && npm run start` |
| Python | `cd examples/basic/python && uv run python basic.py` |
| C# | `cd examples/basic/csharp && dotnet run --project OpenSpanner.Example.csproj` |
| Go | `cd examples/basic/go && go run main.go` |

## Advanced Examples

Each folder under `examples/advance` is its own small project. A scenario does
not seed any other use case, so you can run only the pattern you want to inspect.

| Scenario | What it shows |
| --- | --- |
| `api-requests` | Request volume by endpoint, method, status, and region |
| `active-users` | SaaS seat or active-user metering by plan and workspace |
| `ai-tokens` | AI token consumption by model, provider, operation, and cache path |
| `storage-usage` | Capacity usage by tier, region, and resource type |
| `background-jobs` | Asynchronous work by queue, status, and worker region |
| `feature-usage` | Product feature and entitlement usage by plan and source |
| `historical-backfill` | Importing older usage with stable idempotency keys |

Inside each scenario folder, run the SDK you want:

| SDK | Command from the scenario folder |
| --- | --- |
| TypeScript | `npm install && npm run start` |
| Python | `uv run python python.py` |
| C# | `dotnet run --project OpenSpanner.<Scenario>.csproj` |
| Go | `go run .` |

Create an API key in the dashboard before running an example. The examples read
`OPEN_SPANNER_API_KEY` and `OPEN_SPANNER_BASE_URL`; if those are not set, the
files also show the values to replace inline.

To verify that every local example still compiles and builds against the SDKs in
this checkout, run:

```sh
task test:examples
```
