import { createServer, type Server } from 'node:http'

import { expect, test, type Page } from '@playwright/test'

import { Given, Then, When } from '../support/dashboard.steps'

type AlertDeliveryJob = {
  attempts: number
  event_id: string
  id: string
  last_error?: string
  status: 'pending' | 'running' | 'delivered' | 'dead_letter'
}

type SystemStats = {
  worker_health: Array<{
    failed_jobs: number
    name: string
    status: string
  }>
}

test.describe('Feature: Operational reliability', () => {
  test('Scenario: an operator recovers a failed webhook delivery', async ({ page }) => {
    const account = await Given.aDashboardAccount(page)
    const receiver = await startRecoverableWebhook()
    const id = `${Date.now()}`
    const meter = `worker_recovery_${id}`
    const subject = `org_worker_recovery_${id}`

    try {
      await When.theUserSignsIn(page, account)
      await Then.theDashboardIsAvailable(page, account)

      const destination = await createDestination(page, `Recovery webhook ${id}`, receiver.url)
      const rule = await createTriggeredAlert(page, meter, subject, destination.id, id)

      const job = await waitForDeliveryStatus(page, destination.id, 'dead_letter')
      expect(job.attempts).toBe(1)
      expect(job.last_error).toContain('503')

      await expect.poll(async () => {
        const health = await alertWorkerHealth(page)
        return { failedJobs: health?.failed_jobs, status: health?.status }
      }).toEqual({ failedJobs: 1, status: 'degraded' })

      await page.goto('/overview')
      const healthCard = cardNamed(page, 'Operations Health')
      const alertHealthRow = healthCard.getByRole('row').filter({ hasText: 'alert' })
      await expect(alertHealthRow).toContainText('degraded')
      await expect(alertHealthRow).toContainText('1')

      const outbox = cardNamed(page, 'Alert Delivery Outbox')
      const failedRow = outbox.getByRole('row').filter({ hasText: job.event_id })
      await expect(failedRow).toContainText('dead letter')
      await expect(failedRow).toContainText('503')
      await expect(failedRow.getByRole('button', { name: 'Retry' })).toBeVisible()

      receiver.recover()
      await failedRow.getByRole('button', { name: 'Retry' }).click()
      await expect(failedRow.getByRole('button', { name: 'Retry' })).toHaveCount(0)

      const delivered = await waitForDeliveryStatus(page, destination.id, 'delivered')
      expect(delivered.id).toBe(job.id)
      expect(receiver.requests()).toBeGreaterThanOrEqual(2)

      const duplicateRetry = await page.request.post(`/v1/alerts/delivery-jobs/${job.id}/retry`)
      expect(duplicateRetry.status()).toBe(409)

      await expect.poll(async () => {
        const health = await alertWorkerHealth(page)
        return { failedJobs: health?.failed_jobs, status: health?.status }
      }).toEqual({ failedJobs: 0, status: 'healthy' })

      await page.reload()
      const recoveredHealthRow = cardNamed(page, 'Operations Health').getByRole('row').filter({ hasText: 'alert' })
      await expect(recoveredHealthRow).toContainText('healthy')
      const deliveredRow = cardNamed(page, 'Alert Delivery Outbox').getByRole('row').filter({ hasText: job.event_id })
      await expect(deliveredRow).toContainText('delivered')
      await expect(deliveredRow.getByRole('button', { name: 'Retry' })).toHaveCount(0)

      const eventsResponse = await page.request.get(`/v1/alerts/events?rule_id=${rule.id}&limit=10`)
      expect(eventsResponse.status()).toBe(200)
      const events = await eventsResponse.json() as { items: Array<{ delivery?: { status: string }; id: string }> }
      expect(events.items.find((event) => event.id === job.event_id)?.delivery?.status).toBe('delivered')
    } finally {
      await receiver.close()
    }
  })
})

async function createDestination(page: Page, name: string, webhookURL: string): Promise<{ id: string }> {
  const response = await page.request.post('/v1/alerts/destinations', {
    data: {
      enabled: true,
      name,
      type: 'webhook',
      webhook_url: webhookURL,
    },
  })
  expect(response.status()).toBe(201)
  return response.json() as Promise<{ id: string }>
}

async function createTriggeredAlert(page: Page, meter: string, subject: string, destinationID: string, id: string): Promise<{ id: string }> {
  const meterResponse = await page.request.post('/v1/meters', {
    data: {
      aggregation: 'sum',
      description: 'E2E worker recovery meter',
      dimensions: [],
      event_retention_days: 30,
      name: meter,
      unit: 'request',
    },
  })
  expect(meterResponse.status()).toBe(201)

  const ruleResponse = await page.request.post('/v1/alerts', {
    data: {
      comparator: 'gte',
      destination_id: destinationID,
      enabled: true,
      evaluation_interval_seconds: 60,
      meter,
      name: `Worker recovery ${id}`,
      subject,
      threshold: 1,
      window_seconds: 3600,
    },
  })
  expect(ruleResponse.status()).toBe(201)
  const rule = await ruleResponse.json() as { id: string }

  const usageResponse = await page.request.post('/v1/usages', {
    data: {
      idempotency_key: `worker-recovery-${id}`,
      meter,
      quantity: 2,
      subject,
      timestamp: new Date().toISOString(),
    },
  })
  expect(usageResponse.status()).toBe(201)

  const evaluationResponse = await page.request.post(`/v1/alerts/${rule.id}/evaluate`)
  expect(evaluationResponse.status()).toBe(200)
  return rule
}

async function waitForDeliveryStatus(page: Page, destinationID: string, status: AlertDeliveryJob['status']): Promise<AlertDeliveryJob> {
  let matched: AlertDeliveryJob | undefined
  await expect.poll(async () => {
    const response = await page.request.get('/v1/alerts/delivery-jobs?limit=50')
    expect(response.status()).toBe(200)
    const payload = await response.json() as { items: Array<AlertDeliveryJob & { destination_id: string }> }
    matched = payload.items.find((job) => job.destination_id === destinationID)
    return matched?.status
  }, { intervals: [100, 250, 500], timeout: 20_000 }).toBe(status)

  if (!matched) {
    throw new Error(`No ${status} delivery job found for destination ${destinationID}`)
  }
  return matched
}

async function alertWorkerHealth(page: Page) {
  const response = await page.request.get('/v1/system/stats')
  expect(response.status()).toBe(200)
  const stats = await response.json() as SystemStats
  return stats.worker_health.find((worker) => worker.name === 'alert')
}

function cardNamed(page: Page, name: string) {
  return page.getByRole('heading', { name }).locator('xpath=ancestor::section[1]')
}

async function startRecoverableWebhook(): Promise<{
  close: () => Promise<void>
  recover: () => void
  requests: () => number
  url: string
}> {
  let requestCount = 0
  let status = 503
  const server = createServer((request, response) => {
    request.resume()
    request.once('end', () => {
      requestCount += 1
      response.writeHead(status)
      response.end()
    })
  })

  await listen(server)
  const address = server.address()
  if (!address || typeof address === 'string') {
    throw new Error('Webhook receiver did not bind a TCP port')
  }

  return {
    close: () => close(server),
    recover: () => { status = 204 },
    requests: () => requestCount,
    url: `http://127.0.0.1:${address.port}/alerts`,
  }
}

function listen(server: Server): Promise<void> {
  return new Promise((resolve, reject) => {
    server.once('error', reject)
    server.listen(0, '127.0.0.1', () => resolve())
  })
}

function close(server: Server): Promise<void> {
  return new Promise((resolve, reject) => {
    server.close((error) => error ? reject(error) : resolve())
  })
}
