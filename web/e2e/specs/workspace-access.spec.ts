import { execFile } from 'node:child_process'
import { resolve } from 'node:path'
import { promisify } from 'node:util'

import { expect, request, test, type APIRequestContext, type Page } from '@playwright/test'

import { Given, Then, When } from '../support/dashboard.steps'

test.describe('Feature: Workspace access', () => {
  test('Scenario: an owner invites, promotes, and removes a workspace member', async ({ browser, page }) => {
    const owner = await Given.aDashboardAccount(page)
    const memberID = uniqueID()
    const member = { email: `member-${memberID}@example.com`, password: `open-spanner-${memberID}` }
    const memberContext = await browser.newContext()
    const memberPage = await memberContext.newPage()
    const createMember = await memberPage.request.post('/v1/auth/users', { data: member })
    expect(createMember.status()).toBe(201)

    await When.theUserSignsIn(page, owner)
    await Then.theDashboardIsAvailable(page, owner)
    await page.goto('/settings/workspace')
    await expect(page.getByRole('heading', { name: 'Workspace access' })).toBeVisible()

    await page.getByLabel('Email').fill(member.email)
    await page.getByLabel('Invite role').selectOption('viewer')
    await page.getByRole('button', { name: 'Create invite' }).click()
    const inviteLink = await page.getByLabel('Invitation link').inputValue()
    expect(inviteLink).toContain('/invite?token=')

    await memberPage.goto(inviteLink)
    await expect(memberPage.getByRole('heading', { name: 'Workspace invitation' })).toBeVisible()
    await expect(memberPage.getByText(member.email)).toBeVisible()
    await memberPage.getByRole('link', { name: 'Sign in to accept' }).click()
    await memberPage.getByLabel('Email').fill(member.email)
    await memberPage.getByLabel('Password').fill(member.password)
    await memberPage.getByRole('button', { name: 'Sign in' }).click()
    await memberPage.getByRole('button', { name: 'Accept invitation' }).click()
    await expect(memberPage).toHaveURL(/\/overview$/)

    const viewerWrite = await memberPage.request.post('/v1/meters', { data: { aggregation: 'sum', event_retention_days: 90, name: `viewer_denied_${memberID}`, unit: 'request' } })
    expect(viewerWrite.status()).toBe(403)
    expect((await memberPage.request.get('/v1/meters')).status()).toBe(200)

    await page.reload()
    const memberRow = page.getByLabel(`Role for ${member.email}`).locator('xpath=ancestor::tr')
    await expect(memberRow.getByLabel(`Role for ${member.email}`)).toHaveValue('viewer')
    await memberRow.getByLabel(`Role for ${member.email}`).selectOption('admin')
    await expect.poll(async () => (await memberPage.request.post('/v1/meters', { data: { aggregation: 'sum', event_retention_days: 90, name: `admin_allowed_${memberID}`, unit: 'request' } })).status()).toBe(201)

    page.once('dialog', (dialog) => dialog.accept())
    await memberRow.getByRole('button', { name: 'Remove' }).click()
    await expect(memberRow).toHaveCount(0)
    await expect.poll(async () => (await memberPage.request.get('/v1/meters')).status()).toBe(401)

    await memberContext.close()
  })

  test('Scenario: invitations reject duplicates, wrong accounts, revocation, reuse, and expiry', async ({ browser, page }) => {
    const owner = await createAccount(page, 'invite-owner')
    const inviteeContext = await browser.newContext()
    const inviteePage = await inviteeContext.newPage()
    const invitee = await createAccount(inviteePage, 'invitee')
    const wrongContext = await browser.newContext()
    const wrongPage = await wrongContext.newPage()
    const wrongAccount = await createAccount(wrongPage, 'wrong-invitee')

    try {
      await When.theUserSignsIn(page, owner)
      await When.theUserSignsIn(inviteePage, invitee)
      await When.theUserSignsIn(wrongPage, wrongAccount)
      await Then.theDashboardIsAvailable(page, owner)
      await Then.theDashboardIsAvailable(inviteePage, invitee)
      await Then.theDashboardIsAvailable(wrongPage, wrongAccount)

      const first = await createInvitation(page, invitee.email, 'viewer')
      expect(new Date(first.expires_at).getTime()).toBeGreaterThan(Date.now() + 6 * 24 * 60 * 60 * 1000)
      expect(new Date(first.expires_at).getTime()).toBeLessThan(Date.now() + 8 * 24 * 60 * 60 * 1000)
      expect((await page.request.post('/v1/auth/workspace/invitations', { data: { email: invitee.email, role: 'admin' } })).status()).toBe(409)

      const pendingPreview = await inviteePage.request.get(`/v1/auth/workspace-invitations/${first.token}`)
      expect(pendingPreview.status()).toBe(200)
      expect((await pendingPreview.json()).status).toBe('pending')
      expect((await wrongPage.request.post(`/v1/auth/workspace-invitations/${first.token}/accept`)).status()).toBe(403)

      expect((await page.request.delete(`/v1/auth/workspace/invitations/${first.id}`)).status()).toBe(204)
      const revokedPreview = await inviteePage.request.get(`/v1/auth/workspace-invitations/${first.token}`)
      expect((await revokedPreview.json()).status).toBe('revoked')
      expect((await inviteePage.request.post(`/v1/auth/workspace-invitations/${first.token}/accept`)).status()).toBe(409)

      const accepted = await createInvitation(page, invitee.email, 'viewer')
      expect((await inviteePage.request.post(`/v1/auth/workspace-invitations/${accepted.token}/accept`)).status()).toBe(200)
      expect((await inviteePage.request.post(`/v1/auth/workspace-invitations/${accepted.token}/accept`)).status()).toBe(409)
      const acceptedPreview = await inviteePage.request.get(`/v1/auth/workspace-invitations/${accepted.token}`)
      expect((await acceptedPreview.json()).status).toBe('accepted')

      const expiringAccount = await createAccount(wrongPage, 'expired-invitee')
      const expired = await createInvitation(page, expiringAccount.email, 'viewer')
      await expireInvitation(expired.id)
      const expiredPreview = await wrongPage.request.get(`/v1/auth/workspace-invitations/${expired.token}`)
      expect(expiredPreview.status()).toBe(200)
      expect((await expiredPreview.json()).status).toBe('expired')
      expect((await wrongPage.request.post(`/v1/auth/workspace-invitations/${expired.token}/accept`)).status()).toBe(409)

      const auditResponse = await page.request.get('/v1/auth/workspace/invitations')
      expect(auditResponse.status()).toBe(200)
      const audit = await auditResponse.json() as { items: Array<{ id: string; status: string; token?: string }> }
      expect(audit.items.find((item) => item.id === first.id)?.status).toBe('revoked')
      expect(audit.items.find((item) => item.id === accepted.id)?.status).toBe('accepted')
      expect(audit.items.find((item) => item.id === expired.id)?.status).toBe('expired')
      expect(audit.items.every((item) => !item.token)).toBe(true)
    } finally {
      await inviteeContext.close()
      await wrongContext.close()
    }
  })

  test('Scenario: workspace switching and live roles protect sessions and retained API keys', async ({ browser, page }) => {
    const owner = await createAccount(page, 'workspace-owner')
    const memberContext = await browser.newContext()
    const memberPage = await memberContext.newPage()
    const member = await createAccount(memberPage, 'workspace-member')
    let keyClient: APIRequestContext | null = null
    let removedOwnerKeyClient: APIRequestContext | null = null

    try {
      await When.theUserSignsIn(page, owner)
      await When.theUserSignsIn(memberPage, member)
      await Then.theDashboardIsAvailable(page, owner)
      await Then.theDashboardIsAvailable(memberPage, member)

      const ownerKeyResponse = await page.request.post('/v1/auth/api-keys', {
        data: { name: `removed-owner-${uniqueID()}`, scopes: ['meters:read'] },
      })
      expect(ownerKeyResponse.status()).toBe(201)
      const ownerKey = (await ownerKeyResponse.json() as { key: string }).key
      removedOwnerKeyClient = await request.newContext({ baseURL: new URL(page.url()).origin, extraHTTPHeaders: { Authorization: `Bearer ${ownerKey}` } })

      const invitation = await createInvitation(page, member.email, 'admin')
      const acceptance = await memberPage.request.post(`/v1/auth/workspace-invitations/${invitation.token}/accept`)
      expect(acceptance.status()).toBe(200)
      expect((await acceptance.json()).user.workspace_id).toBe(owner.workspace_id)

      const workspaceResponse = await memberPage.request.get('/v1/auth/workspaces')
      expect(workspaceResponse.status()).toBe(200)
      const workspaces = (await workspaceResponse.json() as { items: Workspace[] }).items
      expect(workspaces).toHaveLength(2)
      const personalWorkspace = workspaces.find((workspace) => workspace.id === member.workspace_id)
      const sharedWorkspace = workspaces.find((workspace) => workspace.id === owner.workspace_id)
      expect(personalWorkspace).toBeTruthy()
      expect(sharedWorkspace?.role).toBe('admin')

      const keyResponse = await memberPage.request.post('/v1/auth/api-keys', {
        data: { name: `retained-${uniqueID()}`, scopes: ['meters:read', 'meters:write'] },
      })
      expect(keyResponse.status()).toBe(201)
      const key = (await keyResponse.json() as { key: string }).key
      keyClient = await request.newContext({ baseURL: new URL(memberPage.url()).origin, extraHTTPHeaders: { Authorization: `Bearer ${key}` } })

      const delegated = await createAccount(memberPage, 'delegated-invitee')
      const delegatedInvite = await createInvitation(memberPage, delegated.email, 'viewer')
      expect((await memberPage.request.delete(`/v1/auth/workspace/invitations/${delegatedInvite.id}`)).status()).toBe(204)

      await memberPage.goto('/overview')
      await switchWorkspaceInDashboard(memberPage, personalWorkspace!)
      const personalMeter = `personal_${uniqueID()}`
      expect((await memberPage.request.post('/v1/meters', { data: meter(personalMeter) })).status()).toBe(201)
      await switchWorkspaceInDashboard(memberPage, sharedWorkspace!)
      const sharedMeters = await memberPage.request.get('/v1/meters')
      expect((await sharedMeters.json() as { items: Array<{ name: string }> }).items.some((item) => item.name === personalMeter)).toBe(false)

      expect((await page.request.patch(`/v1/auth/workspace/members/${owner.id}`, { data: { role: 'admin' } })).status()).toBe(409)
      expect((await page.request.delete(`/v1/auth/workspace/members/${owner.id}`)).status()).toBe(409)

      expect((await page.request.patch(`/v1/auth/workspace/members/${member.id}`, { data: { role: 'viewer' } })).status()).toBe(204)
      expect((await memberPage.request.get('/v1/meters')).status()).toBe(200)
      expect((await memberPage.request.post('/v1/meters', { data: meter(`viewer_session_${uniqueID()}`) })).status()).toBe(403)
      expect((await memberPage.request.post('/v1/auth/workspace/invitations', { data: { email: delegated.email, role: 'viewer' } })).status()).toBe(403)
      expect((await keyClient.get('/v1/meters')).status()).toBe(200)
      expect((await keyClient.post('/v1/meters', { data: meter(`viewer_key_${uniqueID()}`) })).status()).toBe(403)

      expect((await page.request.patch(`/v1/auth/workspace/members/${member.id}`, { data: { role: 'owner' } })).status()).toBe(204)
      expect((await page.request.patch(`/v1/auth/workspace/members/${owner.id}`, { data: { role: 'admin' } })).status()).toBe(204)
      expect((await memberPage.request.patch(`/v1/auth/workspace/members/${member.id}`, { data: { role: 'admin' } })).status()).toBe(409)
      expect((await memberPage.request.delete(`/v1/auth/workspace/members/${member.id}`)).status()).toBe(409)
      expect((await memberPage.request.delete(`/v1/auth/workspace/members/${owner.id}`)).status()).toBe(204)
      expect((await page.request.get('/v1/meters')).status()).toBe(401)
      expect((await removedOwnerKeyClient.get('/v1/meters')).status()).toBe(401)
    } finally {
      await keyClient?.dispose()
      await removedOwnerKeyClient?.dispose()
      await memberContext.close()
    }
  })
})

type Account = { email: string; id: string; password: string; workspace_id: string }
type Invitation = { expires_at: string; id: string; token: string }
type Workspace = { id: string; name: string; role: string }

async function createAccount(page: Page, label: string): Promise<Account> {
  const id = uniqueID()
  const credentials = { email: `${label}-${id}@example.com`, password: `open-spanner-${id}` }
  const response = await page.request.post('/v1/auth/users', { data: credentials })
  expect(response.status()).toBe(201)
  const user = await response.json() as { id: string; workspace_id: string }
  return { ...credentials, ...user }
}

async function createInvitation(page: Page, email: string, role: 'admin' | 'viewer'): Promise<Invitation> {
  const response = await page.request.post('/v1/auth/workspace/invitations', { data: { email, role } })
  expect(response.status()).toBe(201)
  const invitation = await response.json() as Invitation
  expect(invitation.token).toMatch(/^osp_wsi_/)
  return invitation
}

async function switchWorkspaceInDashboard(page: Page, workspace: Workspace) {
  const sidebar = page.locator('aside').filter({ visible: true }).first()
  await sidebar.locator('details > summary').click()
  await sidebar.getByRole('button').filter({ hasText: workspace.name }).click()
  await expect(page).toHaveURL(/\/overview$/)
  await expect(sidebar.locator('details > summary')).toContainText(workspace.name)
}

async function expireInvitation(id: string) {
  const run = promisify(execFile)
  await run('go', ['run', './internal/testsupport/expireinvitation', '--invitation-id', id], {
    cwd: resolve(process.cwd(), '..'),
    env: process.env,
  })
}

function meter(name: string) {
  return { aggregation: 'sum', event_retention_days: 90, name, unit: 'request' }
}

function uniqueID() {
  return `${Date.now()}${Math.random().toString(16).slice(2)}`
}
