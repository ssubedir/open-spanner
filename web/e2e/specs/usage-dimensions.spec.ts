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

  test('Scenario: a user saves and runs an advanced usage query', async ({ page }) => {
    const account = await Given.aDashboardAccount(page)
    const meterName = `api_requests_advanced_${Date.now()}`

    await When.theUserSignsIn(page, account)
    await Then.theDashboardIsAvailable(page, account)

    await When.theUserCreatesAnAPIRequestMeter(page, meterName)
    const scenario = await Given.apiRequestUsageExists(page, meterName)

    await When.theUserRunsAnAdvancedUsageQuery(page, scenario)
    await Then.theUsagePageLoadsWithoutDimensionErrors(page)
    await Then.advancedQueryReturnsOnlyMatchingUsage(page, meterName)

    await When.theUserViewsCurrentUsageEvents(page)
    await Then.rawUsageEventsIncludeOnlyMatchingUsage(page, scenario)
    await When.theUserOpensFirstUsageEvent(page)
    await Then.usageEventDetailsIncludeMatchingUsage(page, scenario)
    await When.theUserClosesUsageEventDetails(page)

    const bucketExport = await When.theUserExportsCurrentUsageBuckets(page)
    await Then.advancedUsageBucketCSVIncludesMatchingUsage(bucketExport, scenario)
  })

  test('Scenario: a user opens usage from subject activity', async ({ page }) => {
    const account = await Given.aDashboardAccount(page)
    const meterName = `api_requests_subject_${Date.now()}`

    await When.theUserSignsIn(page, account)
    await Then.theDashboardIsAvailable(page, account)

    await When.theUserCreatesAnAPIRequestMeter(page, meterName)
    const scenario = await Given.apiRequestUsageExists(page, meterName)

    await When.theUserOpensUsageFromSubjectActivity(page, scenario)
    await Then.usageQueryIsScopedToSubjectAndMeter(page, scenario)
  })

  test('Scenario: a user exports usage CSV files', async ({ page }) => {
    const account = await Given.aDashboardAccount(page)
    const meterName = `api_requests_export_${Date.now()}`

    await When.theUserSignsIn(page, account)
    await Then.theDashboardIsAvailable(page, account)

    await When.theUserCreatesAnAPIRequestMeter(page, meterName)
    const scenario = await Given.apiRequestUsageExists(page, meterName)

    await When.theUserQueriesUsageByServiceTier(page, scenario)
    const bucketExport = await When.theUserExportsCurrentUsageBuckets(page)
    await Then.usageBucketCSVIncludesCurrentQuery(bucketExport, scenario)

    const currentEventExport = await When.theUserExportsCurrentUsageEvents(page)
    await Then.currentUsageEventCSVIncludesCurrentQuery(currentEventExport, scenario)

    await When.theUserQueuesCurrentUsageExport(page)
    const queuedExport = await Then.queuedUsageExportCompletesInDashboard(page, scenario)
    await Then.queuedUsageBucketCSVIncludesCurrentQuery(queuedExport, scenario)
    await Then.theExportsPageShowsCompletedJob(page, scenario)

    const eventExport = await When.theUserExportsSubjectEvents(page, scenario)
    await Then.subjectEventCSVIncludesPrimaryUsage(eventExport, scenario)

    const apiKey = await Given.anAPIKeyExists(page)
    const apiBucketExport = await When.theServiceExportsFilteredUsageBucketsWithAPIKey(page, apiKey, scenario)
    await Then.directUsageBucketCSVResponseIncludesCurrentQuery(apiBucketExport, scenario)

    const apiEventExport = await When.theServiceExportsSubjectEventsWithAPIKey(page, apiKey, scenario)
    await Then.directUsageEventCSVResponseIncludesPrimaryUsage(apiEventExport, scenario)

    const exportJob = await When.theServiceQueuesUsageExportJob(page, apiKey, scenario)
    await Then.queuedExportJobCompletesAndDownloads(page, apiKey, exportJob, scenario)
  })
})
