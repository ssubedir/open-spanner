import assert from "node:assert/strict";
import test from "node:test";

import { client, createUsage } from "../dist/index.js";

test("sends configured api key header with generated client", async () => {
  let request;

  client.setConfig({
    baseUrl: "https://api.example.com",
    fetch: async (input) => {
      request = input;
      return new Response(JSON.stringify({ id: "evt_123" }), {
        headers: { "Content-Type": "application/json" },
        status: 201,
      });
    },
    headers: {
      Authorization: "Bearer osp_sk_test",
    },
  });

  const response = await createUsage({
    body: {
      idempotency_key: "idem_123",
      meter: "api_requests",
      quantity: 1,
      subject: "org_123",
      timestamp: "2026-06-14T00:00:00Z",
    },
    throwOnError: true,
  });

  assert.equal(request.url, "https://api.example.com/v1/usages");
  assert.equal(request.headers.get("Authorization"), "Bearer osp_sk_test");
  assert.equal(request.headers.get("Content-Type"), "application/json");
  assert.equal((await request.clone().json()).meter, "api_requests");
  assert.equal(response.data.id, "evt_123");
});
