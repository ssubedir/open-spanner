import { useSelector } from '@tanstack/react-store'
import { BellRing, Copy, Eye, KeyRound, Loader2, Pencil, Play, Plus, Trash2 } from 'lucide-react'
import { type FormEvent, useCallback, useEffect } from 'react'

import { appStore, appStoreActions } from '../app-store'
import { DataTable, Modal, PageHeader } from '../components/dashboard'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { formatDate, formatNumber } from '../lib/format'
import { useInitialLoad } from '../lib/hooks'
import type { AlertDestination, AlertDestinationRequest, AlertDestinationUpdateRequest, AlertEvent, AlertRule, AlertRuleRequest, AlertRuleUpdateRequest, Meter } from '../api'

const comparators = [
  ['gte', '>='],
  ['gt', '>'],
  ['lte', '<='],
  ['lt', '<'],
  ['eq', '='],
  ['neq', '!='],
] as const

export function AlertsPage() {
  const {
    creating,
    deleting,
    destinationCreating,
    destinationDeleting,
    destinationEditing,
    destinations,
    editing,
    error,
    eventLoadingMore,
    eventNextCursor,
    events,
    items,
    meters,
    saving,
    selectedEvent,
    signingSecret,
  } = useSelector(appStore, (state) => state.alerts)
  const load = useCallback(() => appStoreActions.loadAlerts(), [])
  const pollEvents = useCallback(() => appStoreActions.loadAlertEvents({ quiet: true }), [])
  const selectedEventRule = selectedEvent ? ruleForEvent(items, selectedEvent) : null
  const groupByOptions = alertGroupByOptions(meters)

  useInitialLoad(load)

  useEffect(() => {
    const poll = window.setInterval(() => {
      void pollEvents()
    }, 5000)

    return () => window.clearInterval(poll)
  }, [pollEvents])

  async function submitCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    try {
      await appStoreActions.createAlert(alertRequestFromForm(new FormData(event.currentTarget)))
      event.currentTarget.reset()
      appStoreActions.setAlertCreating(false)
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

  async function submitDestinationCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    try {
      await appStoreActions.createAlertDestination(destinationRequestFromForm(new FormData(event.currentTarget)))
      event.currentTarget.reset()
    } catch {
      // Store owns the visible alerts error state.
    }
  }

  async function submitDestinationUpdate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    try {
      await appStoreActions.updateEditingAlertDestination(destinationUpdateFromForm(new FormData(event.currentTarget)))
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

  async function confirmDestinationDelete() {
    try {
      await appStoreActions.deleteSelectedAlertDestination()
    } catch {
      // Store owns the visible alerts error state.
    }
  }

  async function copySigningSecret() {
    if (!signingSecret) {
      return
    }
    await copyText(signingSecret.secret)
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

      {signingSecret ? (
        <section className="secret-panel" aria-label="Webhook signing secret">
          <div>
            <span>Webhook secret ready</span>
            <strong>{signingSecret.ownerName}</strong>
            <small>Copy this signing secret now. It will not be shown again.</small>
          </div>
          <code title={signingSecret.secret}>{signingSecret.secret}</code>
          <div className="secret-actions">
            <Button onClick={() => void copySigningSecret()} type="button">
              <Copy aria-hidden="true" />
              Copy secret
            </Button>
            <Button onClick={appStoreActions.clearAlertSigningSecret} type="button" variant="outline">Dismiss</Button>
          </div>
        </section>
      ) : null}

      <Card className="api-key-table-card alert-destinations-card">
        <CardHeader className="api-key-card-header">
          <div>
            <CardTitle>Destinations</CardTitle>
            <CardDescription>Reusable delivery targets for alert notifications.</CardDescription>
          </div>
          <div className="card-header-actions">
            <Button disabled={saving} onClick={() => appStoreActions.setAlertDestinationCreating(true)} type="button">
              <Plus aria-hidden="true" />
              New destination
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          <DataTable
            emptyLabel="No alert destinations yet"
            headers={['Destination', 'Webhook URL', 'Signing', 'Updated', 'Actions']}
            rows={destinations.map((destination) => [
              <DestinationName destination={destination} />,
              <span className="mono alert-destination-url" title={destination.webhook_url}>{destination.webhook_url}</span>,
              <DestinationSigning destination={destination} />,
              formatDate(destination.updated_at),
              <span className="table-actions">
                <Button aria-label={`Rotate signing secret for ${destination.name}`} disabled={saving || !destination.webhook_url} onClick={() => void appStoreActions.rotateAlertDestinationSecret(destination)} size="icon" type="button" variant="ghost">
                  <KeyRound aria-hidden="true" />
                </Button>
                <Button aria-label={`Edit ${destination.name}`} disabled={saving} onClick={() => appStoreActions.setAlertDestinationEditing(destination)} size="icon" type="button" variant="ghost">
                  <Pencil aria-hidden="true" />
                </Button>
                <Button aria-label={`Delete ${destination.name}`} disabled={saving} onClick={() => appStoreActions.setAlertDestinationDeleting(destination)} size="icon" type="button" variant="ghost">
                  <Trash2 aria-hidden="true" />
                </Button>
              </span>,
            ])}
          />
        </CardContent>
      </Card>

      <Card className="api-key-table-card alert-rules-card">
        <CardHeader className="api-key-card-header">
          <div>
            <CardTitle>Rules</CardTitle>
            <CardDescription>Active and inactive threshold definitions.</CardDescription>
          </div>
          <div className="card-header-actions">
            <Button disabled={saving} onClick={() => appStoreActions.setAlertCreating(true)} type="button">
              <Plus aria-hidden="true" />
              New rule
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          <DataTable
            emptyLabel="No alert rules yet"
            headers={['Rule', 'Meter', 'Destination', 'Condition', 'Window', 'State', 'Actions']}
            rows={items.map((rule) => [
              <RuleName rule={rule} />,
              <span className="mono">{rule.meter}</span>,
              <RuleDestination rule={rule} />,
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
            headers={['Type', 'Delivery', 'Rule', 'Value', 'Message', 'Created', 'Actions']}
            rows={events.map((event) => {
              const rule = ruleForEvent(items, event)
              return [
                <Badge variant={event.type === 'triggered' ? 'warning' : event.type === 'resolved' ? 'success' : 'muted'}>{event.type}</Badge>,
                <DeliveryBadge event={event} />,
                <EventRule event={event} rule={rule} />,
                <EventValue event={event} />,
                <span>{event.message}</span>,
                formatDate(event.created_at),
                <span className="table-actions">
                  <Button aria-label={`View ${event.type} alert event`} onClick={() => appStoreActions.setAlertSelectedEvent(event)} size="icon" type="button" variant="ghost">
                    <Eye aria-hidden="true" />
                  </Button>
                </span>,
              ]
            })}
          />
          {eventNextCursor ? (
            <div className="pagination-actions">
              <Button disabled={eventLoadingMore} onClick={() => void appStoreActions.loadMoreAlertEvents()} type="button" variant="outline">
                {eventLoadingMore ? <Loader2 className="spin" aria-hidden="true" /> : null}
                Load more events
              </Button>
            </div>
          ) : null}
        </CardContent>
      </Card>

      {creating ? (
        <Modal className="alert-rule-modal" title="Create Alert Rule" onClose={() => appStoreActions.setAlertCreating(false)}>
          <form className="form-grid alert-rule-modal-form" onSubmit={(event) => void submitCreate(event)}>
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
              Evaluate Per
              <select name="group_by" defaultValue="">
                {groupByOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
              </select>
            </label>
            <label>
              Destination
              <select name="destination_id" required>
                <option value="">Select destination</option>
                {destinations.map((destination) => <option key={destination.id} value={destination.id}>{destination.name}</option>)}
              </select>
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
            <div className="modal-actions wide">
              <Button onClick={() => appStoreActions.setAlertCreating(false)} type="button" variant="outline">Cancel</Button>
              <Button disabled={saving} type="submit">
                {saving ? <Loader2 className="spin" aria-hidden="true" /> : <Plus aria-hidden="true" />}
                Create rule
              </Button>
            </div>
          </form>
        </Modal>
      ) : null}

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
              Evaluate Per
              <select defaultValue={editing.group_by || ''} name="group_by">
                {groupByOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
              </select>
            </label>
            <label className="wide">
              Destination
              <select defaultValue={editing.destination_id || ''} name="destination_id" required>
                <option value="">Select destination</option>
                {destinations.map((destination) => <option key={destination.id} value={destination.id}>{destination.name}</option>)}
              </select>
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

      {destinationCreating ? (
        <Modal className="alert-rule-modal" title="Create Alert Destination" onClose={() => appStoreActions.setAlertDestinationCreating(false)}>
          <form className="form-grid alert-rule-modal-form" onSubmit={(event) => void submitDestinationCreate(event)}>
            <label className="wide">
              Name
              <input name="name" placeholder="Primary webhook" required />
            </label>
            <label>
              Type
              <select name="type" defaultValue="webhook">
                <option value="webhook">Webhook</option>
              </select>
            </label>
            <label className="wide">
              Webhook URL
              <input name="webhook_url" placeholder="https://example.com/open-spanner/alerts" required type="url" />
            </label>
            <label className="checkbox-row wide">
              <input defaultChecked name="enabled" type="checkbox" />
              Enabled
            </label>
            <div className="modal-actions wide">
              <Button onClick={() => appStoreActions.setAlertDestinationCreating(false)} type="button" variant="outline">Cancel</Button>
              <Button disabled={saving} type="submit">
                {saving ? <Loader2 className="spin" aria-hidden="true" /> : <Plus aria-hidden="true" />}
                Create
              </Button>
            </div>
          </form>
        </Modal>
      ) : null}

      {destinationEditing ? (
        <Modal className="alert-rule-modal" title="Edit Alert Destination" onClose={() => appStoreActions.setAlertDestinationEditing(null)}>
          <form className="form-grid alert-rule-modal-form" onSubmit={(event) => void submitDestinationUpdate(event)}>
            <label className="wide">
              Name
              <input defaultValue={destinationEditing.name} name="name" required />
            </label>
            <label>
              Type
              <select defaultValue={destinationEditing.type || 'webhook'} name="type">
                <option value="webhook">Webhook</option>
              </select>
            </label>
            <label className="wide">
              Webhook URL
              <input defaultValue={destinationEditing.webhook_url} name="webhook_url" required type="url" />
            </label>
            <label className="checkbox-row wide">
              <input defaultChecked={destinationEditing.enabled} name="enabled" type="checkbox" />
              Enabled
            </label>
            <div className="modal-actions wide">
              <Button onClick={() => appStoreActions.setAlertDestinationEditing(null)} type="button" variant="outline">Cancel</Button>
              <Button disabled={saving} type="submit">Save</Button>
            </div>
          </form>
        </Modal>
      ) : null}

      {destinationDeleting ? (
        <Modal title="Delete Alert Destination" onClose={() => appStoreActions.setAlertDestinationDeleting(null)}>
          <div className="modal-copy">Delete <strong>{destinationDeleting.name}</strong>?</div>
          <div className="modal-actions">
            <Button onClick={() => appStoreActions.setAlertDestinationDeleting(null)} type="button" variant="outline">Cancel</Button>
            <Button disabled={saving} onClick={() => void confirmDestinationDelete()} type="button">Delete</Button>
          </div>
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

      {selectedEvent ? (
        <Modal className="alert-event-modal" title="Alert Event" onClose={() => appStoreActions.setAlertSelectedEvent(null)}>
          <AlertEventDetail event={selectedEvent} rule={selectedEventRule} />
          <div className="modal-actions">
            <Button onClick={() => appStoreActions.setAlertSelectedEvent(null)} type="button" variant="outline">Close</Button>
          </div>
        </Modal>
      ) : null}
    </>
  )
}

function DestinationName({ destination }: { destination: AlertDestination }) {
  return (
    <span>
      <strong>{destination.name}</strong>
      <small className="muted block">{destination.enabled ? 'Enabled' : 'Disabled'} · {destination.type || 'webhook'}</small>
    </span>
  )
}

function DestinationSigning({ destination }: { destination: AlertDestination }) {
  if (!destination.webhook_signing?.enabled) {
    return <Badge variant="muted">Not signed</Badge>
  }
  return (
    <span>
      <Badge variant="success">Signed</Badge>
      <small className="muted block">{destination.webhook_signing.signature_header}</small>
    </span>
  )
}

function RuleName({ rule }: { rule: AlertRule }) {
  return (
    <span>
      <strong>{rule.name}</strong>
      <small className="muted block">{rule.enabled ? 'Enabled' : 'Disabled'}{rule.group_by ? ` · per ${groupLabel(rule.group_by)}` : ''}</small>
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
      <small className="muted block">{state.group_value ? `${groupLabel(state.group_key)} ${state.group_value} · ` : ''}{formatNumber(state.value)}</small>
    </span>
  )
}

function RuleDestination({ rule }: { rule: AlertRule }) {
  if (rule.destination) {
    return (
      <span>
        <Badge variant="muted">{rule.destination.type || 'webhook'}</Badge>
        <small className="muted block">{rule.destination.name}{rule.destination.enabled ? ' · signed' : ' · disabled'}</small>
      </span>
    )
  }

  return (
    <span>
      <Badge variant="warning">Missing</Badge>
      <small className="muted block">{rule.destination_id || 'No destination'}</small>
    </span>
  )
}

function EventRule({ event, rule }: { event: AlertEvent; rule: AlertRule | null }) {
  if (!rule) {
    return (
      <span>
        <strong>Unknown rule</strong>
        <small className="muted block mono">{event.rule_id}</small>
      </span>
    )
  }
  return (
    <span>
      <strong>{rule.name}</strong>
      <small className="muted block mono">{event.group_value ? `${groupLabel(event.group_key)} ${event.group_value}` : rule.meter}</small>
    </span>
  )
}

function EventValue({ event }: { event: AlertEvent }) {
  return (
    <span>
      {formatNumber(event.value)}
      {event.group_value ? <small className="muted block">{groupLabel(event.group_key)} {event.group_value}</small> : null}
    </span>
  )
}

function AlertEventDetail({ event, rule }: { event: AlertEvent; rule: AlertRule | null }) {
  const payload = alertEventJSON(event, rule)
  const signing = ruleSigning(rule)
  return (
    <div className="alert-event-detail">
      <section className="alert-event-hero">
        <div className="alert-event-heading">
          <div>
            <Badge variant={event.type === 'triggered' ? 'warning' : event.type === 'resolved' ? 'success' : 'muted'}>{event.type}</Badge>
            <h3>{rule?.name || 'Unknown alert rule'}</h3>
            <p>{event.message || 'No event message was recorded.'}</p>
          </div>
          <span className="muted">{formatDate(event.created_at)}</span>
        </div>
      </section>

      <section className="alert-event-summary-grid" aria-label="Alert event details">
        <DetailItem label="Meter" value={rule?.meter || 'unknown'} mono />
        <DetailItem label="Value" value={formatNumber(event.value)} />
        <DetailItem label="Condition" value={rule ? `${comparatorLabel(rule.comparator)} ${formatNumber(rule.threshold)}` : 'unknown'} />
        <DetailItem label="Group" value={event.group_value ? `${groupLabel(event.group_key)} ${event.group_value}` : rule?.group_by ? `per ${groupLabel(rule.group_by)}` : 'total'} />
        <DetailItem label="Window" value={rule ? durationLabel(rule.window_seconds) : 'unknown'} />
        <DetailItem label="Delivery" value={deliveryDetail(event)} />
        <DetailItem label="Signature" value={signing?.enabled ? `${signing.signature_header} · ${signing.algorithm}` : 'not configured'} />
        <DetailItem label="Destination" value={rule?.destination?.name || rule?.destination_id || 'not configured'} />
        <DetailItem label="Rule ID" value={event.rule_id} mono wide />
        <DetailItem label="Webhook URL" value={ruleWebhookURL(rule)} mono wide />
        {event.delivery?.error ? <DetailItem label="Delivery Error" value={event.delivery.error} wide /> : null}
      </section>

      <section className="alert-event-json">
        <div className="alert-event-json-header">
          <div>
            <h3>Event JSON</h3>
            <p>Snapshot assembled from the recorded event and current rule.</p>
          </div>
          <Button onClick={() => void copyText(payload)} type="button" variant="outline">
            <Copy aria-hidden="true" />
            Copy
          </Button>
        </div>
        <pre>{payload}</pre>
      </section>
    </div>
  )
}

function DetailItem({ label, mono = false, value, wide = false }: { label: string; mono?: boolean; value: string; wide?: boolean }) {
  return (
    <div className={wide ? 'alert-event-detail-item wide' : 'alert-event-detail-item'}>
      <span>{label}</span>
      <strong className={mono ? 'mono' : undefined}>{value}</strong>
    </div>
  )
}

function DeliveryBadge({ event }: { event: AlertEvent }) {
  const delivery = event.delivery
  if (!delivery) {
    return <Badge variant="muted">Not sent</Badge>
  }
  if (delivery.status === 'delivered') {
    return (
      <span>
        <Badge variant="success">Delivered</Badge>
        <small className="muted block">{delivery.status_code || 'ok'} · {delivery.duration_ms}ms</small>
      </span>
    )
  }
  return (
    <span>
      <Badge variant="warning">Failed</Badge>
      <small className="muted block">{delivery.status_code || 'network'} · {delivery.duration_ms}ms</small>
    </span>
  )
}

function deliveryDetail(event: AlertEvent) {
  const delivery = event.delivery
  if (!delivery) {
    return 'Not sent yet'
  }
  const status = delivery.status === 'delivered' ? 'Delivered' : 'Failed'
  const statusCode = delivery.status_code ? `HTTP ${delivery.status_code}` : 'no status code'
  return `${status} · ${statusCode} · ${delivery.duration_ms}ms`
}

function ruleForEvent(rules: AlertRule[], event: AlertEvent) {
  return rules.find((rule) => rule.id === event.rule_id) ?? null
}

function alertEventJSON(event: AlertEvent, rule: AlertRule | null) {
  return JSON.stringify({
    event,
    rule: rule ? {
      comparator: rule.comparator,
      enabled: rule.enabled,
      evaluation_interval_seconds: rule.evaluation_interval_seconds,
      group_by: rule.group_by || '',
      id: rule.id,
      metadata: rule.metadata || {},
      meter: rule.meter,
      name: rule.name,
      subject: rule.subject || '',
      threshold: rule.threshold,
      destination_id: rule.destination_id || '',
      destination: rule.destination ? {
        enabled: rule.destination.enabled,
        id: rule.destination.id,
        name: rule.destination.name,
        type: rule.destination.type,
      } : null,
      window_seconds: rule.window_seconds,
    } : {
      id: event.rule_id,
    },
    state: rule?.state || null,
  }, null, 2)
}

async function copyText(value: string) {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(value)
    return
  }
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

function alertRequestFromForm(form: FormData): AlertRuleRequest {
  return {
    comparator: String(form.get('comparator') || 'gte'),
    enabled: form.get('enabled') === 'on',
    evaluation_interval_seconds: numberField(form, 'evaluation_interval_seconds'),
    group_by: String(form.get('group_by') || '').trim(),
    metadata: metadataFromText(String(form.get('metadata') || '')),
    meter: String(form.get('meter') || ''),
    name: String(form.get('name') || ''),
    subject: optionalString(form, 'subject'),
    threshold: numberField(form, 'threshold'),
    destination_id: String(form.get('destination_id') || '').trim(),
    window_seconds: numberField(form, 'window_seconds'),
  }
}

function alertUpdateFromForm(form: FormData): AlertRuleUpdateRequest {
  return alertRequestFromForm(form)
}

function destinationRequestFromForm(form: FormData): AlertDestinationRequest {
  return {
    enabled: form.get('enabled') === 'on',
    name: String(form.get('name') || ''),
    type: String(form.get('type') || 'webhook'),
    webhook_url: String(form.get('webhook_url') || ''),
  }
}

function destinationUpdateFromForm(form: FormData): AlertDestinationUpdateRequest {
  return destinationRequestFromForm(form)
}

function ruleSigning(rule: AlertRule | null) {
  return rule?.destination?.webhook_signing || null
}

function ruleWebhookURL(rule: AlertRule | null) {
  return rule?.destination?.webhook_url || 'not configured'
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

function alertGroupByOptions(meters: Meter[]) {
  const options = new Map<string, string>([
    ['', 'Total'],
    ['subject', 'Subject'],
  ])
  for (const meter of meters) {
    for (const dimension of meter.dimensions || []) {
      if (!dimension.name || dimension.deprecated) {
        continue
      }
      options.set(dimension.name, groupLabel(dimension.name))
    }
  }
  return Array.from(options, ([value, label]) => ({ label, value }))
}

function groupLabel(value?: string) {
  const field = String(value || '').replace(/^metadata\./, '')
  if (!field) {
    return 'total'
  }
  if (field === 'subject') {
    return 'subject'
  }
  return field
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
