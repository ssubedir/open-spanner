# Open Spanner TypeScript SDK

Generated TypeScript/JavaScript client for Open Spanner.

```ts
import { client, createUsage } from "@ssubedir/open-spanner";

client.setConfig({
  baseUrl: "https://api.example.com",
  headers: {
    Authorization: `Bearer ${process.env.OPEN_SPANNER_API_KEY}`,
  },
});

const { data: usage } = await createUsage({
  body: {
    idempotency_key: crypto.randomUUID(),
    subject: "org_123",
    meter: "api_requests",
    quantity: 1,
    timestamp: new Date().toISOString(),
  },
  throwOnError: true,
});

console.log(usage.id);
```

The SDK source is generated from `../../openapi/sdk-openapi.json` with `@hey-api/openapi-ts`.
