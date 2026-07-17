import { expect, test, type Page } from '@playwright/test'

import { Given, Then, When } from '../support/dashboard.steps'

type UsageBucket = {
  aggregation: string
  bucket_size: string
  bucket_start: string
  group?: Record<string, string>
  meter: string
  quantity: number
  subject: string
  unit: string
}

type UsageEvent = {
  id: string
  meter: string
  quantity: number
  subject: string
  timestamp: string
}

type SystemStats = {
  last_prune_run?: {
    deleted: number
    dry_run: boolean
  }
  rollup_health: {
    items: Array<{
      meter: string
      status: string
    }>
    status: string
  }
  worker_health: Array<{
    name: string
    status: string
  }>
}

test.describe('Feature: Usage retention and rollups', () => {
  test('Scenario: the retention worker preserves usage analytics', async ({ page }) => {
    const account = await Given.aDashboardAccount(page)
    const id = `${Date.now()}`
    const meter = `retention_rollup_${id}`
    const subject = `org_retention_${id}`
    const oldDay = startOfUTCDay(new Date(Date.now() - 3 * 24 * 60 * 60_000))
    const oldFrom = oldDay.toISOString()
    const oldTo = new Date(oldDay.getTime() + 24 * 60 * 60_000).toISOString()

    await When.theUserSignsIn(page, account)
    await Then.theDashboardIsAvailable(page, account)
    await createRetentionMeter(page, meter)

    for (const [index, event] of [
      { hour: 10, minute: 5, quantity: 2, region: 'us-east' },
      { hour: 10, minute: 10, quantity: 3, region: 'us-east' },
      { hour: 13, minute: 0, quantity: 4, region: 'eu-west' },
    ].entries()) {
      await createUsage(page, {
        idempotencyKey: `retention-old-${id}-${index}`,
        meter,
        quantity: event.quantity,
        region: event.region,
        subject,
        timestamp: new Date(Date.UTC(
          oldDay.getUTCFullYear(), oldDay.getUTCMonth(), oldDay.getUTCDate(), event.hour, event.minute,
        )).toISOString(),
      })
    }

    const recentTimestamp = new Date()
    await createUsage(page, {
      idempotencyKey: `retention-recent-${id}`,
      meter,
      quantity: 5,
      region: 'us-east',
      subject,
      timestamp: recentTimestamp.toISOString(),
    })

    const beforeBuckets = await searchGroupedUsage(page, meter, subject, oldFrom, oldTo)
    expect(bucketSignature(beforeBuckets)).toEqual([
      { quantity: 4, region: 'eu-west' },
      { quantity: 5, region: 'us-east' },
    ])
    expect(await listRawEvents(page, meter, oldFrom, oldTo)).toHaveLength(3)

    let stats: SystemStats | undefined
    await expect.poll(async () => {
      const oldEvents = await listRawEvents(page, meter, oldFrom, oldTo)
      stats = await systemStats(page)
      const coverage = stats.rollup_health.items.find((item) => item.meter === meter)
      return {
        coverage: coverage?.status,
        oldEvents: oldEvents.length,
        retentionWorker: stats.worker_health.find((worker) => worker.name === 'retention')?.status,
      }
    }, { intervals: [500, 1000], timeout: 30_000 }).toEqual({
      coverage: 'healthy',
      oldEvents: 0,
      retentionWorker: 'healthy',
    })

    expect(stats?.last_prune_run).toMatchObject({ deleted: 3, dry_run: false })

    const afterBuckets = await searchGroupedUsage(page, meter, subject, oldFrom, oldTo)
    expect(bucketSignature(afterBuckets)).toEqual(bucketSignature(beforeBuckets))

    const dimensionResponse = await page.request.get('/v1/usages/dimensions', {
      params: { field: 'region', from: oldFrom, meter, to: oldTo },
    })
    expect(dimensionResponse.status()).toBe(200)
    const dimensions = await dimensionResponse.json() as { items: Array<{ value: string }> }
    expect(dimensions.items.map((item) => item.value).sort()).toEqual(['eu-west', 'us-east'])

    const recentEvents = await listRawEvents(
      page,
      meter,
      new Date(recentTimestamp.getTime() - 60_000).toISOString(),
      new Date(recentTimestamp.getTime() + 60_000).toISOString(),
    )
    expect(recentEvents).toHaveLength(1)
    expect(recentEvents[0]).toMatchObject({ meter, quantity: 5, subject })

    await page.goto('/overview')
    const healthCard = cardNamed(page, 'Operations Health')
    await expect(healthCard.getByRole('row').filter({ hasText: 'retention' })).toContainText('healthy')

    const rollupCard = cardNamed(page, 'Retention Rollup Coverage')
    const meterRow = rollupCard.getByRole('row').filter({ hasText: meter })
    await expect(meterRow).toContainText('healthy')
    await expect(page.locator('main')).toContainText('Retention cleanup')
  })
})

async function createRetentionMeter(page: Page, meter: string) {
  const response = await page.request.post('/v1/meters', {
    data: {
      aggregation: 'sum',
      description: 'E2E retention rollup meter',
      dimensions: [{ name: 'region', required: true, type: 'string' }],
      event_retention_days: 1,
      name: meter,
      unit: 'request',
    },
  })
  expect(response.status()).toBe(201)
}

async function createUsage(page: Page, input: {
  idempotencyKey: string
  meter: string
  quantity: number
  region: string
  subject: string
  timestamp: string
}) {
  const response = await page.request.post('/v1/usages', {
    data: {
      idempotency_key: input.idempotencyKey,
      metadata: { region: input.region },
      meter: input.meter,
      quantity: input.quantity,
      subject: input.subject,
      timestamp: input.timestamp,
    },
  })
  expect(response.status()).toBe(201)
}

async function searchGroupedUsage(page: Page, meter: string, subject: string, from: string, to: string): Promise<UsageBucket[]> {
  const response = await page.request.post('/v1/usages/search', {
    data: {
      bucket_size: 'day',
      from,
      group_by: ['region'],
      limit: 100,
      meter,
      subject,
      to,
    },
  })
  expect(response.status()).toBe(200)
  return response.json() as Promise<UsageBucket[]>
}

async function listRawEvents(page: Page, meter: string, from: string, to: string): Promise<UsageEvent[]> {
  const response = await page.request.get('/v1/usageevents', {
    params: { from, limit: 100, meter, to },
  })
  expect(response.status()).toBe(200)
  const payload = await response.json() as { items: UsageEvent[] }
  return payload.items
}

async function systemStats(page: Page): Promise<SystemStats> {
  const response = await page.request.get('/v1/system/stats')
  expect(response.status()).toBe(200)
  return response.json() as Promise<SystemStats>
}

function bucketSignature(buckets: UsageBucket[]) {
  return buckets
    .map((bucket) => ({ quantity: bucket.quantity, region: bucket.group?.region || '' }))
    .sort((left, right) => left.region.localeCompare(right.region))
}

function cardNamed(page: Page, name: string) {
  return page.getByRole('heading', { name }).locator('xpath=ancestor::section[1]')
}

function startOfUTCDay(value: Date) {
  return new Date(Date.UTC(value.getUTCFullYear(), value.getUTCMonth(), value.getUTCDate()))
}
