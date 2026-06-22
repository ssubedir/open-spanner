# Entitlement Check

Check quota before accepting usage for a subject. This is the backend service
pattern for feature gates, API limits, and usage-based package limits.

Before running an example, create a meter, plan, and subject assignment in the
dashboard. The examples default to:

- meter: `api_calls`
- subject: `org_123`
- quantity: `1`

Override those with `OPEN_SPANNER_METER`, `OPEN_SPANNER_SUBJECT`, and
`OPEN_SPANNER_QUANTITY`.

| SDK | Command |
| --- | --- |
| TypeScript | `npm install && npm run start` |
| Python | `uv run python python.py` |
| C# | `dotnet run --project OpenSpanner.EntitlementCheck.csproj` |
| Go | `go run .` |

