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

export type ScopedAPIKeyScenario = {
  allowedMeter: string
  deniedMeter: string
  key: string
  name: string
  subject: string
}

export type WorkspaceIsolationScenario = {
  alertID: string
  alertName: string
  apiKeyID: string
  apiKeyName: string
  destinationName: string
  exportID: string
  meterName: string
  subject: string
}

export type PlanEntitlementScenario = {
  current: number
  eventType: string
  limit: number
  meterName: string
  planID: string
  planName: string
  subject: string
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
      data: {
        name: `e2e-export-${uniqueID()}`,
        scopes: ['usage:read', 'exports:read', 'exports:write'],
      },
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

  async aPlanEntitlementWarningExists(page: Page): Promise<PlanEntitlementScenario> {
    const id = uniqueID()
    const current = 7
    const limit = 10
    const meterName = `entitlement_api_calls_${id}`
    const planName = `Entitlement Pro ${id}`
    const subject = `org_entitlement_${id}`

    const meterResponse = await page.request.post('/v1/meters', {
      data: {
        aggregation: 'sum',
        description: 'E2E entitlement meter',
        dimensions: [],
        event_retention_days: 30,
        name: meterName,
        unit: 'call',
      },
    })
    expect(meterResponse.status()).toBe(201)

    const planResponse = await page.request.post('/v1/plans', {
      data: {
        description: 'E2E entitlement quota plan',
        limits: [
          {
            limit,
            meter: meterName,
            period: 'month',
            warning_percent: 60,
          },
        ],
        name: planName,
      },
    })
    expect(planResponse.status()).toBe(201)
    const plan = await planResponse.json() as { id: string }

    const assignmentResponse = await page.request.put(`/v1/plans/subjects/${encodeURIComponent(subject)}`, {
      data: {
        plan_id: plan.id,
      },
    })
    expect(assignmentResponse.status()).toBe(200)

    const usageResponse = await page.request.post('/v1/usages', {
      data: {
        idempotency_key: `entitlement-warning-${id}`,
        metadata: {
          endpoint: '/plans',
        },
        meter: meterName,
        quantity: current,
        subject,
        timestamp: new Date().toISOString(),
      },
    })
    expect(usageResponse.status()).toBe(201)

    const query = new URLSearchParams({
      limit: '10',
      meter: meterName,
      subject,
    })
    await waitForEntitlementState(page, `/v1/entitlements/states?${query.toString()}`, 'warning')
    await waitForEntitlementState(page, `/v1/entitlements/events?${query.toString()}`, 'warning')

    return {
      current,
      eventType: 'warning',
      limit,
      meterName,
      planID: plan.id,
      planName,
      subject,
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
    const dialog = page.getByRole('dialog', { name: 'Create Meter' })
    await expect(dialog).toBeVisible()

    await dialog.locator('#meter-name').fill(meterName)
    await dialog.locator('#meter-unit').fill('request')
    await dialog.locator('#meter-description').fill('E2E API requests')

    await dialog.getByTestId('meter-dimensions-toggle').click()
    await fillDimension(page, 0, {
      description: 'Serving region',
      displayName: 'Region',
      name: 'region-name',
    })

    await dialog.locator('.schema-builder-actions').getByRole('button', { name: 'Add' }).click()
    await fillDimension(page, 1, {
      description: 'Service tier',
      displayName: 'Service Tier',
      name: 'service.tier',
    })

    await dialog.locator('.schema-builder-actions').getByRole('button', { name: 'Add' }).click()
    await fillDimension(page, 2, {
      description: 'HTTP status code',
      displayName: 'Status Code',
      name: 'status_code',
    })

    await dialog.getByRole('button', { name: 'Create meter' }).click()
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
    const queryName = `Gold API traffic ${scenario.meterName}`
    await page.getByLabel('Saved query name').fill(queryName)
    await expect(page.getByLabel('Saved query name')).toHaveValue(queryName)
    await page.getByRole('button', { name: 'Save' }).click()
    await expectSavedUsageQuery(page, queryName)
    await page.getByRole('button', { name: 'Run Query' }).click()
  },

  async theUserChangesUsageChartControls(page: Page) {
    const chart = page.locator('.usage-chart-card')
    await expect(chart).toBeVisible()

    await selectControlOption(page, chart.getByLabel('Chart bucket'), 'hour')
    await selectControlOption(page, chart.getByLabel('Chart type'), 'area')

    const stacked = chart.getByLabel('Stack chart series')
    if (await stacked.isEnabled()) {
      await stacked.check()
    }

    const cumulative = chart.getByLabel('Cumulative chart')
    if (!await cumulative.isChecked()) {
      await cumulative.check()
    }

    const points = chart.getByLabel('Show chart points')
    if (!await points.isChecked()) {
      await points.check()
    }
  },

  async theUserOpensUsageFromSubjectActivity(page: Page, scenario: UsageScenario) {
    await page.goto(`/subjects/${scenario.primarySubject}`)
    await expect(page.getByRole('heading', { name: scenario.primarySubject })).toBeVisible()
    await expect(page.locator('.subject-meter-list')).toContainText(scenario.meterName)
    await page.getByRole('button', { name: 'Open Usage' }).click()
  },

  async theUserOpensPlan(page: Page, scenario: PlanEntitlementScenario) {
    await page.goto(`/plans/${scenario.planID}`)
    await expect(page.getByRole('heading', { name: scenario.planName })).toBeVisible()
  },

  async theUserOpensSubjectActivityForEntitlement(page: Page, scenario: PlanEntitlementScenario) {
    await page.goto(`/subjects/${scenario.subject}`)
    await expect(page.getByRole('heading', { name: scenario.subject })).toBeVisible()
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
    await expect(page.locator('main')).toContainText('Usage buckets')
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

  async theServiceQueuesFailingUsageExportJob(page: Page, apiKey: string, scenario: UsageScenario): Promise<ExportJobResponse> {
    return withAPIKeyContext(page, apiKey, async (api) => {
      const response = await api.post('/v1/exports', {
        data: {
          format: 'csv',
          kind: 'usage_buckets',
          query: {
            bucket_size: 'day',
            from: apiWindowForScenario(scenario).from,
            limit: 100,
            meter: `missing_export_meter_${uniqueID()}`,
            to: apiWindowForScenario(scenario).to,
          },
        },
      })
      expect(response.status()).toBe(202)
      return response.json() as Promise<ExportJobResponse>
    })
  },

  async theUserCreatesAScopedUsageWriteAPIKey(page: Page, allowedMeter: string): Promise<ScopedAPIKeyScenario> {
    const id = uniqueID()
    const scenario = {
      allowedMeter,
      deniedMeter: `denied_api_requests_${id}`,
      key: '',
      name: `usage-writer-${id}`,
      subject: `org_scoped_key_${id}`,
    }

    await page.goto('/api-keys')
    await expect(page.getByRole('heading', { name: 'SDK access' })).toBeVisible()
    await page.getByRole('button', { name: 'New key' }).click()
    await expect(page.getByRole('dialog', { name: 'Create API Key' })).toBeVisible()

    await page.locator('input[name="name"]').fill(scenario.name)
    await setScope(page, 'usage:write', true)
    await setScope(page, 'usage:read', false)
    await setScope(page, 'meters:read', false)
    await setScope(page, 'meters:write', false)
    await page.locator('textarea[name="allowed_meters"]').fill(allowedMeter)
    await page.getByRole('button', { name: 'Create key' }).click()

    const secretPanel = page.locator('section[aria-label="Created API key"]')
    await expect(secretPanel).toBeVisible()
    await expect(secretPanel).toContainText(scenario.name)
    const key = (await secretPanel.locator('code').textContent())?.trim()
    expect(key).toBeTruthy()
    scenario.key = key || ''

    const table = page.locator('main')
    await expect(table).toContainText(scenario.name)
    await expect(table).toContainText('Write usage')
    await expect(table).toContainText(`meters: ${allowedMeter}`)

    return scenario
  },

  async theScopedAPIKeyWritesAllowedUsage(page: Page, scenario: ScopedAPIKeyScenario) {
    await withAPIKeyContext(page, scenario.key, async (api) => {
      const response = await api.post('/v1/usages', {
        data: {
          idempotency_key: `scoped-key-allowed-${uniqueID()}`,
          metadata: {
            'region-name': 'us-east-1',
            service: { tier: 'gold' },
            status_code: '200',
          },
          meter: scenario.allowedMeter,
          quantity: 1,
          subject: scenario.subject,
          timestamp: new Date().toISOString(),
        },
      })
      expect(response.status()).toBe(201)
    })
  },

  async theScopedAPIKeyAttemptsDeniedUsage(page: Page, scenario: ScopedAPIKeyScenario) {
    await withAPIKeyContext(page, scenario.key, async (api) => {
      const writeResponse = await api.post('/v1/usages', {
        data: {
          idempotency_key: `scoped-key-denied-${uniqueID()}`,
          metadata: { endpoint: '/denied' },
          meter: scenario.deniedMeter,
          quantity: 1,
          subject: scenario.subject,
          timestamp: new Date().toISOString(),
        },
      })
      expect(writeResponse.status()).toBe(403)

      const readQuery = new URLSearchParams({
        bucket_size: 'day',
        from: new Date(Date.now() - 60 * 60_000).toISOString(),
        limit: '100',
        meter: scenario.allowedMeter,
        to: new Date(Date.now() + 60 * 60_000).toISOString(),
      })
      const readResponse = await api.get(`/v1/usages?${readQuery.toString()}`)
      expect(readResponse.status()).toBe(403)
    })
  },

  async theSignedInUserCreatesWorkspaceOwnedResources(page: Page): Promise<WorkspaceIsolationScenario> {
    const id = uniqueID()
    const meterName = `workspace_meter_${id}`
    const subject = `workspace_subject_${id}`
    const apiKeyName = `workspace-sdk-${id}`
    const destinationName = `Workspace webhook ${id}`
    const alertName = `Workspace threshold ${id}`

    const keyResponse = await page.request.post('/v1/auth/api-keys', {
      data: {
        name: apiKeyName,
        scopes: ['usage:write', 'usage:read', 'meters:read'],
      },
    })
    expect(keyResponse.status()).toBe(201)
    const key = await keyResponse.json() as { id: string }

    const meterResponse = await page.request.post('/v1/meters', {
      data: {
        aggregation: 'sum',
        description: 'Workspace isolation meter',
        dimensions: [
          {
            deprecated: false,
            description: 'Endpoint path',
            display_name: 'Endpoint',
            name: 'endpoint',
            required: true,
            type: 'string',
          },
        ],
        event_retention_days: 30,
        name: meterName,
        unit: 'request',
      },
    })
    expect(meterResponse.status()).toBe(201)

    const usageResponse = await page.request.post('/v1/usages', {
      data: {
        idempotency_key: `workspace-isolation-${id}`,
        metadata: { endpoint: '/workspace-owner' },
        meter: meterName,
        quantity: 3,
        subject,
        timestamp: new Date().toISOString(),
      },
    })
    expect(usageResponse.status()).toBe(201)

    const destinationResponse = await page.request.post('/v1/alerts/destinations', {
      data: {
        enabled: true,
        name: destinationName,
        type: 'webhook',
        webhook_url: 'https://example.com/open-spanner/workspace-isolation',
      },
    })
    expect(destinationResponse.status()).toBe(201)
    const destination = await destinationResponse.json() as { id: string }

    const alertResponse = await page.request.post('/v1/alerts', {
      data: {
        comparator: 'gte',
        destination_id: destination.id,
        enabled: true,
        evaluation_interval_seconds: 60,
        meter: meterName,
        name: alertName,
        threshold: 1,
        window_seconds: 3600,
      },
    })
    expect(alertResponse.status()).toBe(201)
    const alert = await alertResponse.json() as { id: string }

    const exportResponse = await page.request.post('/v1/exports', {
      data: {
        format: 'csv',
        kind: 'usage_buckets',
        query: {
          bucket_size: 'day',
          from: new Date(Date.now() - 60 * 60_000).toISOString(),
          limit: 100,
          meter: meterName,
          to: new Date(Date.now() + 60 * 60_000).toISOString(),
        },
      },
    })
    expect(exportResponse.status()).toBe(202)
    const exportJob = await exportResponse.json() as { id: string }

    const scenario = {
      alertID: alert.id,
      alertName,
      apiKeyID: key.id,
      apiKeyName,
      destinationName,
      exportID: exportJob.id,
      meterName,
      subject,
    }

    await waitForWorkspaceResources(page, scenario)

    return scenario
  },

  async theUserSignsOut(page: Page) {
    await page.request.delete('/v1/auth/session')
    await page.goto('/login')
  },
}

export const Then = {
  async theDashboardIsAvailable(page: Page, account: DashboardAccount) {
    await expect(page).toHaveURL(/\/overview$/)
    await expect(page.getByText(account.email)).toBeVisible()
  },

  async expiredDashboardSessionRedirectsCleanly(page: Page) {
    await page.request.delete('/v1/auth/session')
    await page.goto('/overview')

    await expect(page).toHaveURL(/\/login/)
    await expect(page.getByLabel('Email')).toBeVisible()
    await expect(page.getByLabel('Password')).toBeVisible()
    await expect(page.locator('body')).not.toContainText('Unauthorized')
    await expect(page.locator('body')).not.toContainText('Bad Gateway')
    await expect(page.locator('body')).not.toContainText('internal server error')
    await expect(page.locator('body')).not.toContainText('Not Found')
  },

  async theMeterIsVisible(page: Page, meterName: string) {
    await expect(page.locator('main')).toContainText(meterName)
    await expect(page.locator('main')).toContainText('Service Tier')
  },

  async meterDetailIsVisible(page: Page, meterName: string) {
    await page.goto(`/meters/${meterName}`)
    await expect(page.getByRole('heading', { name: meterName })).toBeVisible()
    await expect(page.locator('main')).toContainText('Dimensions')
    await expect(page.locator('main')).toContainText('Service Tier')
    await expect(page.getByRole('button', { name: 'Analyze usage' })).toBeVisible()
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
    await Then.usageChartShowsCurrentBuckets(page)
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
    await Then.usageChartShowsCurrentBuckets(page)
  },

  async usageChartShowsCurrentBuckets(page: Page) {
    const chart = page.locator('.usage-chart-card')
    await expect(chart).toContainText('Usage Over Time')
    await expect(chart.getByLabel('Chart bucket')).toBeVisible()
    await expect(chart.getByLabel('Chart type')).toBeVisible()
    await expect(chart.getByLabel('Cumulative chart')).toBeVisible()
    await expect(chart.locator('canvas')).toBeVisible()
    await expect(chart).toContainText('12')
  },

  async usageChartControlsAreApplied(page: Page) {
    const chart = page.locator('.usage-chart-card')

    await expect(chart.getByLabel('Chart bucket')).toContainText('Hour')
    await expect(chart.getByLabel('Chart type')).toContainText('Filled Area')
    await expect(chart.getByLabel('Cumulative chart')).toBeChecked()
    await expect(chart.getByLabel('Show chart points')).toBeChecked()
    if (await chart.getByLabel('Stack chart series').isEnabled()) {
      await expect(chart.getByLabel('Stack chart series')).toBeChecked()
      await expect(chart.getByLabel('Usage chart summary')).toContainText(/Stacked|Stack needs 2\+ series/)
    }
    await expect(chart).toContainText('Cumulative')
    await expect(chart.locator('canvas')).toBeVisible()
    await expect(chart).toContainText('12')
  },

  async usageFiltersRemainReadable(page: Page) {
    const originalViewport = page.viewportSize()

    for (const viewport of [
      { height: 900, width: 1280 },
      { height: 900, width: 640 },
    ]) {
      await page.setViewportSize(viewport)
      const filterBuilder = page.locator('.filter-builder')
      await expect(filterBuilder).toBeVisible()

      const layout = await filterBuilder.evaluate((node) => {
        const container = node.getBoundingClientRect()
        const controls = Array.from(node.querySelectorAll('button,input')).map((control) => {
          const rect = control.getBoundingClientRect()
          return {
            bottom: rect.bottom,
            height: rect.height,
            left: rect.left,
            right: rect.right,
            top: rect.top,
            width: rect.width,
          }
        })
        const rules = Array.from(node.querySelectorAll('.rule')).map((rule) => {
          const rect = rule.getBoundingClientRect()
          return {
            bottom: rect.bottom,
            left: rect.left,
            right: rect.right,
            top: rect.top,
          }
        })

        return {
          controlsInside: controls.every((control) => control.left >= container.left - 1 && control.right <= container.right + 1),
          horizontalOverflow: Math.ceil(node.scrollWidth - node.clientWidth),
          readableControls: controls.every((control) => control.width >= 28 && control.height >= 28),
          rulesInside: rules.every((rule) => rule.left >= container.left - 1 && rule.right <= container.right + 1),
        }
      })

      expect(layout.horizontalOverflow).toBeLessThanOrEqual(2)
      expect(layout.controlsInside).toBe(true)
      expect(layout.rulesInside).toBe(true)
      expect(layout.readableControls).toBe(true)
    }

    if (originalViewport) {
      await page.setViewportSize(originalViewport)
    }
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
    const filterBuilder = page.locator('.filter-builder')
    await expect(filterBuilder.locator('.rule').first()).toContainText('Meter')
    await expect(filterBuilder.locator('.rule').first()).toContainText(scenario.meterName)
    const subjectRule = filterBuilder.locator('.rule').last()
    await expect(subjectRule).toContainText('Subject')
    await expect(subjectRule.locator('.rule-value')).toHaveValue(scenario.primarySubject)
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
    const row = page.locator('.export-job-row', { hasText: scenario.meterName }).first()
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

    const job = page.locator('.export-job-row', { hasText: scenario.meterName }).first()
    await expect(job).toContainText('Completed')
    await expect(job.getByRole('button', { name: 'Download' })).toBeVisible()
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

  async failedExportDoesNotBlockCompletedExports(
    page: Page,
    apiKey: string,
    failedJob: ExportJobResponse,
    completedJob: ExportJobResponse,
    scenario: UsageScenario,
  ) {
    await withAPIKeyContext(page, apiKey, async (api) => {
      const failed = await waitForExportJobStatus(api, failedJob.id, 'failed')
      expect(failed.error || '').toBeTruthy()

      const completed = await waitForCompletedExportJob(api, completedJob.id)
      expect(completed.query.meter).toBe(scenario.meterName)
      expect(completed.artifact_size || 0).toBeGreaterThan(0)
    })

    await page.goto('/exports')
    await expect(page.getByRole('heading', { name: 'Export jobs' })).toBeVisible()

    const failedRow = page.locator('.export-job-row', { hasText: failedJob.query.meter }).first()
    await expect(failedRow).toContainText('Failed')
    await expect(failedRow.getByRole('button', { name: 'Retry' })).toBeVisible()

    const completedRow = page.locator('.export-job-row', { hasText: completedJob.query.meter }).first()
    await expect(completedRow).toContainText('Completed')
    await expect(completedRow.getByRole('button', { name: 'Download' })).toBeVisible()
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

  async scopedAPIKeyUsageWriteWasRecorded(page: Page, scenario: ScopedAPIKeyScenario) {
    await page.goto('/usage')
    await expect(page.getByRole('heading', { name: 'Usage buckets' })).toBeVisible()
    await selectMeterFilter(page, scenario.allowedMeter)
    await setUsageDateRange(page, {
      from: toLocalDateTime(new Date(Date.now() - 60 * 60_000)),
      meterName: scenario.allowedMeter,
      primarySubject: scenario.subject,
      timestamp: new Date().toISOString(),
      to: toLocalDateTime(new Date(Date.now() + 60 * 60_000)),
    })
    await page.getByRole('button', { name: 'Run Query' }).click()

    const results = page.locator('.usage-results-card')
    await expect(results).toContainText(scenario.allowedMeter)
    await expect(results).toContainText('1')

    await page.getByRole('button', { name: 'View Events' }).click()
    const events = page.locator('.usage-events-card')
    await expect(events).toContainText(scenario.allowedMeter)
    await expect(events).toContainText(scenario.subject)
    await expect(events).toContainText('1')
  },

  async workspaceOwnedDataIsHiddenFromCurrentUser(page: Page, scenario: WorkspaceIsolationScenario) {
    await page.goto('/meters')
    await expect(page.getByRole('heading', { name: 'Meter definitions' })).toBeVisible()
    await expect(page.locator('main')).not.toContainText(scenario.meterName)

    await page.goto('/usage')
    await expect(page.getByRole('heading', { name: 'Usage buckets' })).toBeVisible()
    await expect(page.locator('main')).not.toContainText(scenario.meterName)
    await expect(page.locator('main')).not.toContainText(scenario.subject)

    await page.goto('/subjects')
    await expect(page.getByRole('heading', { name: 'Subjects' })).toBeVisible()
    await expect(page.locator('main')).not.toContainText(scenario.subject)

    await page.goto('/alerts')
    await expect(page.getByRole('heading', { name: 'Alerts' })).toBeVisible()
    await expect(page.locator('main')).not.toContainText(scenario.alertName)
    await expect(page.locator('main')).not.toContainText(scenario.destinationName)

    await page.goto(`/alerts/${scenario.alertID}`)
    await expect(page.getByRole('heading', { name: 'Alert not found' })).toBeVisible()
    await expect(page.locator('main')).not.toContainText(scenario.alertName)

    await page.goto('/exports')
    await expect(page.getByRole('heading', { name: 'Export jobs' })).toBeVisible()
    await expect(page.locator('main')).not.toContainText(scenario.meterName)

    await page.goto('/api-keys')
    await expect(page.getByRole('heading', { name: 'SDK access' })).toBeVisible()
    await expect(page.locator('main')).not.toContainText(scenario.apiKeyName)

    await page.goto('/overview')
    await expect(page.getByRole('heading', { name: 'Metering operations' })).toBeVisible()
    await expect(page.locator('main')).not.toContainText(scenario.subject)
  },

  async malformedSubjectRouteShowsNotFound(page: Page) {
    await page.goto(`/subjects/${encodeURIComponent('gggg-<>ddddd')}`)
    await expect(page.getByRole('heading', { name: 'Subject not found' })).toBeVisible()
    await expect(page.locator('main')).not.toContainText('Meter Activity')
    await expect(page.locator('main')).not.toContainText('Recent Events')
  },

  async missingMeterRouteShowsNotFound(page: Page) {
    await page.goto('/meters/missing_e2e_meter')
    await expect(page.getByRole('heading', { name: 'Meter not found' })).toBeVisible()
    await expect(page.locator('main')).not.toContainText('No route found')
  },

  async missingPlanRouteShowsNotFound(page: Page) {
    await page.goto('/plans/missing_e2e_plan')
    await expect(page.getByRole('heading', { name: 'Plan not found' })).toBeVisible()
    await expect(page.locator('main')).not.toContainText('Assignments')
    await expect(page.locator('main')).not.toContainText('No route found')
  },

  async missingAlertRouteShowsNotFound(page: Page) {
    await page.goto('/alerts/missing_e2e_alert')
    await expect(page.getByRole('heading', { name: 'Alert not found' })).toBeVisible()
    await expect(page.locator('main')).not.toContainText('Recent Events')
    await expect(page.locator('main')).not.toContainText('No route found')
  },

  async workspaceOwnedAPIResourcesAreHiddenFromCurrentUser(page: Page, scenario: WorkspaceIsolationScenario) {
    const [meters, apiKeys, alerts, destinations, subjects, events, exports] = await Promise.all([
      page.request.get('/v1/meters'),
      page.request.get('/v1/auth/api-keys'),
      page.request.get('/v1/alerts'),
      page.request.get('/v1/alerts/destinations'),
      page.request.get('/v1/subjects?limit=50'),
      page.request.get('/v1/usageevents?limit=50'),
      page.request.get('/v1/exports?limit=50'),
    ])

    for (const response of [meters, apiKeys, alerts, destinations, subjects, events, exports]) {
      expect(response.status()).toBe(200)
    }

    await expectListExcludes(meters, 'name', scenario.meterName)
    await expectListExcludes(apiKeys, 'name', scenario.apiKeyName)
    await expectListExcludes(alerts, 'name', scenario.alertName)
    await expectListExcludes(destinations, 'name', scenario.destinationName)
    await expectListExcludes(subjects, 'subject', scenario.subject)
    await expectListExcludes(events, 'subject', scenario.subject)
    await expectListExcludes(exports, 'id', scenario.exportID)

    const usageQuery = new URLSearchParams({
      bucket_size: 'day',
      from: new Date(Date.now() - 60 * 60_000).toISOString(),
      limit: '100',
      meter: scenario.meterName,
      to: new Date(Date.now() + 60 * 60_000).toISOString(),
    })
    const buckets = await page.request.get(`/v1/usages?${usageQuery.toString()}`)
    if (buckets.status() === 200) {
      expect(await buckets.json()).toEqual([])
    } else {
      expect(buckets.status()).toBe(404)
    }

    const alert = await page.request.get(`/v1/alerts/${scenario.alertID}`)
    expect(alert.status()).toBe(404)
    const exportJob = await page.request.get(`/v1/exports/${scenario.exportID}`)
    expect(exportJob.status()).toBe(404)
    const deleteKey = await page.request.delete(`/v1/auth/api-keys/${scenario.apiKeyID}`)
    expect(deleteKey.status()).toBe(404)
  },

  async workspaceOwnedAlertDetailIsVisibleToCurrentUser(page: Page, scenario: WorkspaceIsolationScenario) {
    await page.goto(`/alerts/${scenario.alertID}`)
    await expect(page.getByRole('heading', { name: scenario.alertName })).toBeVisible()
    await expect(page.locator('main')).toContainText(scenario.destinationName)
    await expect(page.locator('main')).toContainText(scenario.meterName)
  },

  async planEntitlementStateIsVisible(page: Page, scenario: PlanEntitlementScenario) {
    const section = page.locator('section,div').filter({ has: page.getByRole('heading', { name: 'Current Entitlements' }) }).first()
    await expect(section).toContainText(scenario.subject, { timeout: 30_000 })
    await expect(section).toContainText(scenario.meterName, { timeout: 30_000 })
    await expect(section).toContainText(scenario.planName, { timeout: 30_000 })
    await expect(section).toContainText('Warning', { timeout: 30_000 })
    await expect(section).toContainText(`${scenario.current} / ${scenario.limit}`, { timeout: 30_000 })
  },

  async planEntitlementChangeIsVisible(page: Page, scenario: PlanEntitlementScenario) {
    const section = page.locator('section,div').filter({ has: page.getByRole('heading', { name: 'Recent Entitlement Changes' }) }).first()
    await expect(section).toContainText(scenario.subject, { timeout: 30_000 })
    await expect(section).toContainText(scenario.meterName, { timeout: 30_000 })
    await expect(section).toContainText('Warning', { timeout: 30_000 })
    await expect(section).toContainText('quota warning threshold reached', { timeout: 30_000 })
  },

  async subjectEntitlementStateIsVisible(page: Page, scenario: PlanEntitlementScenario) {
    const section = page.locator('section').filter({ has: page.getByRole('heading', { name: 'Entitlements' }) }).first()
    await expect(section).toContainText(scenario.meterName, { timeout: 30_000 })
    await expect(section).toContainText(scenario.planName, { timeout: 30_000 })
    await expect(section).toContainText('Warning', { timeout: 30_000 })
    await expect(section).toContainText(`${scenario.current} / ${scenario.limit}`, { timeout: 30_000 })
  },

  async subjectEntitlementChangeIsVisible(page: Page, scenario: PlanEntitlementScenario) {
    const section = page.locator('section,div').filter({ has: page.getByRole('heading', { name: 'Entitlement Changes' }) }).first()
    await expect(section).toContainText(scenario.meterName, { timeout: 30_000 })
    await expect(section).toContainText(scenario.planName, { timeout: 30_000 })
    await expect(section).toContainText('Warning', { timeout: 30_000 })
    await expect(section).toContainText('quota warning threshold reached', { timeout: 30_000 })
  },
}

async function fillDimension(
  page: Page,
  index: number,
  dimension: { description: string; displayName: string; name: string },
) {
  const row = page.getByRole('dialog', { name: 'Create Meter' }).locator('.schema-row').nth(index)
  await row.getByLabel('Dimension name').fill(dimension.name)
  await row.getByLabel('Dimension display name').fill(dimension.displayName)
  await row.getByLabel('Dimension description').fill(dimension.description)
  const dimensionType = row.getByLabel('Dimension type')
  const tagName = await dimensionType.evaluate((element) => element.tagName.toLowerCase())
  if (tagName === 'select') {
    await dimensionType.selectOption('string')
  }
}

async function selectMeterFilter(page: Page, meterName: string) {
  const firstRule = page.locator('.filter-builder .rule').first()
  await selectControlOption(page, firstRule.locator('.rule-fields'), 'meter')
  await selectControlOption(page, firstRule.locator('.rule-operators'), '=')
  await selectControlOption(page, firstRule.locator('.rule-value'), meterName)
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
  await selectControlOption(page, row.locator('.rule-fields'), rule.field)
  await selectControlOption(page, row.locator('.rule-operators'), rule.operator)
  await setRuleValue(page, row, rule.value)
}

async function setRuleValue(page: Page, row: ReturnType<Page['locator']>, value: string) {
  const valueControl = row.locator('.rule-value')
  const tagName = await valueControl.evaluate((element) => element.tagName.toLowerCase())
  if (tagName === 'input') {
    await valueControl.fill(value)
    return
  }
  await selectControlOption(page, valueControl, value)
}

async function setScope(page: Page, scope: string, enabled: boolean) {
  const checkbox = page.locator(`input[name="scopes"][value="${scope}"]`)
  if (await checkbox.isChecked() === enabled) {
    return
  }
  await checkbox.locator('xpath=ancestor::label[1]').click()
  if (enabled) {
    await expect(checkbox).toBeChecked()
    return
  }
  await expect(checkbox).not.toBeChecked()
}

async function selectControlOption(
  page: Page,
  control: ReturnType<Page['locator']>,
  value: string,
) {
  const tagName = await control.evaluate((element) => element.tagName.toLowerCase())
  if (tagName === 'select') {
    await control.selectOption(value)
    return
  }

  await control.click()
  await page.getByRole('option', { name: optionNamePattern(selectOptionLabel(value)) }).click()
}

function selectOptionLabel(value: string) {
  const labels: Record<string, string> = {
    '!=': 'not equals',
    '<': 'less than',
    '<=': 'less or equal',
    '=': 'equals',
    '>': 'greater than',
    '>=': 'greater or equal',
    'metadata.region-name': 'Region',
    'metadata.service.tier': 'Service Tier',
    area: 'Filled Area',
    bar: 'Bar',
    day: 'Day',
    hour: 'Hour',
    line: 'Line',
    meter: 'Meter',
    month: 'Month',
    quantity: 'Quantity',
    subject: 'Subject',
  }
  return labels[value] || value
}

function optionNamePattern(label: string) {
  return new RegExp(`^${escapeRegExp(label)}(?: \\(\\d+\\))?$`)
}

function escapeRegExp(value: string) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
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

async function waitForExportJobStatus(api: APIRequestContext, id: string, status: string): Promise<ExportJobResponse> {
  let latest: ExportJobResponse | null = null

  await expect.poll(async () => {
    const response = await api.get(`/v1/exports/${id}`)
    expect(response.status()).toBe(200)
    latest = await response.json() as ExportJobResponse
    return latest.status
  }, {
    intervals: [250, 500, 1000],
    timeout: 20_000,
  }).toBe(status)

  return latest as ExportJobResponse
}

async function expectSavedUsageQuery(page: Page, name: string) {
  await expect.poll(async () => {
    const response = await page.request.get('/v1/usage/saved-queries')
    expect(response.status()).toBe(200)
    const payload = await response.json() as { items?: Array<{ name: string; pinned: boolean }> }
    return Boolean((payload.items || []).find((item) => item.name === name && item.pinned))
  }, {
    intervals: [250, 500, 1000],
    timeout: 10_000,
  }).toBe(true)
}

async function expectListExcludes(response: APIResponse, key: string, value: string) {
  const payload = await response.json() as { items?: Array<Record<string, unknown>> }
  expect((payload.items || []).some((item) => item[key] === value)).toBe(false)
}

async function waitForWorkspaceResources(page: Page, scenario: WorkspaceIsolationScenario) {
  await Promise.all([
    waitForListIncludes(page, '/v1/meters', 'name', scenario.meterName),
    waitForListIncludes(page, '/v1/auth/api-keys', 'name', scenario.apiKeyName),
    waitForListIncludes(page, '/v1/alerts', 'name', scenario.alertName),
    waitForListIncludes(page, '/v1/alerts/destinations', 'name', scenario.destinationName),
    waitForListIncludes(page, '/v1/subjects?limit=50', 'subject', scenario.subject),
    waitForListIncludes(page, '/v1/usageevents?limit=50', 'subject', scenario.subject),
    waitForListIncludes(page, '/v1/exports?limit=50', 'id', scenario.exportID),
  ])
}

async function waitForListIncludes(page: Page, path: string, key: string, value: string) {
  await expect.poll(async () => {
    const response = await page.request.get(path)
    expect(response.status()).toBe(200)
    const payload = await response.json() as { items?: Array<Record<string, unknown>> }
    return (payload.items || []).some((item) => item[key] === value)
  }, {
    intervals: [250, 500, 1000, 2000],
    timeout: 45_000,
  }).toBe(true)
}

async function waitForEntitlementState(page: Page, path: string, state: string) {
  await expect.poll(async () => {
    const response = await page.request.get(path)
    expect(response.status()).toBe(200)
    const payload = await response.json() as { items?: Array<{ state: string }> }
    return payload.items?.[0]?.state || ''
  }, {
    intervals: [250, 500, 1000, 2000],
    timeout: 45_000,
  }).toBe(state)
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
