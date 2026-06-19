import assert from "node:assert/strict";
import { spawn, spawnSync, type ChildProcess } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import net, { type AddressInfo } from "node:net";
import { tmpdir } from "node:os";
import path from "node:path";
import test, { type TestContext } from "node:test";
import { fileURLToPath } from "node:url";

import { StreamClient, type Event } from "../../typescript/src/stream/index.js";

interface Service {
  baseURL: string;
  stop: () => Promise<void>;
}

interface APIKeyResponse {
  key: string;
}

interface UsageEventList {
  items: Array<{
    meter: string;
    subject: string;
    quantity: number;
    metadata: Record<string, unknown>;
  }>;
}

interface JSONResponse<T> {
  body: T;
  cookies: string[];
}

test("TypeScript stream client records bulk and streamed usage", async (t) => {
  const httpAddr = await freeTCPAddr();
  const grpcAddr = await freeTCPAddr();
  const service = await startOpenSpanner(t, httpAddr, grpcAddr);

  try {
    const suffix = `${Date.now()}`;
    const apiKey = await createAPIKey(service.baseURL, suffix);
    const meterName = `sdk_ts_stream_requests_${suffix}`;
    await createMeter(service.baseURL, apiKey, meterName);

    const client = new StreamClient(grpcAddr, apiKey);
    t.after(() => client.close());

    const now = new Date();
    const bulk = await client.trackBulk(`sdk-ts-stream-bulk-${suffix}`, [
      usageEvent(`sdk-ts-stream-bulk-${suffix}-1`, `org_sdk_ts_stream_${suffix}`, meterName, 2, now, {
        endpoint: "/orders",
        status: 200,
      }),
      usageEvent(`sdk-ts-stream-bulk-${suffix}-2`, `org_sdk_ts_stream_${suffix}`, meterName, 3, new Date(now.getTime() + 1000), {
        endpoint: "/users",
        status: 201,
      }),
    ]);

    assert.equal(bulk.acceptedCount, 2);
    assert.equal(bulk.duplicateCount, 0);
    assert.equal(bulk.failedCount, 0);

    const usageStream = client.stream(`sdk-ts-stream-${suffix}`);
    await usageStream.track(
      usageEvent(`sdk-ts-stream-${suffix}-1`, `org_sdk_ts_stream_${suffix}`, meterName, 7, new Date(now.getTime() + 2000), {
        endpoint: "/checkout",
        status: 200,
      }),
    );
    const streamed = await usageStream.close();

    assert.equal(streamed.acceptedCount, 1);
    assert.equal(streamed.duplicateCount, 0);
    assert.equal(streamed.failedCount, 0);

    const events = await listUsageEvents(service.baseURL, apiKey, meterName);
    assert.equal(events.items.length, 3);
  } finally {
    await service.stop();
  }
});

async function startOpenSpanner(t: TestContext, httpAddr: string, grpcAddr: string): Promise<Service> {
  const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../../..");
  const tempDir = mkdtempSync(path.join(tmpdir(), "open-spanner-sdk-ts-"));

  const binaryPath = path.join(tempDir, process.platform === "win32" ? "open-spanner-sdk-test.exe" : "open-spanner-sdk-test");
  const build = spawnSync("go", ["build", "-o", binaryPath, "./cmd/api"], {
    cwd: repoRoot,
    encoding: "utf8",
    env: {
      ...process.env,
      GOCACHE: path.join(repoRoot, ".tmp", "go-build"),
    },
  });
  assert.equal(build.status, 0, build.stderr || build.stdout);

  const child = spawn(binaryPath, [], {
    cwd: repoRoot,
    env: {
      ...process.env,
      OPEN_SPANNER_HTTP_ADDR: httpAddr,
      OPEN_SPANNER_GRPC_ADDR: grpcAddr,
      OPEN_SPANNER_DB_DRIVER: "sqlite",
      OPEN_SPANNER_SQLITE_PATH: path.join(tempDir, "open-spanner.db"),
      OPEN_SPANNER_EXPORT_STORAGE_PATH: path.join(tempDir, "exports"),
    },
    stdio: ["ignore", "pipe", "pipe"],
  });

  let log = "";
  child.stdout.on("data", (chunk: Buffer) => {
    log += chunk.toString();
  });
  child.stderr.on("data", (chunk: Buffer) => {
    log += chunk.toString();
  });

  let stopped = false;
  const stop = async (): Promise<void> => {
    if (stopped) {
      return;
    }
    stopped = true;

    if (child.exitCode === null) {
      killProcessTree(child);
      await new Promise<void>((resolve) => {
        child.once("exit", () => resolve());
        setTimeout(resolve, 5000);
      });
    }

    rmSync(tempDir, { force: true, recursive: true });
  };
  t.after(stop);

  const baseURL = `http://${httpAddr}`;
  await waitForReady(baseURL, child, () => log);
  return { baseURL, stop };
}

async function waitForReady(baseURL: string, child: ChildProcess, getLog: () => string): Promise<void> {
  const deadline = Date.now() + 20_000;

  while (Date.now() < deadline) {
    if (child.exitCode !== null) {
      throw new Error(`API process exited before ready\n${getLog()}`);
    }

    try {
      const response = await fetch(`${baseURL}/ready`, { signal: AbortSignal.timeout(1000) });
      if (response.status === 204) {
        return;
      }
    } catch {
      // Keep polling until the service is ready or the deadline expires.
    }

    await sleep(100);
  }

  throw new Error(`API did not become ready\n${getLog()}`);
}

async function createAPIKey(baseURL: string, suffix: string): Promise<string> {
  const email = `sdk-ts-stream+${suffix}@example.com`;
  const password = "strong-password";

  await postJSON<unknown>(`${baseURL}/v1/auth/users`, {
    email,
    password,
  }, {}, 201);

  const session = await postJSON<unknown>(`${baseURL}/v1/auth/sessions`, {
    email,
    password,
  }, {}, 201);

  const cookies = session.cookies.map((cookie) => cookie.split(";")[0]).join("; ");
  const apiKey = await postJSON<APIKeyResponse>(`${baseURL}/v1/auth/api-keys`, {
    name: `sdk ts stream test ${suffix}`,
  }, {
    Cookie: cookies,
  }, 201);

  assert.ok(apiKey.body.key);
  return apiKey.body.key;
}

async function createMeter(baseURL: string, apiKey: string, meterName: string): Promise<void> {
  await postJSON<unknown>(`${baseURL}/v1/meters`, {
    name: meterName,
    description: "TypeScript SDK stream integration requests",
    unit: "request",
    aggregation: "sum",
    event_retention_days: 30,
    dimensions: [
      { name: "endpoint", type: "string", required: true },
      { name: "status", type: "number", required: true },
    ],
  }, {
    Authorization: `Bearer ${apiKey}`,
  }, 201);
}

async function listUsageEvents(baseURL: string, apiKey: string, meterName: string): Promise<UsageEventList> {
  const response = await fetch(`${baseURL}/v1/usageevents?meter=${encodeURIComponent(meterName)}&limit=10`, {
    headers: {
      Authorization: `Bearer ${apiKey}`,
    },
  });
  if (response.status !== 200) {
    assert.equal(response.status, 200, await response.text());
  }
  return response.json() as Promise<UsageEventList>;
}

function usageEvent(
  idempotencyKey: string,
  subject: string,
  meterName: string,
  quantity: number,
  timestamp: Date,
  metadata: Record<string, unknown>,
): Event {
  return {
    idempotencyKey,
    subject,
    meter: meterName,
    quantity,
    timestamp,
    metadata,
  };
}

async function postJSON<T>(url: string, body: unknown, headers: Record<string, string>, wantStatus: number): Promise<JSONResponse<T>> {
  const response = await fetch(url, {
    body: JSON.stringify(body),
    headers: {
      "Content-Type": "application/json",
      ...headers,
    },
    method: "POST",
  });
  const text = await response.text();
  assert.equal(response.status, wantStatus, text);
  return {
    body: text ? JSON.parse(text) as T : undefined as T,
    cookies: responseCookies(response.headers),
  };
}

function responseCookies(headers: Headers): string[] {
  const cookieHeaders = headers as Headers & { getSetCookie?: () => string[] };
  return cookieHeaders.getSetCookie?.() ?? splitSetCookie(headers.get("set-cookie"));
}

function splitSetCookie(header: string | null): string[] {
  if (!header) {
    return [];
  }
  return header.split(/,(?=[^;]+?=)/);
}

function freeTCPAddr(): Promise<string> {
  return new Promise((resolve, reject) => {
    const server = net.createServer();
    server.listen(0, "127.0.0.1", () => {
      const address = server.address() as AddressInfo | null;
      server.close(() => {
        if (!address) {
          reject(new Error("could not allocate tcp address"));
          return;
        }

        resolve(`${address.address}:${address.port}`);
      });
    });
    server.on("error", reject);
  });
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function killProcessTree(child: ChildProcess): void {
  if (!child.pid) {
    return;
  }

  if (process.platform === "win32") {
    spawnSync("taskkill", ["/PID", `${child.pid}`, "/T", "/F"], { stdio: "ignore" });
    return;
  }

  child.kill("SIGKILL");
}
