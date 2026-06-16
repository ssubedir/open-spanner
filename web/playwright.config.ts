import { defineConfig, devices } from '@playwright/test'

const apiPort = 19183
const webPort = 19173

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
      env: {
        OPEN_SPANNER_DB_DRIVER: 'sqlite',
        OPEN_SPANNER_HTTP_ADDR: `127.0.0.1:${apiPort}`,
        OPEN_SPANNER_SQLITE_PATH: '.tmp/e2e-open-spanner.db',
      },
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
