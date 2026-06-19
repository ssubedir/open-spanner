import { client, createMeter, createUsage } from "@ssubedir/open-spanner";

const baseUrl = process.env.OPEN_SPANNER_BASE_URL ?? "http://localhost:18081";
const apiKey = process.env.OPEN_SPANNER_API_KEY ?? "osp_...";

client.setConfig({
  baseUrl,
  headers: {
    Authorization: `Bearer ${apiKey}`,
  },
});

const now = new Date();
const meterName = `sdk_ts_requests_${Math.floor(now.getTime() / 1000)}`;
const subject = "org_sdk_ts";

const { data: meter } = await createMeter({
  body: {
    name: meterName,
    description: "TypeScript SDK example request counter",
    unit: "request",
    aggregation: "sum",
    event_retention_days: 30,
    dimensions: [
      {
        name: "endpoint",
        display_name: "Endpoint",
        description: "API route that handled the request",
        type: "string",
        required: true,
      },
      {
        name: "status",
        display_name: "HTTP status",
        description: "Response status code",
        type: "number",
        required: true,
      },
      {
        name: "region",
        display_name: "Region",
        description: "Serving region",
        type: "string",
        required: false,
      },
    ],
  },
  throwOnError: true,
});

const { data: usage } = await createUsage({
  body: {
    idempotency_key: `${meterName}-${Date.now()}`,
    subject,
    meter: meterName,
    quantity: 42,
    timestamp: now.toISOString(),
    metadata: {
      endpoint: "/v1/orders",
      status: 200,
      region: "us-east",
      trace_id: "trace-ts-example",
    },
  },
  throwOnError: true,
});

let validationMessage: string | undefined;
try {
  await createUsage({
    body: {
      idempotency_key: `${meterName}-invalid-${Date.now()}`,
      subject,
      meter: meterName,
      quantity: 1,
      timestamp: now.toISOString(),
      metadata: {
        endpoint: "/v1/orders",
        status: "200",
      },
    },
    throwOnError: true,
  });
} catch (error) {
  validationMessage = errorMessage(error);
}
if (!validationMessage) {
  throw new Error("expected dimension validation error");
}

console.log(`created meter: ${meter?.name} (${meter?.id})`);
console.log(`recorded usage: ${usage?.id} quantity=${usage?.quantity?.toFixed(2)}`);
console.log(`dimension validation rejected invalid usage: ${validationMessage}`);

function errorMessage(error: unknown) {
  if (error && typeof error === "object" && "message" in error) {
    return String((error as { message?: unknown }).message);
  }
  return String(error);
}
