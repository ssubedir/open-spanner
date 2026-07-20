# Open Spanner TypeScript SDK

Generated TypeScript/JavaScript client for Open Spanner.

```ts
import { client, createUsage, retryUsageWrite } from "@ssubedir/open-spanner";

const apiKey = "...";

client.setConfig({
  baseUrl: "https://api.example.com",
  headers: {
    Authorization: `Bearer ${apiKey}`,
  },
});

const request = {
  body: {
    idempotency_key: crypto.randomUUID(),
    subject: "org_123",
    meter: "api_requests",
    quantity: 1,
    timestamp: new Date().toISOString(),
  },
};
const { data: usage } = await retryUsageWrite(() => createUsage(request));

console.log(usage.id);
```

`retryUsageWrite` retries transport failures, `429`, and temporary `5xx` responses, and honors `Retry-After`. Keep `throwOnError` disabled so it can inspect generated error responses.

The SDK source is generated from `../../openapi/sdk-openapi.json` with `@hey-api/openapi-ts`.

## Reliable gRPC ingestion

```ts
import { StreamClient } from "@ssubedir/open-spanner/stream";

const ingestion = new StreamClient("localhost:18090", "osp_...", {
  retry: {
    maxAttempts: 3,
    onRetry: ({ attempt, delayMs, error }) =>
      console.warn("retrying ingestion", { attempt, delayMs, code: error.code }),
  },
});

await ingestion.track({
  idempotencyKey: "usage-1",
  subject: "org_123",
  meter: "api_requests",
  quantity: 1,
});
```

Retries are opt-in and apply to unary `track` and `trackBulk` calls. They honor `google.rpc.RetryInfo` and retry only overload, temporary unavailability, and deadline failures. Client streams are not replayed automatically; retry an uncertain stream through `trackBulk` with the original event idempotency keys.
