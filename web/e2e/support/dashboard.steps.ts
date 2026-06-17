import { expect, type Page } from '@playwright/test'

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

    await page.locator('.meter-create-form').getByRole('button', { name: 'Create' }).click()
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

  async usageQueryIsScopedToSubjectAndMeter(page: Page, scenario: UsageScenario) {
    await expect(page).toHaveURL(/\/usage$/)
    await expect.poll(() => queryRulePairs(page)).toEqual(expect.arrayContaining([
      `meter:${scenario.meterName}`,
      `subject:${scenario.primarySubject}`,
    ]))
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

function toLocalDateTime(date: Date) {
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60_000)
  return local.toISOString().slice(0, 16)
}

function uniqueID() {
  return `${Date.now()}-${Math.random().toString(16).slice(2)}`
    .replace(/[^a-z0-9]/gi, '')
    .slice(0, 24)
}
