import assert from "node:assert/strict";
import test from "node:test";

import { OpenSpannerClient, OpenSpannerError } from "./index.js";

test("builds requests with base url and json body", async () => {
  const calls: Array<{ input: string | URL | Request; init?: RequestInit }> = [];
  const client = new OpenSpannerClient({
    baseUrl: "http://localhost:18081/",
    fetch: async (input, init) => {
      calls.push({ input, init });
      return new Response(JSON.stringify({ name: "api_requests" }), { status: 200 });
    },
  });

  const meter = await client.createMeter({ name: "api_requests" });

  assert.equal(meter.name, "api_requests");
  assert.equal(calls[0]?.input, "http://localhost:18081/v1/meters");
  assert.equal(calls[0]?.init?.method, "POST");
  assert.equal(calls[0]?.init?.body, JSON.stringify({ name: "api_requests" }));
});

test("maps 204 responses to undefined", async () => {
  const client = new OpenSpannerClient({
    baseUrl: "http://localhost:18081",
    fetch: async () => new Response(undefined, { status: 204 }),
  });

  await assert.doesNotReject(() => client.health());
});

test("throws sdk error for non-2xx responses", async () => {
  const client = new OpenSpannerClient({
    baseUrl: "http://localhost:18081",
    fetch: async () =>
      new Response(JSON.stringify({ error: { code: "bad_request" } }), {
        status: 400,
      }),
  });

  await assert.rejects(() => client.listMeters(), OpenSpannerError);
});

test("serializes numeric query parameters", async () => {
  const calls: Array<{ input: string | URL | Request; init?: RequestInit }> = [];
  const client = new OpenSpannerClient({
    baseUrl: "http://localhost:18081",
    fetch: async (input, init) => {
      calls.push({ input, init });
      return new Response(JSON.stringify([]), { status: 200 });
    },
  });

  await client.listUsageBuckets({
    subject: "org_sdk_ts",
    meter: "api_requests",
    from: "2026-06-09T00:00:00Z",
    to: "2026-06-09T01:00:00Z",
    limit: 10,
  });

  assert.match(String(calls[0]?.input), /limit=10/);
});
