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
    metadata_schema: {
      plan: "string",
      region: "string",
    },
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
      plan: "pro",
      region: "us-east",
    },
  },
  throwOnError: true,
});

console.log(`created meter: ${meter?.name} (${meter?.id})`);
console.log(`recorded usage: ${usage?.id} quantity=${usage?.quantity?.toFixed(2)}`);
