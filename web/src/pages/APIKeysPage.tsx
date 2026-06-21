import { useSelector } from '@tanstack/react-store'
import { Copy, KeyRound, Loader2, Plus, Trash2 } from 'lucide-react'
import { type FormEvent, useCallback } from 'react'

import { appStore, appStoreActions } from '../app-store'
import { DataTable, Modal, PageHeader } from '../components/dashboard'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { formatDate } from '../lib/format'
import { useInitialLoad } from '../lib/hooks'

const apiKeyScopes = [
  { value: 'usage:write', label: 'Write usage', group: 'Usage', description: 'Record usage events from a backend service.' },
  { value: 'usage:read', label: 'Read usage', group: 'Usage', description: 'Query buckets, raw events, dimensions, and breakdowns.' },
  { value: 'meters:read', label: 'Read meters', group: 'Meters', description: 'Read meter definitions and schemas.' },
  { value: 'meters:write', label: 'Write meters', group: 'Meters', description: 'Create and edit meter definitions.' },
  { value: 'alerts:read', label: 'Read alerts', group: 'Alerts', description: 'List alert rules, destinations, and events.' },
  { value: 'alerts:write', label: 'Write alerts', group: 'Alerts', description: 'Manage alert rules and destinations.' },
  { value: 'exports:read', label: 'Read exports', group: 'Exports', description: 'List and download usage exports.' },
  { value: 'exports:write', label: 'Write exports', group: 'Exports', description: 'Queue, cancel, and retry export jobs.' },
  { value: 'system:read', label: 'Read system', group: 'System', description: 'Read operational stats for the workspace.' },
]

const defaultAPIKeyScopes = new Set(['usage:write', 'usage:read', 'meters:read', 'meters:write'])
const apiKeyScopeGroups = Array.from(new Set(apiKeyScopes.map((scope) => scope.group)))

const apiKeyExpirationPresets = [
  { value: '', label: 'Never expires' },
  { value: '1d', label: '1 day' },
  { value: '7d', label: '1 week' },
  { value: '30d', label: '1 month' },
  { value: '90d', label: '3 months' },
  { value: '180d', label: '6 months' },
  { value: '365d', label: '1 year' },
]

export function APIKeysPage() {
  const { creating, createdKey, deleting, error, items, saving } = useSelector(appStore, (state) => state.apiKeys)
  const load = useCallback(() => appStoreActions.loadAPIKeys(), [])

  useInitialLoad(load)

  async function submitCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const formElement = event.currentTarget
    const form = new FormData(formElement)

    try {
      const expiresAfter = String(form.get('expires_after') || '').trim()
      await appStoreActions.createAPIKey({
        allowed_meters: splitList(String(form.get('allowed_meters') || '')),
        expires_at: expirationPresetToISO(expiresAfter),
        name: String(form.get('name') || ''),
        scopes: form.getAll('scopes').map(String),
      })
      formElement.reset()
      appStoreActions.setAPIKeyCreating(false)
    } catch {
      // Store owns the visible API key error state.
    }
  }

  async function confirmDelete() {
    try {
      await appStoreActions.deleteSelectedAPIKey()
    } catch {
      // Store owns the visible API key error state.
    }
  }

  async function copyCreatedKey() {
    if (!createdKey) {
      return
    }
    await copyText(createdKey.key)
  }

  return (
    <>
      <PageHeader
        eyebrow="API Keys"
        icon={<KeyRound />}
        title="SDK access"
        description="Issue API keys for trusted backend clients and revoke stale credentials."
        action={null}
      />

      {error ? <div className="error-banner">{error}</div> : null}

      {createdKey ? (
        <section className="secret-panel" aria-label="Created API key">
          <div>
            <span>New key ready</span>
            <strong>{createdKey.name}</strong>
            <small>Copy this secret now. It will not be shown again.</small>
          </div>
          <code title={createdKey.key}>{createdKey.key}</code>
          <div className="secret-actions">
            <Button onClick={() => void copyCreatedKey()} type="button">
              <Copy aria-hidden="true" />
              Copy key
            </Button>
            <Button onClick={appStoreActions.clearCreatedAPIKey} type="button" variant="outline">Dismiss</Button>
          </div>
        </section>
      ) : null}

      <Card className="api-key-table-card">
        <CardHeader className="api-key-card-header">
          <div>
            <CardTitle>Keys</CardTitle>
            <CardDescription>Active keys for SDK clients.</CardDescription>
          </div>
          <div className="card-header-actions">
            <Button disabled={saving} onClick={() => appStoreActions.setAPIKeyCreating(true)} type="button">
              <Plus aria-hidden="true" />
              New key
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          <DataTable
            emptyLabel="No API keys yet"
            headers={['Name', 'Prefix', 'Created', 'Last Used', 'Actions']}
            rows={items.map((key) => [
              <span className="api-key-name-block">
                <strong className="api-key-name">{key.name}</strong>
                <ScopeBadges scopes={key.scopes} />
                {key.allowed_meters.length > 0 ? <small>meters: {key.allowed_meters.join(', ')}</small> : null}
              </span>,
              <Badge className="api-key-prefix" variant="muted">
                <span className="mono">{key.prefix}</span>
              </Badge>,
              formatDate(key.created_at),
              key.last_used_at ? formatDate(key.last_used_at) : <span className="muted">Never</span>,
              <span className="table-actions">
                <Button aria-label={`Delete ${key.name}`} disabled={saving} onClick={() => appStoreActions.setAPIKeyDeleting(key)} size="icon" type="button" variant="ghost">
                  <Trash2 aria-hidden="true" />
                </Button>
              </span>,
            ])}
          />
        </CardContent>
      </Card>

      {creating ? (
        <Modal className="api-key-modal" title="Create API Key" onClose={() => appStoreActions.setAPIKeyCreating(false)}>
          <form className="modal-form api-key-modal-form" onSubmit={(event) => void submitCreate(event)}>
            <label>
              Name
              <input name="name" placeholder="server-billing-sync" required />
            </label>
            <div className="form-field wide">
              <span className="field-label">Scopes</span>
              <div className="scope-picker">
                {apiKeyScopeGroups.map((group) => (
                  <section className="scope-group" key={group} aria-label={`${group} scopes`}>
                    <strong>{group}</strong>
                    <div className="scope-options">
                      {apiKeyScopes.filter((scope) => scope.group === group).map((scope) => (
                        <label className="scope-option" key={scope.value}>
                          <input defaultChecked={defaultAPIKeyScopes.has(scope.value)} name="scopes" type="checkbox" value={scope.value} />
                          <span>
                            <b>{scope.label}</b>
                            <small>{scope.description}</small>
                          </span>
                        </label>
                      ))}
                    </div>
                  </section>
                ))}
              </div>
            </div>
            <label>
              Allowed meters
              <textarea name="allowed_meters" placeholder="Leave blank for all meters&#10;api_requests&#10;storage_bytes" rows={3} />
            </label>
            <label>
              Expires after
              <select defaultValue="" name="expires_after">
                {apiKeyExpirationPresets.map((preset) => (
                  <option key={preset.value || 'never'} value={preset.value}>{preset.label}</option>
                ))}
              </select>
            </label>
            <div className="modal-actions">
              <Button onClick={() => appStoreActions.setAPIKeyCreating(false)} type="button" variant="outline">Cancel</Button>
              <Button disabled={saving} type="submit">
                {saving ? <Loader2 className="spin" aria-hidden="true" /> : <Plus aria-hidden="true" />}
                Create key
              </Button>
            </div>
          </form>
        </Modal>
      ) : null}

      {deleting ? (
        <Modal title="Delete API Key" onClose={() => appStoreActions.setAPIKeyDeleting(null)}>
          <div className="modal-copy">Delete <strong>{deleting.name}</strong>?</div>
          <div className="modal-actions">
            <Button onClick={() => appStoreActions.setAPIKeyDeleting(null)} type="button" variant="outline">Cancel</Button>
            <Button disabled={saving} onClick={() => void confirmDelete()} type="button">Delete</Button>
          </div>
        </Modal>
      ) : null}
    </>
  )
}

function ScopeBadges({ scopes }: { scopes: string[] }) {
  if (scopes.length === 0) {
    return <Badge variant="warning">No scopes</Badge>
  }

  return (
    <span className="scope-badge-list" aria-label={scopes.join(', ')}>
      {scopes.map((scope) => (
        <Badge key={scope} title={scope} variant="muted">
          {scopeLabel(scope)}
        </Badge>
      ))}
    </span>
  )
}

function scopeLabel(scope: string) {
  return apiKeyScopes.find((item) => item.value === scope)?.label || scope
}

async function copyText(value: string) {
  try {
    await navigator.clipboard.writeText(value)
    return
  } catch {
    const textarea = document.createElement('textarea')
    textarea.value = value
    textarea.setAttribute('readonly', 'true')
    textarea.style.left = '-9999px'
    textarea.style.position = 'fixed'
    document.body.appendChild(textarea)
    textarea.select()
    document.execCommand('copy')
    textarea.remove()
  }
}

function splitList(value: string) {
  return value
    .split(/[\n,]/)
    .map((item) => item.trim())
    .filter(Boolean)
}

function expirationPresetToISO(value: string) {
  if (!value) {
    return undefined
  }

  const match = value.match(/^(\d+)d$/)
  if (!match) {
    return undefined
  }

  const expiresAt = new Date()
  expiresAt.setDate(expiresAt.getDate() + Number(match[1]))
  return expiresAt.toISOString()
}
