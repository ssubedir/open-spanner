import { defineConfig, devices } from '@playwright/test'

const apiPort = Number(process.env.OPEN_SPANNER_E2E_API_PORT || 19183)
const dbDriver = (process.env.OPEN_SPANNER_E2E_DB_DRIVER || 'sqlite').toLowerCase()
const postgresDSN = process.env.OPEN_SPANNER_E2E_POSTGRES_DSN || 'postgres://postgres:postgres@localhost:5432/open_spanner_e2e?sslmode=disable'
const sqlitePath = process.env.OPEN_SPANNER_E2E_SQLITE_PATH || '.tmp/e2e-open-spanner.db'
const webPort = Number(process.env.OPEN_SPANNER_E2E_WEB_PORT || 19173)

const apiEnv = dbDriver === 'postgres'
  ? {
      OPEN_SPANNER_DB_DRIVER: 'postgres',
      OPEN_SPANNER_HTTP_ADDR: `127.0.0.1:${apiPort}`,
      OPEN_SPANNER_POSTGRES_DSN: postgresDSN,
    }
  : {
      OPEN_SPANNER_DB_DRIVER: 'sqlite',
      OPEN_SPANNER_HTTP_ADDR: `127.0.0.1:${apiPort}`,
      OPEN_SPANNER_SQLITE_PATH: sqlitePath,
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
      env: apiEnv,
      reuseExistingServer: false,
      timeout: 120_000,
      url: `http://127.0.0.1:${apiPort}/ready`,
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
