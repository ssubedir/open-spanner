import { defineConfig, devices } from '@playwright/test'
import { resolve } from 'node:path'

const apiPort = Number(process.env.OPEN_SPANNER_E2E_API_PORT || 19183)
const dbDriver = (process.env.OPEN_SPANNER_E2E_DB_DRIVER || 'sqlite').toLowerCase()
const exportStoragePath = process.env.OPEN_SPANNER_E2E_EXPORT_STORAGE_PATH || resolve(import.meta.dirname, '../.tmp/e2e-exports')
const grpcPort = Number(process.env.OPEN_SPANNER_E2E_GRPC_PORT || 19190)
const postgresDSN = process.env.OPEN_SPANNER_E2E_POSTGRES_DSN || 'postgres://postgres:postgres@localhost:5432/open_spanner_e2e?sslmode=disable'
const sqlitePath = process.env.OPEN_SPANNER_E2E_SQLITE_PATH || '.tmp/e2e-open-spanner.db'
const webPort = Number(process.env.OPEN_SPANNER_E2E_WEB_PORT || 19173)

const apiEnv = dbDriver === 'postgres'
  ? {
      OPEN_SPANNER_DB_DRIVER: 'postgres',
      OPEN_SPANNER_GRPC_ADDR: `127.0.0.1:${grpcPort}`,
      OPEN_SPANNER_HTTP_ADDR: `127.0.0.1:${apiPort}`,
      OPEN_SPANNER_POSTGRES_DSN: postgresDSN,
    }
  : {
      OPEN_SPANNER_DB_DRIVER: 'sqlite',
      OPEN_SPANNER_GRPC_ADDR: `127.0.0.1:${grpcPort}`,
      OPEN_SPANNER_HTTP_ADDR: `127.0.0.1:${apiPort}`,
      OPEN_SPANNER_SQLITE_PATH: sqlitePath,
    }
const serviceEnv = {
  ...apiEnv,
  OPEN_SPANNER_EXPORT_STORAGE_PATH: exportStoragePath,
  OPEN_SPANNER_EXPORT_WORKER_INTERVAL: '250ms',
}

export default defineConfig({
  expect: {
    timeout: 10_000,
  },
  fullyParallel: false,
  outputDir: '../.tmp/playwright-results',
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  reporter: process.env.CI ? 'github' : 'list',
  testDir: './e2e/specs',
  timeout: 60_000,
  use: {
    baseURL: `http://127.0.0.1:${webPort}`,
    screenshot: 'only-on-failure',
    trace: 'on-first-retry',
    video: 'retain-on-failure',
  },
  webServer: [
    {
      command: 'go run ./cmd/api',
      cwd: '..',
      env: serviceEnv,
      reuseExistingServer: false,
      timeout: 120_000,
      url: `http://127.0.0.1:${apiPort}/ready`,
    },
    {
      command: 'go run ./cmd/export-worker',
      cwd: '..',
      env: serviceEnv,
      reuseExistingServer: false,
      timeout: 120_000,
      wait: {
        stderr: /export storage path:/,
      },
    },
    {
      command: `npm run dev -- --host 127.0.0.1 --port ${webPort} --strictPort`,
      env: {
        OPEN_SPANNER_API_PROXY_URL: `http://127.0.0.1:${apiPort}`,
      },
      reuseExistingServer: false,
      timeout: 120_000,
      url: `http://127.0.0.1:${webPort}/login`,
    },
  ],
  workers: 1,
})
