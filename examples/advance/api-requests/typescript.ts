import { client, createMeter, createUsage } from "@ssubedir/open-spanner";

const baseUrl = process.env.OPEN_SPANNER_BASE_URL ?? "http://localhost:18081";
const apiKey = process.env.OPEN_SPANNER_API_KEY ?? "osp_...";

client.setConfig({ baseUrl, headers: { Authorization: `Bearer ${apiKey}` } });

const now = new Date();
const runId = now.getTime();
const meterName = `sdk_ts_api_requests_${runId}`;

await createMeter({
  body: {
    name: meterName,
    description: "Track request volume by endpoint, method, status, and region",
    unit: "request",
    aggregation: "sum",
    event_retention_days: 90,
    dimensions: [
      { name: "endpoint", display_name: "Endpoint", description: "Route or operation", type: "string", required: true },
      { name: "method", display_name: "Method", description: "HTTP method", type: "string", required: true },
      { name: "status", display_name: "Status", description: "HTTP status code", type: "number", required: true },
      { name: "region", display_name: "Region", description: "Serving region", type: "string", required: false },
    ],
  },
  throwOnError: true,
});

const events = [
  { subject: "org_acme", quantity: 38, metadata: { endpoint: "/v1/orders", method: "POST", status: 201, region: "us-east" } },
  { subject: "org_acme", quantity: 91, metadata: { endpoint: "/v1/orders", method: "GET", status: 200, region: "us-east" } },
  { subject: "org_globex", quantity: 14, metadata: { endpoint: "/v1/invoices", method: "GET", status: 200, region: "eu-west" } },
];

for (const [index, event] of events.entries()) {
  await createUsage({
    body: {
      idempotency_key: `${meterName}-${index}-${runId}`,
      subject: event.subject,
      meter: meterName,
      quantity: event.quantity,
      timestamp: new Date(now.getTime() + index * 60_000).toISOString(),
      metadata: event.metadata,
    },
    throwOnError: true,
  });
}

console.log(`seeded API request meter ${meterName} with ${events.length} events`);
