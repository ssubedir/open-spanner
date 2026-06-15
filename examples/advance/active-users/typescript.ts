import { client, createMeter, createUsage } from "@ssubedir/open-spanner";

const baseUrl = process.env.OPEN_SPANNER_BASE_URL ?? "http://localhost:18081";
const apiKey = process.env.OPEN_SPANNER_API_KEY ?? "osp_...";

client.setConfig({ baseUrl, headers: { Authorization: `Bearer ${apiKey}` } });

const now = new Date();
const runId = now.getTime();
const meterName = `sdk_ts_active_users_${runId}`;

await createMeter({
  body: {
    name: meterName,
    description: "Track billable active users by plan, workspace type, and region",
    unit: "user",
    aggregation: "sum",
    event_retention_days: 90,
    dimensions: [
      { name: "plan", display_name: "Plan", description: "Customer plan", type: "string", required: true },
      { name: "workspace_type", display_name: "Workspace type", description: "Workspace segment", type: "string", required: false },
      { name: "region", display_name: "Region", description: "Primary customer region", type: "string", required: false },
    ],
  },
  throwOnError: true,
});

const events = [
  { subject: "org_acme", quantity: 128, metadata: { plan: "enterprise", workspace_type: "production", region: "us-east" } },
  { subject: "org_globex", quantity: 76, metadata: { plan: "business", workspace_type: "production", region: "eu-west" } },
  { subject: "org_initech", quantity: 42, metadata: { plan: "starter", workspace_type: "sandbox", region: "us-west" } },
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

console.log(`seeded active-user meter ${meterName} with ${events.length} events`);
