import { checkEntitlement, client, createUsage } from "@ssubedir/open-spanner";

const baseUrl = process.env.OPEN_SPANNER_BASE_URL ?? "http://localhost:18081";
const apiKey = process.env.OPEN_SPANNER_API_KEY ?? "osp_...";
const meter = process.env.OPEN_SPANNER_METER ?? "api_calls";
const subject = process.env.OPEN_SPANNER_SUBJECT ?? "org_123";
const quantity = Number(process.env.OPEN_SPANNER_QUANTITY ?? "1");

client.setConfig({ baseUrl, headers: { Authorization: `Bearer ${apiKey}` } });

const { data: entitlement } = await checkEntitlement({
  body: { subject, meter, quantity },
  throwOnError: true,
});

console.log(
  `${entitlement?.subject} on ${entitlement?.plan_name}: allowed=${entitlement?.allowed} state=${entitlement?.state} remaining=${entitlement?.remaining}`,
);

if (entitlement?.allowed) {
  await createUsage({
    body: {
      idempotency_key: `entitlement-check-${subject}-${Date.now()}`,
      subject,
      meter,
      quantity,
      timestamp: new Date().toISOString(),
      metadata: { source: "entitlement-check-example" },
    },
    throwOnError: true,
  });

  console.log("usage accepted");
}

