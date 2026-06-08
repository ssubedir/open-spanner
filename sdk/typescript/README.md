# Open Spanner TypeScript SDK

Small TypeScript/JavaScript client stub for Open Spanner.

```ts
import { OpenSpannerClient } from "@ssubedir/open-spanner";

const client = new OpenSpannerClient({
  baseUrl: "https://api.example.com",
});

const meter = await client.createMeter({
  name: "api_requests",
  description: "API request counter",
  unit: "request",
  aggregation: "sum",
  event_retention_days: 30,
});

const usage = await client.createUsage({
  idempotency_key: crypto.randomUUID(),
  subject: "org_123",
  meter: meter.name,
  quantity: 1,
  timestamp: new Date().toISOString(),
});

console.log(meter.id, usage.id);
```

Types are generated from `../../docs/sdk-openapi.json` with `openapi-typescript`. The handwritten code is only a small `fetch` wrapper around those generated OpenAPI types.
