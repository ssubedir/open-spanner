import { client, createMeter, createUsage } from "@ssubedir/open-spanner";

const baseUrl = process.env.OPEN_SPANNER_BASE_URL ?? "http://localhost:18081";
const apiKey = process.env.OPEN_SPANNER_API_KEY ?? "osp_...";

client.setConfig({ baseUrl, headers: { Authorization: `Bearer ${apiKey}` } });

const now = new Date();
const runId = now.getTime();
const meterName = `sdk_ts_jobs_processed_${runId}`;

await createMeter({
  body: {
    name: meterName,
    description: "Track asynchronous work by queue, result status, and worker region",
    unit: "job",
    aggregation: "sum",
    event_retention_days: 90,
    dimensions: [
      { name: "queue", display_name: "Queue", description: "Queue or worker pool", type: "string", required: true },
      { name: "status", display_name: "Status", description: "Processing result", type: "string", required: true },
      { name: "worker_region", display_name: "Worker region", description: "Worker region", type: "string", required: false },
    ],
  },
  throwOnError: true,
});

const events = [
  { subject: "org_acme", quantity: 1200, metadata: { queue: "exports", status: "succeeded", worker_region: "us-east" } },
  { subject: "org_acme", quantity: 28, metadata: { queue: "exports", status: "failed", worker_region: "us-east" } },
  { subject: "org_globex", quantity: 1145, metadata: { queue: "imports", status: "succeeded", worker_region: "eu-west" } },
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

console.log(`seeded background job meter ${meterName} with ${events.length} events`);
