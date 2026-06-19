# API Request Metering

Track request volume by customer, endpoint, method, status, region, and service
tier. This is the starting point for billing on API calls, monitoring customer
traffic, and finding high-volume routes.

This example also shows dimension naming styles:

- `status_code` for underscore-separated names
- `region-name` for hyphenated names
- `service.tier` for a nested metadata path sent as `{ "service": { "tier": "gold" } }`

| SDK | Command |
| --- | --- |
| TypeScript | `npm install && npm run start` |
| Python | `uv run python python.py` |
| C# | `dotnet run --project OpenSpanner.ApiRequests.csproj` |
| Go | `go run .` |
