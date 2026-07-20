import { execFile } from 'node:child_process'
import { promisify } from 'node:util'
import { resolve } from 'node:path'

import { expect, test, type Page } from '@playwright/test'

import { Given, Then, When } from '../support/dashboard.steps'

test.describe('Feature: Quota reconciliation', () => {
  test('Scenario: a user detects, previews, and repairs quota counter drift', async ({ page }) => {
    const account = await Given.aDashboardAccount(page)
    const id = uniqueID()
    const meter = `reconciliation_requests_${id}`
    const subject = `org_reconciliation_${id}`

    await When.theUserSignsIn(page, account)
    await Then.theDashboardIsAvailable(page, account)
    await createMeter(page, meter)
    const planID = await createPlan(page, meter, id)

    const assignment = await page.request.put(`/v1/plans/subjects/${encodeURIComponent(subject)}`, {
      data: { plan_id: planID },
    })
    expect(assignment.status()).toBe(200)

    const consumed = await page.request.post('/v1/entitlements/consume', {
      data: {
        idempotency_key: `reconciliation-consume-${id}`,
        meter,
        quantity: 2,
        subject,
      },
    })
    expect(consumed.status()).toBe(201)

    await expect.poll(async () => {
      const response = await page.request.get(`/v1/entitlements/states?subject=${encodeURIComponent(subject)}&meter=${encodeURIComponent(meter)}`)
      expect(response.status()).toBe(200)
      const states = await response.json() as { items: Array<{ current: number }> }
      return states.items[0]?.current
    }).toBe(2)

    await corruptCounter(subject, meter)

    await page.goto('/reconciliation')
    await expect(page.getByRole('heading', { name: 'Quota reconciliation' })).toBeVisible()
    await page.getByRole('button', { name: 'Run scan' }).click()

    const driftRow = page.getByRole('row').filter({ hasText: 'counter_event_count_mismatch' })
    await expect(driftRow).toBeVisible()
    await expect(driftRow).toContainText(subject)
    await expect(driftRow).toContainText(meter)
    await expect(page.locator('main')).toContainText('Drift detected')

    await driftRow.getByRole('button', { name: 'Preview repair' }).click()
    const preview = page.getByRole('dialog', { name: 'Preview quota counter repair' })
    await expect(preview).toContainText(subject)
    await expect(preview).toContainText(meter)
    const eventCount = preview.getByRole('row').filter({ hasText: 'Event count' })
    await expect(eventCount).toContainText('0')
    await expect(eventCount).toContainText('1')
    await expect(preview.getByRole('row').filter({ hasText: 'Quantity sum' })).toContainText('2')

    await preview.getByRole('button', { name: 'Apply audited repair' }).click()

    await expect(page.getByText('No quota inconsistencies detected.')).toBeVisible()
    await expect(page.locator('main')).toContainText('Healthy')
    const appliedRepair = page.getByRole('row').filter({ hasText: subject }).filter({ hasText: meter }).filter({ hasText: 'Applied' })
    await expect(appliedRepair).toBeVisible()
  })
})

async function createMeter(page: Page, name: string) {
  const response = await page.request.post('/v1/meters', {
    data: {
      aggregation: 'sum',
      description: 'Reconciliation E2E meter',
      dimensions: [],
      event_retention_days: 30,
      name,
      unit: 'request',
    },
  })
  expect(response.status()).toBe(201)
}

async function createPlan(page: Page, meter: string, id: string) {
  const response = await page.request.post('/v1/plans', {
    data: {
      description: 'Reconciliation E2E plan',
      limits: [{
        enforcement: 'hard',
        failure_policy: 'fail_closed',
        limit: 100,
        meter,
        period: 'month',
        warning_percent: 80,
      }],
      name: `Reconciliation plan ${id}`,
    },
  })
  expect(response.status()).toBe(201)
  const plan = await response.json() as { id: string }
  expect(plan.id).toBeTruthy()
  return plan.id
}

function uniqueID() {
  return `${Date.now()}${Math.random().toString(16).slice(2, 10)}`
}

async function corruptCounter(subject: string, meter: string) {
  const run = promisify(execFile)
  await run('go', [
    'run',
    './internal/testsupport/corruptcounter',
    '--subject',
    subject,
    '--meter',
    meter,
  ], {
    cwd: resolve(process.cwd(), '..'),
    env: process.env,
  })
}
