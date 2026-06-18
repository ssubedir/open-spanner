import { useSelector } from '@tanstack/react-store'
import { BellRing, Loader2, Pencil, Play, Plus, Trash2 } from 'lucide-react'
import { type FormEvent, useCallback } from 'react'

import { appStore, appStoreActions } from '../app-store'
import { DataTable, Modal, PageHeader } from '../components/dashboard'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { formatDate, formatNumber } from '../lib/format'
import { useInitialLoad } from '../lib/hooks'
import type { AlertRule, AlertRuleRequest, AlertRuleUpdateRequest } from '../api'

const comparators = [
  ['gte', '>='],
  ['gt', '>'],
  ['lte', '<='],
  ['lt', '<'],
  ['eq', '='],
  ['neq', '!='],
] as const

export function AlertsPage() {
  const { deleting, editing, error, events, items, meters, saving } = useSelector(appStore, (state) => state.alerts)
  const load = useCallback(() => appStoreActions.loadAlerts(), [])

  useInitialLoad(load)

  async function submitCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    try {
      await appStoreActions.createAlert(alertRequestFromForm(new FormData(event.currentTarget)))
      event.currentTarget.reset()
    } catch {
      // Store owns the visible alerts error state.
    }
  }

  async function submitUpdate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    try {
      await appStoreActions.updateEditingAlert(alertUpdateFromForm(new FormData(event.currentTarget)))
    } catch {
      // Store owns the visible alerts error state.
    }
  }

  async function confirmDelete() {
    try {
      await appStoreActions.deleteSelectedAlert()
    } catch {
      // Store owns the visible alerts error state.
    }
  }

  return (
    <>
      <PageHeader
        eyebrow="Alerts"
        icon={<BellRing />}
        title="Threshold rules"
        description="Track usage windows and surface threshold crossings for important meters."
        action={null}
      />

      {error ? <div className="error-banner">{error}</div> : null}

      <section className="api-key-grid">
        <Card className="api-key-create-card">
          <CardHeader className="api-key-card-header">
            <div>
              <CardTitle>Create Rule</CardTitle>
              <CardDescription>Evaluate a meter over a rolling window and record state changes.</CardDescription>
            </div>
          </CardHeader>
          <CardContent className="form-card">
            <form className="form-grid alert-rule-create-form" onSubmit={(event) => void submitCreate(event)}>
              <label className="wide">
                Name
                <input name="name" placeholder="High API traffic" required />
              </label>
              <label>
                Meter
                <select name="meter" required>
                  <option value="">Select meter</option>
                  {meters.map((meter) => <option key={meter.id} value={meter.name}>{meter.name}</option>)}
                </select>
              </label>
              <label>
                Threshold
                <input name="threshold" placeholder="1000" required step="any" type="number" />
              </label>
              <label>
                Comparator
                <select name="comparator" defaultValue="gte">
                  {comparators.map(([value, label]) => <option key={value} value={value}>{label}</option>)}
                </select>
              </label>
              <label>
                Window
                <select name="window_seconds" defaultValue="3600">
                  <option value="300">5 minutes</option>
                  <option value="900">15 minutes</option>
                  <option value="3600">1 hour</option>
                  <option value="86400">1 day</option>
                </select>
              </label>
              <label>
                Evaluate Every
                <select name="evaluation_interval_seconds" defaultValue="60">
                  <option value="30">30 seconds</option>
                  <option value="60">1 minute</option>
                  <option value="300">5 minutes</option>
                  <option value="900">15 minutes</option>
                </select>
              </label>
              <label>
                Trigger
                <select name="trigger_type" defaultValue="webhook">
                  <option value="webhook">Webhook</option>
                </select>
              </label>
              <label>
                Webhook URL
                <input name="webhook_url" placeholder="https://example.com/open-spanner/alerts" type="url" />
              </label>
              <label>
                Subject
                <input name="subject" placeholder="Optional subject" />
              </label>
              <label>
                Metadata Filters
                <textarea name="metadata" placeholder={'region=us-east\nplan=enterprise'} rows={3} />
              </label>
              <label className="checkbox-row wide">
                <input defaultChecked name="enabled" type="checkbox" />
                Enabled
              </label>
              <Button disabled={saving} type="submit">
                {saving ? <Loader2 className="spin" aria-hidden="true" /> : <Plus aria-hidden="true" />}
                Create
              </Button>
            </form>
          </CardContent>
        </Card>

        <Card className="api-key-table-card">
          <CardHeader className="api-key-card-header">
            <div>
              <CardTitle>Rules</CardTitle>
              <CardDescription>Active and inactive threshold definitions.</CardDescription>
            </div>
            <Badge variant={items.length > 0 ? 'success' : 'muted'}>{items.length} rows</Badge>
          </CardHeader>
          <CardContent>
            <DataTable
              emptyLabel="No alert rules yet"
              headers={['Rule', 'Meter', 'Trigger', 'Condition', 'Window', 'State', 'Actions']}
              rows={items.map((rule) => [
                <RuleName rule={rule} />,
                <span className="mono">{rule.meter}</span>,
                <RuleTrigger rule={rule} />,
                <span>{comparatorLabel(rule.comparator)} {formatNumber(rule.threshold)}</span>,
                <span>{durationLabel(rule.window_seconds)}</span>,
                <RuleState rule={rule} />,
                <span className="table-actions">
                  <Button aria-label={`Evaluate ${rule.name}`} disabled={saving} onClick={() => void appStoreActions.evaluateAlert(rule)} size="icon" type="button" variant="ghost">
                    <Play aria-hidden="true" />
                  </Button>
                  <Button aria-label={`Edit ${rule.name}`} disabled={saving} onClick={() => appStoreActions.setAlertEditing(rule)} size="icon" type="button" variant="ghost">
                    <Pencil aria-hidden="true" />
                  </Button>
                  <Button aria-label={`Delete ${rule.name}`} disabled={saving} onClick={() => appStoreActions.setAlertDeleting(rule)} size="icon" type="button" variant="ghost">
                    <Trash2 aria-hidden="true" />
                  </Button>
                </span>,
              ])}
            />
          </CardContent>
        </Card>
      </section>

      <Card>
        <CardHeader className="api-key-card-header">
          <div>
            <CardTitle>Recent Events</CardTitle>
            <CardDescription>Triggered, resolved, and failed evaluations.</CardDescription>
          </div>
        </CardHeader>
        <CardContent>
          <DataTable
            emptyLabel="No alert events yet"
            headers={['Type', 'Rule', 'Value', 'Message', 'Created']}
            rows={events.map((event) => [
              <Badge variant={event.type === 'triggered' ? 'warning' : event.type === 'resolved' ? 'success' : 'muted'}>{event.type}</Badge>,
              <span className="mono">{event.rule_id}</span>,
              formatNumber(event.value),
              <span>{event.message}</span>,
              formatDate(event.created_at),
            ])}
          />
        </CardContent>
      </Card>

      {editing ? (
        <Modal className="alert-rule-modal" title="Edit Alert Rule" onClose={() => appStoreActions.setAlertEditing(null)}>
          <form className="form-grid alert-rule-modal-form" onSubmit={(event) => void submitUpdate(event)}>
            <label className="wide">
              Name
              <input defaultValue={editing.name} name="name" required />
            </label>
            <label>
              Meter
              <select defaultValue={editing.meter} name="meter" required>
                {meters.map((meter) => <option key={meter.id} value={meter.name}>{meter.name}</option>)}
              </select>
            </label>
            <label>
              Threshold
              <input defaultValue={editing.threshold} name="threshold" required step="any" type="number" />
            </label>
            <label>
              Comparator
              <select defaultValue={editing.comparator} name="comparator">
                {comparators.map(([value, label]) => <option key={value} value={value}>{label}</option>)}
              </select>
            </label>
            <label>
              Window
              <input defaultValue={editing.window_seconds} min="60" name="window_seconds" required type="number" />
            </label>
            <label>
              Evaluate Every
              <input defaultValue={editing.evaluation_interval_seconds} min="1" name="evaluation_interval_seconds" required type="number" />
            </label>
            <label>
              Trigger
              <select defaultValue={editing.trigger_type || 'webhook'} name="trigger_type">
                <option value="webhook">Webhook</option>
              </select>
            </label>
            <label className="wide">
              Webhook URL
              <input defaultValue={editing.webhook_url || ''} name="webhook_url" placeholder="https://example.com/open-spanner/alerts" type="url" />
            </label>
            <label className="wide">
              Subject
              <input defaultValue={editing.subject || ''} name="subject" />
            </label>
            <label className="wide">
              Metadata Filters
              <textarea defaultValue={metadataText(editing.metadata)} name="metadata" rows={3} />
            </label>
            <label className="checkbox-row wide">
              <input defaultChecked={editing.enabled} name="enabled" type="checkbox" />
              Enabled
            </label>
            <div className="modal-actions wide">
              <Button onClick={() => appStoreActions.setAlertEditing(null)} type="button" variant="outline">Cancel</Button>
              <Button disabled={saving} type="submit">Save</Button>
            </div>
          </form>
        </Modal>
      ) : null}

      {deleting ? (
        <Modal title="Delete Alert Rule" onClose={() => appStoreActions.setAlertDeleting(null)}>
          <div className="modal-copy">Delete <strong>{deleting.name}</strong>?</div>
          <div className="modal-actions">
            <Button onClick={() => appStoreActions.setAlertDeleting(null)} type="button" variant="outline">Cancel</Button>
            <Button disabled={saving} onClick={() => void confirmDelete()} type="button">Delete</Button>
          </div>
        </Modal>
      ) : null}
    </>
  )
}

function RuleName({ rule }: { rule: AlertRule }) {
  return (
    <span>
      <strong>{rule.name}</strong>
      <small className="muted block">{rule.enabled ? 'Enabled' : 'Disabled'}</small>
    </span>
  )
}

function RuleState({ rule }: { rule: AlertRule }) {
  const state = rule.state
  if (!state) {
    return <Badge variant="muted">Not evaluated</Badge>
  }
  const variant = state.status === 'alerting' ? 'warning' : state.status === 'ok' ? 'success' : 'muted'
  return (
    <span>
      <Badge variant={variant}>{state.status}</Badge>
      <small className="muted block">{formatNumber(state.value)}</small>
    </span>
  )
}

function RuleTrigger({ rule }: { rule: AlertRule }) {
  return (
    <span>
      <Badge variant="muted">{rule.trigger_type || 'webhook'}</Badge>
      {rule.webhook_url ? <small className="muted block">Configured</small> : <small className="muted block">No URL</small>}
    </span>
  )
}

function alertRequestFromForm(form: FormData): AlertRuleRequest {
  return {
    comparator: String(form.get('comparator') || 'gte'),
    enabled: form.get('enabled') === 'on',
    evaluation_interval_seconds: numberField(form, 'evaluation_interval_seconds'),
    metadata: metadataFromText(String(form.get('metadata') || '')),
    meter: String(form.get('meter') || ''),
    name: String(form.get('name') || ''),
    subject: optionalString(form, 'subject'),
    threshold: numberField(form, 'threshold'),
    trigger_type: String(form.get('trigger_type') || 'webhook'),
    webhook_url: optionalString(form, 'webhook_url'),
    window_seconds: numberField(form, 'window_seconds'),
  }
}

function alertUpdateFromForm(form: FormData): AlertRuleUpdateRequest {
  return alertRequestFromForm(form)
}

function numberField(form: FormData, name: string) {
  return Number(form.get(name) || 0)
}

function optionalString(form: FormData, name: string) {
  const value = String(form.get(name) || '').trim()
  return value || undefined
}

function metadataFromText(value: string) {
  return Object.fromEntries(value
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean)
    .map((line) => {
      const index = line.indexOf('=')
      return index === -1 ? [line, ''] : [line.slice(0, index).trim(), line.slice(index + 1).trim()]
    })
    .filter(([key, item]) => key && item))
}

function metadataText(metadata?: Record<string, string>) {
  return Object.entries(metadata || {})
    .map(([key, value]) => `${key}=${value}`)
    .join('\n')
}

function comparatorLabel(value: string) {
  return comparators.find(([key]) => key === value)?.[1] ?? value
}

function durationLabel(seconds: number) {
  if (seconds % 86400 === 0) {
    return `${seconds / 86400}d`
  }
  if (seconds % 3600 === 0) {
    return `${seconds / 3600}h`
  }
  if (seconds % 60 === 0) {
    return `${seconds / 60}m`
  }
  return `${seconds}s`
}
