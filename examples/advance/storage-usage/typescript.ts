import { client, createMeter, createUsage } from "@ssubedir/open-spanner";

const baseUrl = process.env.OPEN_SPANNER_BASE_URL ?? "http://localhost:18081";
const apiKey = process.env.OPEN_SPANNER_API_KEY ?? "osp_...";

client.setConfig({ baseUrl, headers: { Authorization: `Bearer ${apiKey}` } });

const now = new Date();
const runId = now.getTime();
const meterName = `sdk_ts_storage_gb_hours_${runId}`;

await createMeter({
  body: {
    name: meterName,
    description: "Track storage consumption by tier, region, and resource type",
    unit: "gb_hour",
    aggregation: "sum",
    event_retention_days: 90,
    dimensions: [
      { name: "tier", display_name: "Tier", description: "Storage tier", type: "string", required: true },
      { name: "region", display_name: "Region", description: "Storage region", type: "string", required: true },
      { name: "resource_type", display_name: "Resource type", description: "Stored resource type", type: "string", required: true },
    ],
  },
  throwOnError: true,
});

const events = [
  { subject: "org_acme", quantity: 512.5, metadata: { tier: "hot", region: "us-east", resource_type: "object" } },
  { subject: "org_acme", quantity: 128, metadata: { tier: "archive", region: "us-east", resource_type: "backup" } },
  { subject: "org_globex", quantity: 74.25, metadata: { tier: "hot", region: "eu-west", resource_type: "object" } },
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

console.log(`seeded storage usage meter ${meterName} with ${events.length} events`);
