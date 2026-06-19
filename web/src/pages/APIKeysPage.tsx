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

export function APIKeysPage() {
  const { creating, createdKey, deleting, error, items, saving } = useSelector(appStore, (state) => state.apiKeys)
  const load = useCallback(() => appStoreActions.loadAPIKeys(), [])

  useInitialLoad(load)

  async function submitCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const formElement = event.currentTarget
    const form = new FormData(formElement)

    try {
      await appStoreActions.createAPIKey({ name: String(form.get('name') || '') })
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
            <Badge variant={items.length > 0 ? 'success' : 'muted'}>{items.length} rows</Badge>
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
              <strong className="api-key-name">{key.name}</strong>,
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
        <Modal title="Create API Key" onClose={() => appStoreActions.setAPIKeyCreating(false)}>
          <form className="modal-form" onSubmit={(event) => void submitCreate(event)}>
            <label>
              Name
              <input name="name" placeholder="server-billing-sync" required />
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
