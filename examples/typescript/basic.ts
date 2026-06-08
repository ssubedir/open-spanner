import { OpenSpannerClient } from "@ssubedir/open-spanner";

const baseUrl = process.env.OPEN_SPANNER_BASE_URL ?? "http://localhost:18081";
const client = new OpenSpannerClient({ baseUrl });

const now = new Date();
const meterName = `sdk_ts_requests_${Math.floor(now.getTime() / 1000)}`;
const subject = "org_sdk_ts";

const meter = await client.createMeter({
  name: meterName,
  description: "TypeScript SDK example request counter",
  unit: "request",
  aggregation: "sum",
  event_retention_days: 30,
  metadata_schema: {
    plan: "string",
    region: "string",
  },
});

const usage = await client.createUsage({
  idempotency_key: `${meterName}-${Date.now()}`,
  subject,
  meter: meterName,
  quantity: 42,
  timestamp: now.toISOString(),
  metadata: {
    plan: "pro",
    region: "us-east",
  },
});

console.log(`created meter: ${meter.name} (${meter.id})`);
console.log(`recorded usage: ${usage.id} quantity=${usage.quantity?.toFixed(2)}`);
