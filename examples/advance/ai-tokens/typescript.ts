import { client, createMeter, createUsage } from "@ssubedir/open-spanner";

const baseUrl = process.env.OPEN_SPANNER_BASE_URL ?? "http://localhost:18081";
const apiKey = process.env.OPEN_SPANNER_API_KEY ?? "osp_...";

client.setConfig({ baseUrl, headers: { Authorization: `Bearer ${apiKey}` } });

const now = new Date();
const runId = now.getTime();
const meterName = `sdk_ts_tokens_used_${runId}`;

await createMeter({
  body: {
    name: meterName,
    description: "Track model token consumption by provider, model, operation, and cache path",
    unit: "token",
    aggregation: "sum",
    event_retention_days: 90,
    dimensions: [
      { name: "model", display_name: "Model", description: "Model identifier", type: "string", required: true },
      { name: "provider", display_name: "Provider", description: "AI provider", type: "string", required: true },
      { name: "operation", display_name: "Operation", description: "Completion, embedding, or rerank", type: "string", required: true },
      { name: "cached", display_name: "Cached", description: "Whether cached context was used", type: "boolean", required: false },
    ],
  },
  throwOnError: true,
});

const events = [
  { subject: "org_acme", quantity: 24800, metadata: { model: "gpt-4.1", provider: "openai", operation: "completion", cached: false } },
  { subject: "org_acme", quantity: 13200, metadata: { model: "text-embedding-3-large", provider: "openai", operation: "embedding", cached: true } },
  { subject: "org_globex", quantity: 4100, metadata: { model: "claude-3-5-sonnet", provider: "anthropic", operation: "completion", cached: false } },
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

console.log(`seeded AI token meter ${meterName} with ${events.length} events`);
