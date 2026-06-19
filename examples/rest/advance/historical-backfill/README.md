# Historical Backfill

Import older usage from another system with stable idempotency keys. This is the
pattern to adapt for migrations, CSV imports, or replaying legacy billing data.

| SDK | Command |
| --- | --- |
| TypeScript | `npm install && npm run start` |
| Python | `uv run python python.py` |
| C# | `dotnet run --project OpenSpanner.HistoricalBackfill.csproj` |
| Go | `go run .` |
