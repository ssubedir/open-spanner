import assert from "node:assert/strict";
import test from "node:test";
import { Metadata, status } from "@grpc/grpc-js";

import { client, createUsage, retryUsageWrite } from "../dist/index.js";
import { StreamClient } from "../dist/stream/index.js";

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

test("REST retry helper honors Retry-After and returns the eventual response", async () => {
  let calls = 0;
  const retries = [];
  const result = await retryUsageWrite(async () => {
    calls++;
    return {
      response: new Response(null, {
        status: calls === 1 ? 429 : 201,
        headers: calls === 1 ? { "Retry-After": "0.001" } : {},
      }),
    };
  }, { maxAttempts: 2, initialBackoffMs: 50, jitter: 0, onRetry: (event) => retries.push(event) });

  assert.equal(result.response.status, 201);
  assert.equal(calls, 2);
  assert.equal(retries[0].delayMs, 1);
});

test("retries transient unary ingestion with the original event", async () => {
  const retries = [];
  const streamClient = new StreamClient("localhost:1", "osp_test", {
    retry: { maxAttempts: 2, initialBackoffMs: 50, maxBackoffMs: 50, jitter: 0, onRetry: (event) => retries.push(event) },
  });
  const requests = [];
  streamClient.client.createUsage = (request, _metadata, callback) => {
    requests.push(request);
    if (requests.length === 1) {
      const metadata = new Metadata();
      metadata.set("grpc-status-details-bin", retryInfoStatus(1_000_000));
      callback(Object.assign(new Error("limited"), {
        code: status.RESOURCE_EXHAUSTED,
        details: "limited",
        metadata,
      }));
      return;
    }
    callback(null, { event: { id: "evt_retry", ...request.event } });
  };

  try {
    const result = await streamClient.track({ idempotencyKey: "idem_retry", subject: "org", meter: "requests", quantity: 1 });
    assert.equal(result.id, "evt_retry");
    assert.equal(requests.length, 2);
    assert.equal(requests[0], requests[1]);
    assert.equal(retries.length, 1);
    assert.equal(retries[0].attempt, 2);
    assert.equal(retries[0].delayMs, 1);
  } finally {
    streamClient.close();
  }
});

function retryInfoStatus(nanos) {
  const duration = fieldVarint(2, nanos);
  const retryInfo = fieldBytes(1, duration);
  const detail = Buffer.concat([
    fieldBytes(1, Buffer.from("type.googleapis.com/google.rpc.RetryInfo")),
    fieldBytes(2, retryInfo),
  ]);
  return fieldBytes(3, detail);
}

function fieldBytes(number, value) {
  return Buffer.concat([varint((number << 3) | 2), varint(value.length), value]);
}

function fieldVarint(number, value) {
  return Buffer.concat([varint(number << 3), varint(value)]);
}

function varint(value) {
  const bytes = [];
  while (value >= 0x80) {
    bytes.push((value & 0x7f) | 0x80);
    value = Math.floor(value / 128);
  }
  bytes.push(value);
  return Buffer.from(bytes);
}
