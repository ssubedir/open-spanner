import { expect, request, test, type APIRequestContext, type Page } from '@playwright/test'

import { Given, Then, When } from '../support/dashboard.steps'

type APIKey = {
  allowed_meters: string[]
  expires_at?: string | null
  id: string
  name: string
  prefix: string
  revoked_at?: string | null
  scopes: string[]
  status: 'active' | 'revoking' | 'revoked' | 'expired'
}

test.describe('Feature: API key lifecycle', () => {
  test('Scenario: a user expires, rotates, and revokes API keys with an audit trail', async ({ page }) => {
    const account = await Given.aDashboardAccount(page)
    const id = uniqueID()
    const rotatedName = `Lifecycle key ${id}`
    const expiringName = `Expiring key ${id}`

    await When.theUserSignsIn(page, account)
    await Then.theDashboardIsAvailable(page, account)
    await page.goto('/api-keys')

    const originalSecret = await createKeyInDashboard(page, rotatedName, '1 day')
    const original = await findKey(page, rotatedName, (key) => key.status === 'active')
    expect(new Date(original.expires_at || 0).getTime()).toBeGreaterThan(Date.now() + 23 * 60 * 60 * 1000)

    const originalClient = await apiKeyClient(page, originalSecret)
    await expectAuthenticated(originalClient)

    await page.getByRole('button', { name: `Rotate ${rotatedName}` }).click()
    const rotation = page.getByRole('dialog', { name: 'Rotate API Key' })
    await rotation.getByRole('combobox', { name: 'Old key grace period' }).click()
    await page.getByRole('option', { name: '5 minutes' }).click()
    await rotation.getByRole('button', { name: 'Rotate key' }).click()

    const replacementSecret = await readCreatedSecret(page)
    const replacementClient = await apiKeyClient(page, replacementSecret)
    await expectAuthenticated(originalClient)
    await expectAuthenticated(replacementClient)
    await page.getByRole('button', { name: 'Dismiss' }).click()

    const originalDuringGrace = await findKey(page, rotatedName, (key) => key.id === original.id && key.status === 'revoking')
    const replacement = await findKey(page, rotatedName, (key) => key.id !== original.id && key.status === 'active')
    expect(replacement.scopes).toEqual(original.scopes)
    expect(replacement.allowed_meters).toEqual(original.allowed_meters)
    expect(replacement.expires_at).toBe(original.expires_at)
    expect(new Date(originalDuringGrace.revoked_at || 0).getTime()).toBeGreaterThan(Date.now())

    const revokeButton = page.getByRole('button', { name: `Revoke ${rotatedName}` })
    const originalRow = page.getByRole('row').filter({ hasText: original.prefix }).filter({ has: revokeButton })
    await expect(originalRow).toContainText('Revokes')
    await originalRow.getByRole('button', { name: `Revoke ${rotatedName}` }).click()
    await page.getByRole('dialog', { name: 'Revoke API Key' }).getByRole('button', { name: 'Revoke', exact: true }).click()
    await expectRejected(originalClient)
    await expectAuthenticated(replacementClient)

    const expiringResponse = await page.request.post('/v1/auth/api-keys', {
      data: {
        expires_at: new Date(Date.now() + 5_000).toISOString(),
        name: expiringName,
        scopes: ['meters:read'],
      },
    })
    expect(expiringResponse.status()).toBe(201)
    const expiring = await expiringResponse.json() as { key: string }
    const expiringClient = await apiKeyClient(page, expiring.key)
    await expectAuthenticated(expiringClient)
    await expect.poll(async () => (await expiringClient.get('/v1/meters')).status(), { timeout: 10_000 }).toBe(401)

    await page.reload()
    const expiredRow = page.getByRole('row').filter({ hasText: expiringName }).filter({ hasText: 'Expired' })
    await expect(expiredRow).toBeVisible()

    const replacementRow = page.getByRole('row').filter({ hasText: replacement.prefix }).filter({ has: revokeButton })
    await replacementRow.getByRole('button', { name: `Revoke ${rotatedName}` }).click()
    const revocation = page.getByRole('dialog', { name: 'Revoke API Key' })
    await revocation.getByRole('button', { name: 'Revoke', exact: true }).click()
    await expectRejected(replacementClient)

    const lifecycleRows = page.getByRole('row').filter({ hasText: rotatedName })
    await expect(lifecycleRows.getByText('rotated', { exact: true })).toHaveCount(1)
    await expect(lifecycleRows.getByText('revoked', { exact: true })).toHaveCount(2)

    await originalClient.dispose()
    await replacementClient.dispose()
    await expiringClient.dispose()
  })
})

async function createKeyInDashboard(page: Page, name: string, expiry: string) {
  await page.getByRole('button', { name: 'New key' }).click()
  const dialog = page.getByRole('dialog', { name: 'Create API Key' })
  await dialog.getByLabel('Name').fill(name)
  await dialog.getByRole('combobox', { name: 'Expires after' }).click()
  await page.getByRole('option', { name: expiry, exact: true }).click()
  await dialog.getByRole('button', { name: 'Create key' }).click()
  return readCreatedSecret(page)
}

async function readCreatedSecret(page: Page) {
  const panel = page.getByRole('region', { name: 'Created API key' })
  await expect(panel).toBeVisible()
  const secret = (await panel.locator('code').textContent())?.trim() || ''
  expect(secret).toMatch(/^osp_/)
  return secret
}

async function findKey(page: Page, name: string, predicate: (key: APIKey) => boolean) {
  const response = await page.request.get('/v1/auth/api-keys')
  expect(response.status()).toBe(200)
  const payload = await response.json() as { items: APIKey[] }
  const key = payload.items.find((candidate) => candidate.name === name && predicate(candidate))
  expect(key).toBeTruthy()
  return key as APIKey
}

async function apiKeyClient(page: Page, key: string): Promise<APIRequestContext> {
  return request.newContext({
    baseURL: new URL(page.url()).origin,
    extraHTTPHeaders: { Authorization: `Bearer ${key}` },
  })
}

async function expectAuthenticated(client: APIRequestContext) {
  expect((await client.get('/v1/meters')).status()).toBe(200)
}

async function expectRejected(client: APIRequestContext) {
  expect((await client.get('/v1/meters')).status()).toBe(401)
}

function uniqueID() {
  return `${Date.now()}${Math.random().toString(16).slice(2, 10)}`
}
