import { expect, request, test, type Download, type Page } from '@playwright/test'

import { Given, Then, When, type ExportJobResponse } from '../support/dashboard.steps'

type CanceledExport = {
  job: ExportJobResponse
  meter: string
  subject: string
}

test.describe('Feature: Export lifecycle', () => {
  test('Scenario: a user cancels, retries, and downloads an export job', async ({ page }) => {
    const account = await Given.aDashboardAccount(page)

    await When.theUserSignsIn(page, account)
    await Then.theDashboardIsAvailable(page, account)

    const scenario = await createCanceledExport(page)
    const canceledDownload = await page.request.get(`/v1/exports/${scenario.job.id}/download`)
    expect(canceledDownload.status()).toBe(409)

    await verifyReadOnlyExportKey(page, scenario.job.id)

    await page.goto('/exports')
    await expect(page.getByRole('heading', { name: 'Export jobs' })).toBeVisible()

    const canceledRow = exportRow(page, scenario.meter)
    await expect(canceledRow).toContainText('Canceled')
    await expect(canceledRow.getByRole('button', { name: 'Download' })).toHaveCount(0)
    await canceledRow.getByRole('button', { name: 'Retry' }).click()

    await expect.poll(async () => {
      const response = await page.request.get(`/v1/exports/${scenario.job.id}`)
      expect(response.status()).toBe(200)
      const job = await response.json() as ExportJobResponse
      return job.status
    }, { timeout: 20_000 }).toBe('completed')

    await page.reload()
    const completedRow = exportRow(page, scenario.meter)
    await expect(completedRow).toContainText('Completed')
    await expect(completedRow.getByRole('button', { name: 'Retry' })).toHaveCount(0)

    const downloadPromise = page.waitForEvent('download')
    await completedRow.getByRole('button', { name: 'Download' }).click()
    const download = await readDownload(await downloadPromise)

    expect(download.filename).toBe(`usage-export-${scenario.meter}-${scenario.job.id}.csv`)
    expect(download.text).toContain('bucket_start,subject,meter,bucket_size,aggregation,unit,quantity')
    expect(download.text).toContain(scenario.meter)
    expect(download.text).toContain(scenario.subject)
    expect(download.text).toContain(',3')
  })
})

async function createCanceledExport(page: Page): Promise<CanceledExport> {
  for (let attempt = 0; attempt < 8; attempt += 1) {
    const id = `${Date.now()}_${attempt}`
    const meter = `export_lifecycle_${id}`
    const subject = `org_export_lifecycle_${id}`
    const timestamp = new Date(Date.now() - 30_000)

    const meterResponse = await page.request.post('/v1/meters', {
      data: {
        aggregation: 'sum',
        description: 'E2E export lifecycle meter',
        dimensions: [],
        event_retention_days: 30,
        name: meter,
        unit: 'request',
      },
    })
    expect(meterResponse.status()).toBe(201)

    const usageResponse = await page.request.post('/v1/usages', {
      data: {
        idempotency_key: `export-lifecycle-${id}`,
        meter,
        quantity: 3,
        subject,
        timestamp: timestamp.toISOString(),
      },
    })
    expect(usageResponse.status()).toBe(201)

    const exportResponse = await page.request.post('/v1/exports', {
      data: {
        format: 'csv',
        kind: 'usage_buckets',
        query: {
          bucket_size: 'day',
          from: new Date(timestamp.getTime() - 60_000).toISOString(),
          limit: 100,
          meter,
          subject,
          to: new Date(timestamp.getTime() + 60_000).toISOString(),
        },
      },
    })
    expect(exportResponse.status()).toBe(202)
    const job = await exportResponse.json() as ExportJobResponse

    const cancelResponse = await page.request.post(`/v1/exports/${job.id}/cancel`)
    if (cancelResponse.status() === 200) {
      const canceled = await cancelResponse.json() as ExportJobResponse
      expect(canceled.status).toBe('canceled')
      return { job: canceled, meter, subject }
    }

    expect(cancelResponse.status()).toBe(409)
  }

  throw new Error('Export worker completed every job before it could be canceled')
}

async function verifyReadOnlyExportKey(page: Page, jobID: string) {
  const createKeyResponse = await page.request.post('/v1/auth/api-keys', {
    data: {
      name: `e2e-export-read-only-${Date.now()}`,
      scopes: ['exports:read'],
    },
  })
  expect(createKeyResponse.status()).toBe(201)
  const payload = await createKeyResponse.json() as { key?: string }
  expect(payload.key).toBeTruthy()

  const api = await request.newContext({
    baseURL: new URL(page.url()).origin,
    extraHTTPHeaders: { Authorization: `Bearer ${payload.key}` },
  })

  try {
    const readResponse = await api.get(`/v1/exports/${jobID}`)
    expect(readResponse.status()).toBe(200)

    const cancelResponse = await api.post(`/v1/exports/${jobID}/cancel`)
    expect(cancelResponse.status()).toBe(403)

    const retryResponse = await api.post(`/v1/exports/${jobID}/retry`)
    expect(retryResponse.status()).toBe(403)
  } finally {
    await api.dispose()
  }
}

function exportRow(page: Page, meter: string) {
  return page.locator('.export-job-row').filter({ hasText: meter }).first()
}

async function readDownload(download: Download): Promise<{ filename: string; text: string }> {
  const stream = await download.createReadStream()
  const chunks: Buffer[] = []
  for await (const chunk of stream) {
    chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk))
  }
  return {
    filename: download.suggestedFilename(),
    text: Buffer.concat(chunks).toString('utf8'),
  }
}
