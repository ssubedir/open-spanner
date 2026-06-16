import { test } from '@playwright/test'

import { Given, Then, When } from '../support/dashboard.steps'

test.describe('Feature: Dashboard usage exploration', () => {
  test('Scenario: a user queries usage by nested and hyphenated meter dimensions', async ({ page }) => {
    const account = await Given.aDashboardAccount(page)
    const meterName = `api_requests_${Date.now()}`

    await When.theUserSignsIn(page, account)
    await Then.theDashboardIsAvailable(page, account)

    await When.theUserCreatesAnAPIRequestMeter(page, meterName)
    await Then.theMeterIsVisible(page, meterName)

    const scenario = await Given.apiRequestUsageExists(page, meterName)

    await When.theUserQueriesUsageByServiceTier(page, scenario)
    await Then.theUsagePageLoadsWithoutDimensionErrors(page)
    await Then.bucketedUsageIncludesGoldServiceTier(page, meterName)
  })
})
