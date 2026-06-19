import { client, createMeter, createUsage } from "@ssubedir/open-spanner";

const baseUrl = process.env.OPEN_SPANNER_BASE_URL ?? "http://localhost:18081";
const apiKey = process.env.OPEN_SPANNER_API_KEY ?? "osp_...";

client.setConfig({ baseUrl, headers: { Authorization: `Bearer ${apiKey}` } });

const now = new Date();
const runId = now.getTime();
const meterName = `sdk_ts_feature_uses_${runId}`;

await createMeter({
  body: {
    name: meterName,
    description: "Track usage of premium features and add-ons by customer plan",
    unit: "use",
    aggregation: "sum",
    event_retention_days: 90,
    dimensions: [
      { name: "feature", display_name: "Feature", description: "Product feature or add-on", type: "string", required: true },
      { name: "plan", display_name: "Plan", description: "Customer plan", type: "string", required: true },
      { name: "source", display_name: "Source", description: "UI, API, automation, or integration", type: "string", required: false },
    ],
  },
  throwOnError: true,
});

const events = [
  { subject: "org_acme", quantity: 48, metadata: { feature: "audit_exports", plan: "enterprise", source: "ui" } },
  { subject: "org_acme", quantity: 19, metadata: { feature: "custom_reports", plan: "enterprise", source: "api" } },
  { subject: "org_globex", quantity: 8, metadata: { feature: "custom_reports", plan: "business", source: "automation" } },
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

console.log(`seeded feature usage meter ${meterName} with ${events.length} events`);
