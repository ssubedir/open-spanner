import { createServer, type Server } from 'node:http'

import { expect, request, test, type Page } from '@playwright/test'

import { Given, Then, When } from '../support/dashboard.steps'

type WebhookRequest = {
  body: Record<string, unknown>
  signature: string
  timestamp: string
}

test.describe('Feature: Alerts, plans, and entitlements', () => {
  test('Scenario: alert evaluation handles no data and delivers a triggered webhook', async ({ page }) => {
    const account = await Given.aDashboardAccount(page)
    const id = uniqueID()
    const meter = `alert_requests_${id}`
    const subject = `org_alert_${id}`
    const alertName = `High request volume ${id}`
    const webhook = await startWebhookReceiver()

    try {
      await When.theUserSignsIn(page, account)
      await Then.theDashboardIsAvailable(page, account)
      await createMeter(page, meter, 'request')

      const destinationName = `E2E webhook ${id}`
      await page.goto('/alerts')
      await page.getByRole('button', { name: 'New destination' }).click()
      const destinationDialog = page.getByRole('dialog', { name: 'Create Alert Destination' })
      await destinationDialog.getByLabel('Name').fill(destinationName)
      await destinationDialog.getByLabel('Webhook URL').fill(webhook.url)
      await destinationDialog.getByRole('button', { name: 'Create', exact: true }).click()
      await expect(page.locator('main')).toContainText(destinationName)
      const destination = await findListItem(page, '/v1/alerts/destinations', 'name', destinationName)

      const emptyRuleResponse = await page.request.post('/v1/alerts', {
        data: {
          comparator: 'gte',
          destination_id: destination.id,
          evaluation_interval_seconds: 60,
          meter,
          name: `No data ${id}`,
          subject: `org_without_usage_${id}`,
          threshold: 1,
          window_seconds: 3600,
        },
      })
      expect(emptyRuleResponse.status()).toBe(201)
      const emptyRule = await emptyRuleResponse.json() as { id: string }

      const emptyEvaluation = await page.request.post(`/v1/alerts/${emptyRule.id}/evaluate`)
      expect(emptyEvaluation.status()).toBe(200)
      const emptyResult = await emptyEvaluation.json() as { state: { status: string; value: number } }
      expect(emptyResult.state).toMatchObject({ status: 'no_data', value: 0 })

      const emptyEvents = await page.request.get(`/v1/alerts/events?rule_id=${encodeURIComponent(emptyRule.id)}&limit=10`)
      expect(emptyEvents.status()).toBe(200)
      const emptyEventList = await emptyEvents.json() as { items: Array<{ type: string }> }
      expect(emptyEventList.items.some((event) => event.type === 'evaluation_failed')).toBe(false)

      await page.getByRole('button', { name: 'New rule' }).click()
      const ruleDialog = page.getByRole('dialog', { name: 'Create Alert Rule' })
      await ruleDialog.getByLabel('Name').fill(alertName)
      await ruleDialog.getByLabel('Threshold').fill('5')
      await selectLabeledOption(page, ruleDialog, 'Meter', meter)
      await selectLabeledOption(page, ruleDialog, 'Destination', destinationName)
      await ruleDialog.getByRole('button', { name: 'Create rule' }).click()
      await expect(page.locator('main')).toContainText(alertName)
      const rule = await findListItem(page, '/v1/alerts', 'name', alertName)

      const usageResponse = await page.request.post('/v1/usages', {
        data: {
          idempotency_key: `alert-trigger-${id}`,
          meter,
          quantity: 6,
          subject,
          timestamp: new Date().toISOString(),
        },
      })
      expect(usageResponse.status()).toBe(201)

      const evaluation = await page.request.post(`/v1/alerts/${rule.id}/evaluate`)
      expect(evaluation.status()).toBe(200)
      const evaluationResult = await evaluation.json() as { state: { status: string; value: number } }
      expect(evaluationResult.state).toMatchObject({ status: 'alerting', value: 6 })

      const delivery = await webhook.nextRequest()
      expect(delivery.signature).toMatch(/^v1=/)
      expect(delivery.timestamp).toMatch(/^\d+$/)
      expect(delivery.body).toMatchObject({
        event: { type: 'triggered', value: 6 },
        rule: { id: rule.id, meter },
        state: { status: 'alerting', value: 6 },
      })

      await expect.poll(async () => {
        const response = await page.request.get(`/v1/alerts/events?rule_id=${encodeURIComponent(rule.id)}&limit=10`)
        expect(response.status()).toBe(200)
        const payload = await response.json() as {
          items: Array<{ delivery?: { status: string; status_code: number }; type: string }>
        }
        return payload.items.find((event) => event.type === 'triggered')?.delivery
      }, {
        intervals: [250, 500, 1000],
        timeout: 20_000,
      }).toMatchObject({ status: 'delivered', status_code: 204 })

      await page.goto(`/alerts/${rule.id}`)
      await expect(page.getByRole('heading', { name: alertName })).toBeVisible()
      await expect(page.locator('main')).toContainText('Delivered')
      await page.getByRole('button', { name: 'View triggered alert event' }).click()

      const eventDialog = page.getByRole('dialog', { name: 'Alert Event' })
      await expect(eventDialog).toContainText(alertName)
      await expect(eventDialog).toContainText('Value')
      await expect(eventDialog).toContainText('6')
      await expect(eventDialog).toContainText('Condition')
      await expect(eventDialog).toContainText('>= 5')
      await expect(eventDialog).toContainText('Event JSON')
    } finally {
      await webhook.close()
    }
  })

  test('Scenario: a multi-limit plan supports assignment, progress, preview, and API quota checks', async ({ page }) => {
    const account = await Given.aDashboardAccount(page)
    const id = uniqueID()
    const requestMeter = `plan_requests_${id}`
    const storageMeter = `plan_storage_${id}`
    const planName = `Growth ${id}`
    const subject = `org_plan_${id}`

    await When.theUserSignsIn(page, account)
    await Then.theDashboardIsAvailable(page, account)
    await createMeter(page, requestMeter, 'request')
    await createMeter(page, storageMeter, 'GB')

    await page.goto('/plans')
    await page.getByRole('button', { name: 'New plan' }).click()
    const createDialog = page.getByRole('dialog', { name: 'Create Plan' })
    await createDialog.getByLabel('Name').fill(planName)
    await createDialog.getByLabel('Description').fill('E2E multi-limit plan')
    await fillPlanLimit(page, createDialog, 0, requestMeter, '10', '60')
    await createDialog.getByRole('button', { name: 'Add limit' }).click()
    await fillPlanLimit(page, createDialog, 1, storageMeter, '100', '80')
    await createDialog.getByRole('button', { name: 'Save' }).click()
    await expect(page.locator('main')).toContainText(planName)
    const plan = await findListItem(page, '/v1/plans', 'name', planName)

    await page.goto(`/plans/${plan.id}`)
    await page.getByRole('button', { name: 'Assign subject' }).click()
    const assignmentDialog = page.getByRole('dialog', { name: 'Assign Subject' })
    await assignmentDialog.getByLabel('Subject').fill(subject)
    await assignmentDialog.getByRole('button', { name: 'Assign', exact: true }).click()
    await expect(page.locator('main')).toContainText(subject)

    for (const [meter, quantity] of [[requestMeter, 12], [storageMeter, 25]] as const) {
      const usage = await page.request.post('/v1/usages', {
        data: {
          idempotency_key: `${meter}-${id}`,
          meter,
          quantity,
          subject,
          timestamp: new Date().toISOString(),
        },
      })
      expect(usage.status()).toBe(201)
    }

    await expect(page.getByRole('heading', { name: planName })).toBeVisible()
    await expect(page.locator('main')).toContainText(requestMeter)
    await expect(page.locator('main')).toContainText(storageMeter)
    await expect(page.locator('main')).toContainText('10 / month')
    await expect(page.locator('main')).toContainText('100 / month')
    await expect(page.locator('main')).toContainText(subject)

    await page.getByRole('button', { name: `View ${subject} progress` }).click()
    const progressDialog = page.getByRole('dialog', { name: 'Usage Progress' })
    await expect(progressDialog).toContainText(requestMeter)
    await expect(progressDialog).toContainText('12 / 10 request')
    await expect(progressDialog).toContainText('Exceeded')
    await expect(progressDialog).toContainText('2 over')
    await expect(progressDialog).toContainText(storageMeter)
    await expect(progressDialog).toContainText('25 / 100 GB')
    await progressDialog.getByRole('button', { name: 'Close' }).last().click()

    const keyResponse = await page.request.post('/v1/auth/api-keys', {
      data: {
        name: `plan-reader-${id}`,
        scopes: ['plans:read'],
      },
    })
    expect(keyResponse.status()).toBe(201)
    const apiKey = await keyResponse.json() as { key: string }
    const api = await request.newContext({
      baseURL: new URL(page.url()).origin,
      extraHTTPHeaders: { Authorization: `Bearer ${apiKey.key}` },
    })
    try {
      const quotaResponse = await api.post('/v1/entitlements/check', {
        data: { meter: requestMeter, quantity: 1, subject },
      })
      expect(quotaResponse.status()).toBe(200)
      const quota = await quotaResponse.json() as {
        allowed: boolean
        current: number
        limit: number
        remaining: number
      }
      expect(quota).toMatchObject({ allowed: false, current: 12, limit: 10, remaining: 0 })

      const storageQuotaResponse = await api.post('/v1/entitlements/check', {
        data: { meter: storageMeter, quantity: 1, subject },
      })
      expect(storageQuotaResponse.status()).toBe(200)
      const storageQuota = await storageQuotaResponse.json() as {
        allowed: boolean
        current: number
        limit: number
        remaining: number
      }
      expect(storageQuota).toMatchObject({ allowed: true, current: 25, limit: 100, remaining: 74 })
    } finally {
      await api.dispose()
    }

    await page.getByRole('button', { name: 'Edit' }).click()
    const editDialog = page.getByRole('dialog', { name: 'Edit Plan' })
    await editDialog.locator('input[type="number"]').nth(0).fill('4')
    await editDialog.getByRole('button', { name: 'Preview changes' }).click()

    const previewDialog = page.getByRole('dialog', { name: 'Plan Change Impact' })
    await expect(previewDialog).toContainText('Subjects')
    await expect(previewDialog).toContainText('OK')
    await expect(previewDialog).toContainText('Warning')
    await expect(previewDialog).toContainText('Exceeded')
    await expect(previewDialog).toContainText('1')
    await expect(previewDialog).not.toContainText(subject)
  })
})

async function createMeter(page: Page, name: string, unit: string) {
  const response = await page.request.post('/v1/meters', {
    data: {
      aggregation: 'sum',
      description: 'E2E feature coverage meter',
      dimensions: [],
      event_retention_days: 30,
      name,
      unit,
    },
  })
  expect(response.status()).toBe(201)
}

async function findListItem(page: Page, path: string, key: string, value: string): Promise<{ id: string }> {
  const response = await page.request.get(path)
  expect(response.status()).toBe(200)
  const payload = await response.json() as { items: Array<Record<string, unknown>> }
  const item = payload.items.find((candidate) => candidate[key] === value)
  expect(item).toBeTruthy()
  return { id: String(item?.id || '') }
}

async function selectLabeledOption(
  page: Page,
  dialog: ReturnType<Page['getByRole']>,
  label: string,
  option: string,
) {
  const field = dialog.locator('label').filter({ hasText: new RegExp(`^${label}`) }).first()
  await field.getByRole('combobox').click()
  await page.getByRole('option', { name: option, exact: true }).click()
}

async function fillPlanLimit(
  page: Page,
  dialog: ReturnType<Page['getByRole']>,
  index: number,
  meter: string,
  limit: string,
  warningPercent: string,
) {
  const row = dialog.getByRole('group', { name: `Limit ${index + 1}` })
  const meterSelect = row.getByRole('combobox', { name: 'Meter' })
  await meterSelect.click()
  await page.getByRole('option', { name: meter, exact: true }).click()
  await row.locator('input[type="number"]').nth(0).fill(limit)
  await row.locator('input[type="number"]').nth(1).fill(warningPercent)
}

async function startWebhookReceiver(): Promise<{
  close: () => Promise<void>
  nextRequest: () => Promise<WebhookRequest>
  url: string
}> {
  let resolveRequest: ((request: WebhookRequest) => void) | null = null
  const pending = new Promise<WebhookRequest>((resolve) => {
    resolveRequest = resolve
  })
  const server = createServer((request, response) => {
    const chunks: Buffer[] = []
    request.on('data', (chunk) => chunks.push(Buffer.from(chunk)))
    request.on('end', () => {
      const body = JSON.parse(Buffer.concat(chunks).toString('utf8')) as Record<string, unknown>
      resolveRequest?.({
        body,
        signature: String(request.headers['x-open-spanner-signature'] || ''),
        timestamp: String(request.headers['x-open-spanner-timestamp'] || ''),
      })
      response.writeHead(204)
      response.end()
    })
  })

  await listen(server)
  const address = server.address()
  if (!address || typeof address === 'string') {
    throw new Error('Webhook receiver did not bind to a TCP port')
  }

  return {
    close: () => close(server),
    nextRequest: () => Promise.race([
      pending,
      new Promise<never>((_, reject) => {
        setTimeout(() => reject(new Error('Timed out waiting for alert webhook')), 20_000)
      }),
    ]),
    url: `http://127.0.0.1:${address.port}`,
  }
}

function listen(server: Server) {
  return new Promise<void>((resolve, reject) => {
    server.once('error', reject)
    server.listen(0, '127.0.0.1', () => {
      server.off('error', reject)
      resolve()
    })
  })
}

function close(server: Server) {
  return new Promise<void>((resolve, reject) => {
    server.close((error) => error ? reject(error) : resolve())
  })
}

function uniqueID() {
  return `${Date.now()}${Math.random().toString(16).slice(2, 10)}`
}
