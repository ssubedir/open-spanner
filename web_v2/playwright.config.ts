import { defineConfig, devices } from "@playwright/test";
import { resolve } from "node:path";

const apiPort = Number(process.env.OPEN_SPANNER_E2E_API_PORT || 19183);
const dbDriver = (process.env.OPEN_SPANNER_E2E_DB_DRIVER || "sqlite").toLowerCase();
const exportStoragePath =
  process.env.OPEN_SPANNER_E2E_EXPORT_STORAGE_PATH ||
  resolve(process.cwd(), "../.tmp/e2e-exports");
const grpcPort = Number(process.env.OPEN_SPANNER_E2E_GRPC_PORT || 19190);
const postgresDSN =
  process.env.OPEN_SPANNER_E2E_POSTGRES_DSN ||
  "postgres://postgres:postgres@localhost:5432/open_spanner_e2e?sslmode=disable";
const sqlitePath =
  process.env.OPEN_SPANNER_E2E_SQLITE_PATH ||
  resolve(process.cwd(), "../.tmp/e2e-open-spanner.db");
const webPort = Number(process.env.OPEN_SPANNER_E2E_WEB_PORT || 19173);

const databaseEnv: Record<string, string> = {};
if (dbDriver === "postgres") {
  databaseEnv.OPEN_SPANNER_DB_DRIVER = "postgres";
  databaseEnv.OPEN_SPANNER_POSTGRES_DSN = postgresDSN;
} else {
  databaseEnv.OPEN_SPANNER_DB_DRIVER = "sqlite";
  databaseEnv.OPEN_SPANNER_SQLITE_PATH = sqlitePath;
}

const serviceEnv = {
  ...databaseEnv,
  OPEN_SPANNER_ALERT_WORKER_HEALTH_ADDR: "127.0.0.1:19283",
  OPEN_SPANNER_ALERT_WORKER_INTERVAL: "250ms",
  OPEN_SPANNER_ENTITLEMENT_WORKER_HEALTH_ADDR: "127.0.0.1:19284",
  OPEN_SPANNER_ENTITLEMENT_WORKER_INTERVAL: "250ms",
  OPEN_SPANNER_EXPORT_STORAGE_PATH: exportStoragePath,
  OPEN_SPANNER_EXPORT_WORKER_HEALTH_ADDR: "127.0.0.1:19282",
  OPEN_SPANNER_EXPORT_WORKER_INTERVAL: "250ms",
  OPEN_SPANNER_GRPC_ADDR: `127.0.0.1:${grpcPort}`,
  OPEN_SPANNER_HTTP_ADDR: `127.0.0.1:${apiPort}`,
  OPEN_SPANNER_REGISTRATION_ENABLED: "true",
};

export default defineConfig({
  expect: {
    timeout: 10_000,
  },
  fullyParallel: false,
  outputDir: "../.tmp/playwright-results",
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
  reporter: process.env.CI ? "github" : "list",
  testDir: "./e2e/specs",
  timeout: 90_000,
  use: {
    baseURL: `http://127.0.0.1:${webPort}`,
    screenshot: "only-on-failure",
    trace: "on-first-retry",
    video: "retain-on-failure",
  },
  webServer: [
    {
      command: "go run ./cmd/api",
      cwd: "..",
      env: serviceEnv,
      reuseExistingServer: false,
      timeout: 120_000,
      url: `http://127.0.0.1:${apiPort}/ready`,
    },
    {
      command: "go run ./cmd/export-worker",
      cwd: "..",
      env: serviceEnv,
      reuseExistingServer: false,
      timeout: 120_000,
      url: "http://127.0.0.1:19282/ready",
    },
    {
      command: "go run ./cmd/alert-worker",
      cwd: "..",
      env: serviceEnv,
      reuseExistingServer: false,
      timeout: 120_000,
      url: "http://127.0.0.1:19283/ready",
    },
    {
      command: "go run ./cmd/entitlement-worker",
      cwd: "..",
      env: serviceEnv,
      reuseExistingServer: false,
      timeout: 120_000,
      url: "http://127.0.0.1:19284/ready",
    },
    {
      command: `npm run dev -- --hostname 127.0.0.1 --port ${webPort}`,
      env: {
        OPEN_SPANNER_API_PROXY_URL: `http://127.0.0.1:${apiPort}`,
        OPEN_SPANNER_NEXT_DIST_DIR: ".next-e2e",
      },
      reuseExistingServer: false,
      timeout: 120_000,
      url: `http://127.0.0.1:${webPort}/login`,
    },
  ],
  workers: 1,
});
