import {
  expect,
  request as playwrightRequest,
  type APIRequestContext,
  type APIResponse,
  type Download,
  type Page,
} from '@playwright/test'

export type DashboardAccount = {
  email: string
  password: string
}

export type UsageScenario = {
  from: string
  meterName: string
  primarySubject: string
  timestamp: string
  to: string
}

export type CSVDownload = {
  filename: string
  text: string
}

export type CSVResponse = {
  headers: Record<string, string>
  status: number
  text: string
}

export type ExportJobResponse = {
  artifact_size?: number
  completed_at?: string
  created_at: string
  download_url?: string
  error?: string
  format: string
  id: string
  kind: string
  query: {
    bucket_size: string
    filter?: unknown
    from: string
    group_by?: string[]
    limit?: number
    meter: string
    subject?: string
    to: string
  }
  status: string
  updated_at: string
}

export const Given = {
  async aDashboardAccount(page: Page): Promise<DashboardAccount> {
    const id = uniqueID()
    const account = {
      email: `e2e-${id}@example.com`,
      password: `open-spanner-${id}`,
    }

    const response = await page.request.post('/v1/auth/users', { data: account })
    expect(response.status()).toBe(201)

    return account
  },

  async anAPIKeyExists(page: Page): Promise<string> {
    const response = await page.request.post('/v1/auth/api-keys', {
      data: { name: `e2e-export-${uniqueID()}` },
    })
    expect(response.status()).toBe(201)

    const payload = await response.json() as { key?: string }
    expect(payload.key).toBeTruthy()
    return payload.key || ''
  },

  async apiRequestUsageExists(page: Page, meterName: string): Promise<UsageScenario> {
    const subjectSuffix = uniqueID()
    const now = new Date()
    const timestamp = new Date(now.getTime() - 60_000)
    const from = toLocalDateTime(new Date(now.getTime() - 60 * 60_000))
    const to = toLocalDateTime(new Date(now.getTime() + 60 * 60_000))
    const primarySubject = `org_e2e_alpha_${subjectSuffix}`

    for (const [index, event] of [
      {
        metadata: {
          'region-name': 'us-east-1',
          service: { tier: 'gold' },
          status_code: '200',
        },
        quantity: 12,
        subject: primarySubject,
      },
      {
        metadata: {
          'region-name': 'eu-west-1',
          service: { tier: 'silver' },
          status_code: '201',
        },
        quantity: 4,
        subject: `org_e2e_beta_${subjectSuffix}`,
      },
    ].entries()) {
      const response = await page.request.post('/v1/usages', {
        data: {
          idempotency_key: `${meterName}-${index}`,
          meter: meterName,
          timestamp: timestamp.toISOString(),
          ...event,
        },
      })
      expect(response.status()).toBe(201)
    }

    return {
      from,
      meterName,
      primarySubject,
      timestamp: timestamp.toISOString(),
      to,
    }
  },
}

export const When = {
  async theUserSignsIn(page: Page, account: DashboardAccount) {
    await page.goto('/login')
    await page.getByLabel('Email').fill(account.email)
    await page.getByLabel('Password').fill(account.password)
    await page.getByRole('button', { name: 'Sign in' }).click()
  },

  async theUserCreatesAnAPIRequestMeter(page: Page, meterName: string) {
    await page.goto('/meters')
    await expect(page.getByRole('heading', { name: 'Meter definitions' })).toBeVisible()

    await page.getByRole('button', { name: 'New meter' }).click()
    await expect(page.getByRole('dialog', { name: 'Create Meter' })).toBeVisible()

    await page.locator('#meter-name').fill(meterName)
    await page.locator('#meter-unit').fill('request')
    await page.locator('#meter-description').fill('E2E API requests')
    await page.locator('select[name="aggregation"]').selectOption('sum')

    await page.getByTestId('meter-dimensions-toggle').click()
    await fillDimension(page, 0, {
      description: 'Serving region',
      displayName: 'Region',
      name: 'region-name',
    })

    await page.locator('.meter-create-form .schema-builder-actions').getByRole('button', { name: 'Add' }).click()
    await fillDimension(page, 1, {
      description: 'Service tier',
      displayName: 'Service Tier',
      name: 'service.tier',
    })

    await page.locator('.meter-create-form .schema-builder-actions').getByRole('button', { name: 'Add' }).click()
    await fillDimension(page, 2, {
      description: 'HTTP status code',
      displayName: 'Status Code',
      name: 'status_code',
    })

    await page.locator('.meter-create-form').getByRole('button', { name: 'Create meter' }).click()
  },

  async theUserQueriesUsageByServiceTier(page: Page, scenario: UsageScenario) {
    await page.goto('/usage')
    await expect(page.getByRole('heading', { name: 'Usage buckets' })).toBeVisible()

    await selectMeterFilter(page, scenario.meterName)
    await setUsageDateRange(page, scenario)

    await expect(page.getByRole('button', { name: 'Filter by Service Tier: gold' })).toBeVisible()
    await page.getByRole('button', { name: 'Filter by Service Tier: gold' }).click()
    await page.getByRole('checkbox', { name: 'Service Tier' }).check()
    await page.getByRole('button', { name: 'Run Query' }).click()
  },

  async theUserRunsAnAdvancedUsageQuery(page: Page, scenario: UsageScenario) {
    await page.goto('/usage')
    await expect(page.getByRole('heading', { name: 'Usage buckets' })).toBeVisible()

    await selectMeterFilter(page, scenario.meterName)
    await setUsageDateRange(page, scenario)
    await addQueryRule(page, {
      field: 'metadata.service.tier',
      operator: '=',
      value: 'gold',
    })
    await addQueryRule(page, {
      field: 'metadata.region-name',
      operator: '=',
      value: 'us-east-1',
    })
    await addQueryRule(page, {
      field: 'quantity',
      operator: '>',
      value: '10',
    })

    await page.getByRole('checkbox', { name: 'Region' }).check()
    await page.getByRole('checkbox', { name: 'Service Tier' }).check()
    await page.getByLabel('Saved query name').fill(`Gold API traffic ${scenario.meterName}`)
    await page.getByRole('button', { name: 'Save' }).click()
    await expect(page.getByRole('button', { name: 'Update' })).toBeVisible()
    await page.getByRole('button', { name: 'Pin' }).click()
    await expect(page.getByRole('button', { name: 'Unpin' })).toBeVisible()
    await page.getByRole('button', { name: 'Run Query' }).click()
  },

  async theUserOpensUsageFromSubjectActivity(page: Page, scenario: UsageScenario) {
    await page.goto(`/subjects/${scenario.primarySubject}`)
    await expect(page.getByRole('heading', { name: scenario.primarySubject })).toBeVisible()
    await expect(page.locator('.subject-meter-list')).toContainText(scenario.meterName)
    await page.getByRole('button', { name: 'Open Usage' }).click()
  },

  async theUserExportsCurrentUsageBuckets(page: Page): Promise<CSVDownload> {
    const downloadPromise = page.waitForEvent('download')
    await page.getByRole('button', { name: 'Export Buckets' }).click()
    return csvDownload(await downloadPromise)
  },

  async theUserExportsCurrentUsageEvents(page: Page): Promise<CSVDownload> {
    const downloadPromise = page.waitForEvent('download')
    await page.getByRole('button', { name: 'Export Events' }).click()
    return csvDownload(await downloadPromise)
  },

  async theUserViewsCurrentUsageEvents(page: Page) {
    await page.getByRole('button', { name: 'View Events' }).click()
  },

  async theUserOpensFirstUsageEvent(page: Page) {
    await page.locator('.usage-events-card').getByRole('button', { name: 'Details' }).first().click()
  },

  async theUserClosesUsageEventDetails(page: Page) {
    await page.getByRole('button', { name: 'Close event details' }).click()
    await expect(page.locator('.usage-event-drawer')).toHaveCount(0)
  },

  async theUserQueuesCurrentUsageExport(page: Page) {
    await page.getByRole('button', { name: 'Queue Export' }).click()
    await expect(page.locator('.usage-export-card')).toContainText('Usage buckets')
  },

  async theUserExportsSubjectEvents(page: Page, scenario: UsageScenario): Promise<CSVDownload> {
    await page.goto(`/subjects/${scenario.primarySubject}`)
    await expect(page.getByRole('heading', { name: scenario.primarySubject })).toBeVisible()
    await expect(page.locator('.subject-meter-list')).toContainText(scenario.meterName)

    const downloadPromise = page.waitForEvent('download')
    await page.getByRole('button', { name: 'Export Events' }).click()
    return csvDownload(await downloadPromise)
  },

  async theServiceExportsFilteredUsageBucketsWithAPIKey(page: Page, apiKey: string, scenario: UsageScenario): Promise<CSVResponse> {
    return withAPIKeyContext(page, apiKey, async (api) => {
      const response = await api.post('/v1/usages/export', {
        data: {
          bucket_size: 'day',
          filter: {
            field: 'metadata.service.tier',
            op: 'eq',
            type: 'condition',
            value: 'gold',
          },
          from: apiWindowForScenario(scenario).from,
          group_by: ['service.tier'],
          limit: 100,
          meter: scenario.meterName,
          to: apiWindowForScenario(scenario).to,
        },
      })
      return csvResponse(response)
    })
  },

  async theServiceExportsSubjectEventsWithAPIKey(page: Page, apiKey: string, scenario: UsageScenario): Promise<CSVResponse> {
    return withAPIKeyContext(page, apiKey, async (api) => {
      const response = await api.post('/v1/usageevents/export', {
        data: {
          filter: {
            field: 'quantity',
            op: 'gte',
            type: 'condition',
            value: 1,
          },
          from: apiWindowForScenario(scenario).from,
          limit: 100,
          meter: scenario.meterName,
          subject: scenario.primarySubject,
          to: apiWindowForScenario(scenario).to,
        },
      })
      return csvResponse(response)
    })
  },

  async theServiceQueuesUsageExportJob(page: Page, apiKey: string, scenario: UsageScenario): Promise<ExportJobResponse> {
    return withAPIKeyContext(page, apiKey, async (api) => {
      const response = await api.post('/v1/exports', {
        data: {
          format: 'csv',
          kind: 'usage_buckets',
          query: {
            bucket_size: 'day',
            filter: {
              field: 'metadata.service.tier',
              op: 'eq',
              type: 'condition',
              value: 'gold',
            },
            from: apiWindowForScenario(scenario).from,
            group_by: ['service.tier'],
            limit: 100,
            meter: scenario.meterName,
            to: apiWindowForScenario(scenario).to,
          },
        },
      })
      expect(response.status()).toBe(202)
      return response.json() as Promise<ExportJobResponse>
    })
  },
}

export const Then = {
  async theDashboardIsAvailable(page: Page, account: DashboardAccount) {
    await expect(page).toHaveURL(/\/overview$/)
    await expect(page.getByText(account.email)).toBeVisible()
  },

  async theMeterIsVisible(page: Page, meterName: string) {
    await expect(page.locator('.meter-table-card')).toContainText(meterName)
    await expect(page.locator('.meter-table-card')).toContainText('Service Tier')
  },

  async theUsagePageLoadsWithoutDimensionErrors(page: Page) {
    await expect(page.getByText(/unsupported breakdown field/i)).toHaveCount(0)
    await expect(page.getByText(/unable to load usage breakdowns/i)).toHaveCount(0)
  },

  async bucketedUsageIncludesGoldServiceTier(page: Page, meterName: string) {
    const results = page.locator('.usage-results-card')
    await expect(results).toContainText(meterName)
    await expect(results).toContainText('service.tier')
    await expect(results).toContainText('gold')
    await expect(results).toContainText('12')
  },

  async advancedQueryReturnsOnlyMatchingUsage(page: Page, meterName: string) {
    const results = page.locator('.usage-results-card')
    await expect(results).toContainText(meterName)
    await expect(results).toContainText('region-name')
    await expect(results).toContainText('us-east-1')
    await expect(results).toContainText('service.tier')
    await expect(results).toContainText('gold')
    await expect(results).toContainText('12')
    await expect(results).not.toContainText('silver')
    await expect(results).not.toContainText('eu-west-1')
  },

  async advancedUsageBucketCSVIncludesMatchingUsage(download: CSVDownload, scenario: UsageScenario) {
    expect(download.filename).toMatch(/^usage-buckets-.+\.csv$/)
    expect(download.text).toContain('bucket_start,subject,meter,bucket_size,aggregation,unit,quantity')
    expect(download.text).toContain('region-name')
    expect(download.text).toContain('service.tier')
    expect(download.text).toContain(scenario.meterName)
    expect(download.text).toContain('us-east-1')
    expect(download.text).toContain('gold')
    expect(download.text).toContain(',12,')
    expect(download.text).not.toContain('eu-west-1')
    expect(download.text).not.toContain('silver')
  },

  async rawUsageEventsIncludeOnlyMatchingUsage(page: Page, scenario: UsageScenario) {
    const events = page.locator('.usage-events-card')
    await expect(events).toContainText(scenario.primarySubject)
    await expect(events).toContainText(scenario.meterName)
    await expect(events).toContainText('region-name')
    await expect(events).toContainText('us-east-1')
    await expect(events).toContainText('service')
    await expect(events).toContainText('gold')
    await expect(events).toContainText('12')
    await expect(events).not.toContainText('silver')
    await expect(events).not.toContainText('eu-west-1')
  },

  async usageEventDetailsIncludeMatchingUsage(page: Page, scenario: UsageScenario) {
    const drawer = page.locator('.usage-event-drawer')
    await expect(drawer).toBeVisible()
    await expect(drawer).toContainText(scenario.primarySubject)
    await expect(drawer).toContainText(scenario.meterName)
    await expect(drawer).toContainText('region-name')
    await expect(drawer).toContainText('us-east-1')
    await expect(drawer).toContainText('service')
    await expect(drawer).toContainText('gold')
    await expect(drawer).toContainText('12')
    await expect(drawer.getByRole('button', { name: 'Copy event ID' })).toBeVisible()
    await expect(drawer.getByRole('button', { name: 'Copy metadata' })).toBeVisible()
  },

  async usageQueryIsScopedToSubjectAndMeter(page: Page, scenario: UsageScenario) {
    await expect(page).toHaveURL(/\/usage$/)
    await expect.poll(() => queryRulePairs(page)).toEqual(expect.arrayContaining([
      `meter:${scenario.meterName}`,
      `subject:${scenario.primarySubject}`,
    ]))
  },

  async usageBucketCSVIncludesCurrentQuery(download: CSVDownload, scenario: UsageScenario) {
    expect(download.filename).toMatch(/^usage-buckets-.+\.csv$/)
    expect(download.text).toContain('bucket_start,subject,meter,bucket_size,aggregation,unit,quantity,service.tier')
    expect(download.text).toContain(scenario.meterName)
    expect(download.text).toContain('gold')
    expect(download.text).toContain(',12,')
    expect(download.text).not.toContain('silver')
  },

  async subjectEventCSVIncludesPrimaryUsage(download: CSVDownload, scenario: UsageScenario) {
    expect(download.filename).toMatch(/^org_e2e_alpha_.+-usage-events\.csv$/)
    expect(download.text).toContain('timestamp,received_at,subject,meter,quantity,metadata,id,idempotency_key')
    expect(download.text).toContain(scenario.primarySubject)
    expect(download.text).toContain(scenario.meterName)
    expect(download.text).toContain('region-name')
    expect(download.text).toContain('us-east-1')
    expect(download.text).toContain(',12,')
  },

  async currentUsageEventCSVIncludesCurrentQuery(download: CSVDownload, scenario: UsageScenario) {
    expect(download.filename).toMatch(new RegExp(`^usage-events-${scenario.meterName}\\.csv$`))
    expect(download.text).toContain('timestamp,received_at,subject,meter,quantity,metadata,id,idempotency_key')
    expect(download.text).toContain(scenario.primarySubject)
    expect(download.text).toContain(scenario.meterName)
    expect(download.text).toContain('region-name')
    expect(download.text).toContain('us-east-1')
    expect(download.text).toContain('gold')
    expect(download.text).toContain(',12,')
    expect(download.text).not.toContain('silver')
    expect(download.text).not.toContain('eu-west-1')
  },

  async queuedUsageExportCompletesInDashboard(page: Page, scenario: UsageScenario): Promise<CSVDownload> {
    const row = page.locator('.usage-export-card .export-job-row', { hasText: scenario.meterName }).first()
    await expect(row).toContainText('Completed', { timeout: 20_000 })

    const downloadPromise = page.waitForEvent('download')
    await row.getByRole('button', { name: 'Download' }).click()
    return csvDownload(await downloadPromise)
  },

  async queuedUsageBucketCSVIncludesCurrentQuery(download: CSVDownload, scenario: UsageScenario) {
    expect(download.filename).toMatch(/^usage-export-.+\.csv$/)
    expect(download.text).toContain('bucket_start,subject,meter,bucket_size,aggregation,unit,quantity,service.tier')
    expect(download.text).toContain(scenario.meterName)
    expect(download.text).toContain('gold')
    expect(download.text).toContain(',12,')
    expect(download.text).not.toContain('silver')
  },

  async theExportsPageShowsCompletedJob(page: Page, scenario: UsageScenario) {
    await page.goto('/exports')
    await expect(page.getByRole('heading', { name: 'Export jobs' })).toBeVisible()

    const filters = page.locator('.exports-filter-bar')
    await expect(filters.getByRole('button', { name: /Completed/ })).toBeVisible()
    await filters.getByRole('button', { name: /Completed/ }).click()

    const jobs = page.locator('.usage-export-card')
    await expect(jobs).toContainText(scenario.meterName)
    await expect(jobs).toContainText('Completed')
    await expect(jobs.getByRole('button', { name: 'Download' })).toBeVisible()
  },

  async directUsageBucketCSVResponseIncludesCurrentQuery(response: CSVResponse, scenario: UsageScenario) {
    expect(response.status).toBe(200)
    expect(response.headers['content-type']).toContain('text/csv')
    expect(response.headers['content-disposition']).toContain('attachment')
    expect(response.headers['content-disposition']).toContain('usage-buckets.csv')
    expect(response.text).toContain('bucket_start,subject,meter,bucket_size,aggregation,unit,quantity,service.tier')
    expect(response.text).toContain(scenario.meterName)
    expect(response.text).toContain('gold')
    expect(response.text).toContain(',12,')
    expect(response.text).not.toContain('silver')
  },

  async directUsageEventCSVResponseIncludesPrimaryUsage(response: CSVResponse, scenario: UsageScenario) {
    expect(response.status).toBe(200)
    expect(response.headers['content-type']).toContain('text/csv')
    expect(response.headers['content-disposition']).toContain('attachment')
    expect(response.headers['content-disposition']).toContain('usage-events.csv')
    expect(response.text).toContain('timestamp,received_at,subject,meter,quantity,metadata,id,idempotency_key')
    expect(response.text).toContain(scenario.primarySubject)
    expect(response.text).toContain(scenario.meterName)
    expect(response.text).toContain('region-name')
    expect(response.text).toContain('us-east-1')
    expect(response.text).toContain(',12,')
    expect(response.text).not.toContain('eu-west-1')
  },

  async queuedExportJobCompletesAndDownloads(page: Page, apiKey: string, job: ExportJobResponse, scenario: UsageScenario) {
    expect(job.id).toBeTruthy()
    expect(job.kind).toBe('usage_buckets')
    expect(job.status).toBe('queued')
    expect(job.format).toBe('csv')
    expect(job.created_at).toBeTruthy()
    expect(job.updated_at).toBeTruthy()
    expect(job.completed_at || '').toBe('')
    expect(job.query.meter).toBe(scenario.meterName)
    expect(job.query.bucket_size).toBe('day')

    await withAPIKeyContext(page, apiKey, async (api) => {
      const completed = await waitForCompletedExportJob(api, job.id)
      expect(completed.id).toBe(job.id)
      expect(completed.completed_at).toBeTruthy()
      expect(completed.download_url).toBe(`/v1/exports/${job.id}/download`)
      expect(completed.artifact_size || 0).toBeGreaterThan(0)

      const listResponse = await api.get('/v1/exports?limit=10')
      expect(listResponse.status()).toBe(200)
      const list = await listResponse.json() as { items: ExportJobResponse[] }
      expect(list.items.some((item) => item.id === job.id)).toBe(true)

      const downloadResponse = await api.get(`/v1/exports/${job.id}/download`)
      const csv = await csvResponse(downloadResponse)
      await Then.queuedUsageBucketCSVResponseIncludesCurrentQuery(csv, scenario, job.id)
    })
  },

  async queuedUsageBucketCSVResponseIncludesCurrentQuery(response: CSVResponse, scenario: UsageScenario, jobID: string) {
    expect(response.status).toBe(200)
    expect(response.headers['content-type']).toContain('text/csv')
    expect(response.headers['content-disposition']).toContain('attachment')
    expect(response.headers['content-disposition']).toContain(`open-spanner-export-${jobID}.csv`)
    expect(response.text).toContain('bucket_start,subject,meter,bucket_size,aggregation,unit,quantity,service.tier')
    expect(response.text).toContain(scenario.meterName)
    expect(response.text).toContain('gold')
    expect(response.text).toContain(',12,')
    expect(response.text).not.toContain('silver')
  },
}

async function fillDimension(
  page: Page,
  index: number,
  dimension: { description: string; displayName: string; name: string },
) {
  const row = page.locator('.meter-create-form .schema-row').nth(index)
  await row.getByLabel('Dimension name').fill(dimension.name)
  await row.getByLabel('Dimension display name').fill(dimension.displayName)
  await row.getByLabel('Dimension description').fill(dimension.description)
  await row.getByLabel('Dimension type').selectOption('string')
}

async function selectMeterFilter(page: Page, meterName: string) {
  const firstRule = page.locator('.filter-builder .rule').first()
  await firstRule.locator('.rule-fields').selectOption('meter')
  await firstRule.locator('.rule-operators').selectOption('=')
  await firstRule.locator('.rule-value').selectOption(meterName)
}

async function setUsageDateRange(page: Page, scenario: UsageScenario) {
  const dateInputs = page.locator('input[type="datetime-local"]')
  await expect(dateInputs).toHaveCount(2)
  await dateInputs.nth(0).fill(scenario.from)
  await dateInputs.nth(1).fill(scenario.to)
  await page.getByRole('heading', { name: 'Usage buckets' }).click()
  await expect(dateInputs.nth(0)).toHaveValue(scenario.from)
  await expect(dateInputs.nth(1)).toHaveValue(scenario.to)
}

async function addQueryRule(
  page: Page,
  rule: { field: string; operator: string; value: string },
) {
  const filterBuilder = page.locator('.filter-builder')
  await filterBuilder.locator('.ruleGroup-addRule').first().click()

  const row = filterBuilder.locator('.rule').last()
  await row.locator('.rule-fields').selectOption(rule.field)
  await row.locator('.rule-operators').selectOption(rule.operator)
  await setRuleValue(row, rule.value)
}

async function setRuleValue(row: ReturnType<Page['locator']>, value: string) {
  const valueControl = row.locator('.rule-value')
  if (await valueControl.evaluate((element) => element.tagName.toLowerCase() === 'select')) {
    await valueControl.selectOption(value)
    return
  }
  await valueControl.fill(value)
}

async function queryRulePairs(page: Page) {
  return page.locator('.filter-builder .rule').evaluateAll((rows) => rows.map((row) => {
    const field = (row.querySelector('.rule-fields') as HTMLSelectElement | null)?.value || ''
    const value = (row.querySelector('.rule-value') as HTMLInputElement | HTMLSelectElement | null)?.value || ''
    return `${field}:${value}`
  }))
}

async function csvDownload(download: Download): Promise<CSVDownload> {
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

async function csvResponse(response: APIResponse): Promise<CSVResponse> {
  return {
    headers: response.headers(),
    status: response.status(),
    text: await response.text(),
  }
}

async function waitForCompletedExportJob(api: APIRequestContext, id: string): Promise<ExportJobResponse> {
  let latest: ExportJobResponse | null = null

  await expect.poll(async () => {
    const response = await api.get(`/v1/exports/${id}`)
    expect(response.status()).toBe(200)
    latest = await response.json() as ExportJobResponse
    return latest.status
  }, {
    intervals: [250, 500, 1000],
    timeout: 20_000,
  }).toBe('completed')

  return latest as ExportJobResponse
}

async function withAPIKeyContext<T>(
  page: Page,
  apiKey: string,
  callback: (api: APIRequestContext) => Promise<T>,
): Promise<T> {
  const api = await playwrightRequest.newContext({
    baseURL: new URL(page.url()).origin,
    extraHTTPHeaders: {
      Authorization: `Bearer ${apiKey}`,
    },
  })
  try {
    return await callback(api)
  } finally {
    await api.dispose()
  }
}

function apiWindowForScenario(scenario: UsageScenario) {
  const eventTime = Date.parse(scenario.timestamp)
  return {
    from: new Date(eventTime - 60 * 60_000).toISOString(),
    to: new Date(eventTime + 60 * 60_000).toISOString(),
  }
}

function toLocalDateTime(date: Date) {
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60_000)
  return local.toISOString().slice(0, 16)
}

function uniqueID() {
  return `${Date.now()}-${Math.random().toString(16).slice(2)}`
    .replace(/[^a-z0-9]/gi, '')
    .slice(0, 24)
}
