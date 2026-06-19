import { client, createMeter, createUsage } from "@ssubedir/open-spanner";

const baseUrl = process.env.OPEN_SPANNER_BASE_URL ?? "http://localhost:18081";
const apiKey = process.env.OPEN_SPANNER_API_KEY ?? "osp_...";

client.setConfig({ baseUrl, headers: { Authorization: `Bearer ${apiKey}` } });

const now = new Date();
const runId = now.getTime();
const meterName = `sdk_ts_billing_events_backfilled_${runId}`;

await createMeter({
  body: {
    name: meterName,
    description: "Import historical billing events with stable idempotency keys",
    unit: "event",
    aggregation: "sum",
    event_retention_days: 90,
    dimensions: [
      { name: "source", display_name: "Source", description: "Imported source system", type: "string", required: true },
      { name: "event_type", display_name: "Event type", description: "Imported billing event type", type: "string", required: true },
      { name: "import_batch", display_name: "Import batch", description: "Backfill batch identifier", type: "string", required: true },
    ],
  },
  throwOnError: true,
});

const events = [
  { subject: "org_acme", quantity: 340, offsetMinutes: -1440, metadata: { source: "legacy-billing", event_type: "api_request", import_batch: "batch-2026-06" } },
  { subject: "org_globex", quantity: 112, offsetMinutes: -720, metadata: { source: "legacy-billing", event_type: "storage", import_batch: "batch-2026-06" } },
  { subject: "org_initech", quantity: 64, offsetMinutes: -60, metadata: { source: "csv-import", event_type: "feature_use", import_batch: "batch-2026-06" } },
];

for (const [index, event] of events.entries()) {
  await createUsage({
    body: {
      idempotency_key: `${meterName}-${event.metadata.import_batch}-${event.subject}-${index}`,
      subject: event.subject,
      meter: meterName,
      quantity: event.quantity,
      timestamp: new Date(now.getTime() + event.offsetMinutes * 60_000).toISOString(),
      metadata: event.metadata,
    },
    throwOnError: true,
  });
}

console.log(`seeded historical backfill meter ${meterName} with ${events.length} events`);
