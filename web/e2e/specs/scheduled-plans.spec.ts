import { expect, request, test, type Page } from '@playwright/test'

import { Given, Then, When } from '../support/dashboard.steps'

test.describe('Feature: Scheduled plan assignments', () => {
  test('Scenario: a user schedules, activates, and removes a plan change', async ({ page }) => {
    const account = await Given.aDashboardAccount(page)
    const id = uniqueID()
    const currentMeter = `scheduled_current_${id}`
    const futureMeter = `scheduled_future_${id}`
    const subject = `org_scheduled_${id}`

    await When.theUserSignsIn(page, account)
    await Then.theDashboardIsAvailable(page, account)
    await createMeter(page, currentMeter)
    await createMeter(page, futureMeter)
    const currentPlan = await createPlan(page, `Current plan ${id}`, currentMeter, 10)
    const futurePlan = await createPlan(page, `Future plan ${id}`, futureMeter, 20)

    await page.goto(`/plans/${currentPlan.id}`)
    await assignSubject(page, subject)
    await expect(assignmentCard(page).getByRole('row').filter({ hasText: subject })).toContainText('Active')

    const effectiveAt = new Date(Date.now() + 12_000)
    await page.goto(`/plans/${futurePlan.id}`)
    await assignSubject(page, subject, effectiveAt)
    await expect(assignmentCard(page).getByRole('row').filter({ hasText: subject })).toContainText('Scheduled')
    await expect(historyCard(page).getByRole('row').filter({ hasText: subject })).toContainText('Scheduled')

    expect(await checkEntitlement(page, subject, currentMeter)).toMatchObject({ allowed: true, limit: 10, state: 'ok' })
    expect(await checkEntitlement(page, subject, futureMeter)).toMatchObject({ allowed: false, state: 'not_in_plan' })
    await expectWorkspaceIsolation(page, subject, id)

    await expect.poll(async () => (await checkEntitlement(page, subject, futureMeter)).state, { timeout: 25_000 }).toBe('ok')
    expect(await checkEntitlement(page, subject, futureMeter)).toMatchObject({ allowed: true, limit: 20, state: 'ok' })
    expect(await checkEntitlement(page, subject, currentMeter)).toMatchObject({ allowed: false, state: 'not_in_plan' })

    await page.reload()
    const remove = page.getByRole('button', { name: `Remove ${subject} assignment` })
    const activeRow = assignmentCard(page).getByRole('row').filter({ hasText: subject }).filter({ has: remove })
    await expect(activeRow).toContainText('Active')

    await page.goto(`/plans/${currentPlan.id}`)
    await expect(historyCard(page).getByRole('row').filter({ hasText: subject })).toContainText('Ended')

    await page.goto(`/plans/${futurePlan.id}`)
    await page.getByRole('button', { name: `Remove ${subject} assignment` }).click()
    await expect(assignmentCard(page)).not.toContainText(subject)
    await expect(historyCard(page).getByRole('row').filter({ hasText: subject })).toContainText('Ended')
    expect(await checkEntitlement(page, subject, futureMeter)).toMatchObject({ allowed: false, state: 'no_plan' })
    expect(await checkEntitlement(page, subject, currentMeter)).toMatchObject({ allowed: false, state: 'no_plan' })
  })
})

async function assignSubject(page: Page, subject: string, effectiveAt?: Date) {
  await page.getByRole('button', { name: 'Assign subject' }).click()
  const dialog = page.getByRole('dialog', { name: 'Assign Subject' })
  await dialog.getByLabel('Subject').fill(subject)
  if (effectiveAt) {
    await dialog.getByRole('combobox', { name: 'Effective' }).click()
    await page.getByRole('option', { name: 'Schedule change' }).click()
    await dialog.getByLabel('Effective at').fill(toLocalDateTime(effectiveAt))
  }
  await dialog.getByRole('button', { name: 'Assign', exact: true }).click()
}

async function createMeter(page: Page, name: string) {
  const response = await page.request.post('/v1/meters', {
    data: {
      aggregation: 'sum',
      description: 'Scheduled assignment E2E meter',
      dimensions: [],
      event_retention_days: 30,
      name,
      unit: 'request',
    },
  })
  expect(response.status()).toBe(201)
}

async function createPlan(page: Page, name: string, meter: string, limit: number) {
  const response = await page.request.post('/v1/plans', {
    data: {
      description: 'Scheduled assignment E2E plan',
      limits: [{ enforcement: 'hard', failure_policy: 'fail_closed', limit, meter, period: 'month', warning_percent: 80 }],
      name,
    },
  })
  expect(response.status()).toBe(201)
  return response.json() as Promise<{ id: string }>
}

async function checkEntitlement(page: Page, subject: string, meter: string) {
  const response = await page.request.post('/v1/entitlements/check', { data: { meter, quantity: 1, subject } })
  expect(response.status()).toBe(200)
  return response.json() as Promise<{ allowed: boolean; limit: number; state: string }>
}

async function expectWorkspaceIsolation(page: Page, subject: string, id: string) {
  const client = await request.newContext({ baseURL: new URL(page.url()).origin })
  const account = { email: `scheduled-other-${id}@example.com`, password: `scheduled-other-${id}` }
  try {
    expect((await client.post('/v1/auth/users', { data: account })).status()).toBe(201)
    expect((await client.post('/v1/auth/sessions', { data: account })).status()).toBe(201)
    const response = await client.get(`/v1/plans/assignments?include_history=true&subject=${encodeURIComponent(subject)}`)
    expect(response.status()).toBe(200)
    expect(await response.json()).toMatchObject({ items: [] })
  } finally {
    await client.dispose()
  }
}

function assignmentCard(page: Page) {
  return page.getByRole('heading', { name: 'Assignments', exact: true }).locator('xpath=ancestor::section[1]')
}

function historyCard(page: Page) {
  return page.getByRole('heading', { name: 'Assignment History' }).locator('xpath=ancestor::section[1]')
}

function toLocalDateTime(value: Date) {
  const pad = (part: number) => String(part).padStart(2, '0')
  return `${value.getFullYear()}-${pad(value.getMonth() + 1)}-${pad(value.getDate())}T${pad(value.getHours())}:${pad(value.getMinutes())}:${pad(value.getSeconds())}`
}

function uniqueID() {
  return `${Date.now()}${Math.random().toString(16).slice(2, 10)}`
}
