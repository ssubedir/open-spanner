import { Copy, Eye } from 'lucide-react'

import { appStoreActions } from '../app-store'
import { DataTable } from '../components/dashboard'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import type { AlertDestination, AlertDestinationRequest, AlertDestinationUpdateRequest, AlertEvent, AlertRule, AlertRuleRequest, AlertRuleUpdateRequest, Meter } from '../api'
import { formatDate, formatNumber } from '../lib/format'

export const noAlertGroupByValue = '__total__'

export const comparators = [
  ['gte', '>='],
  ['gt', '>'],
  ['lte', '<='],
  ['lt', '<'],
  ['eq', '='],
  ['neq', '!='],
] as const

export function DestinationName({ destination }: { destination: AlertDestination }) {
  return (
    <span>
      <strong>{destination.name}</strong>
      <small className="muted block">{destination.enabled ? 'Enabled' : 'Disabled'} · {destination.type || 'webhook'}</small>
    </span>
  )
}

export function DestinationSigning({ destination }: { destination: AlertDestination }) {
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

export function RuleName({ rule }: { rule: AlertRule }) {
  return (
    <span>
      <strong>{rule.name}</strong>
      <small className="muted block">{rule.enabled ? 'Enabled' : 'Disabled'}{rule.group_by ? ` · per ${groupLabel(rule.group_by)}` : ''}</small>
    </span>
  )
}

export function RuleState({ rule }: { rule: AlertRule }) {
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

export function RuleDestination({ rule }: { rule: AlertRule }) {
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

export function RuleDestinationDetail({ rule }: { rule: AlertRule }) {
  if (!rule.destination) {
    return (
      <div className="grid gap-1 rounded-md border border-border bg-[#f8fafc] p-3">
        <Badge variant="warning">Missing destination</Badge>
        <span className="text-sm text-muted">{rule.destination_id || 'No destination selected'}</span>
      </div>
    )
  }

  return (
    <div className="grid gap-3 rounded-md border border-border bg-[#f8fafc] p-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <strong>{rule.destination.name}</strong>
        <Badge variant={rule.destination.enabled ? 'success' : 'muted'}>{rule.destination.enabled ? 'Enabled' : 'Disabled'}</Badge>
      </div>
      <span className="mono truncate text-sm" title={rule.destination.webhook_url}>{rule.destination.webhook_url}</span>
      <DestinationSigning destination={rule.destination} />
    </div>
  )
}

export function AlertEventTable({ events, loading, rules }: { events: AlertEvent[]; loading: boolean; rules: AlertRule[] }) {
  return (
    <DataTable
      emptyLabel={loading ? 'Loading alert events' : 'No alert events yet'}
      headers={['Type', 'Delivery', 'Rule', 'Value', 'Message', 'Created', 'Actions']}
      rows={events.map((event) => {
        const rule = ruleForEvent(rules, event)
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
  )
}

export function AlertEventDetail({ event, rule }: { event: AlertEvent; rule: AlertRule | null }) {
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

export function ruleForEvent(rules: AlertRule[], event: AlertEvent) {
  return rules.find((rule) => rule.id === event.rule_id) ?? null
}

export async function copyText(value: string) {
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

export function alertRequestFromForm(form: FormData): AlertRuleRequest {
  return {
    comparator: String(form.get('comparator') || 'gte'),
    enabled: form.get('enabled') === 'on',
    evaluation_interval_seconds: numberField(form, 'evaluation_interval_seconds'),
    group_by: optionalSelectField(form, 'group_by', noAlertGroupByValue),
    metadata: metadataFromText(String(form.get('metadata') || '')),
    meter: String(form.get('meter') || ''),
    name: String(form.get('name') || ''),
    subject: optionalString(form, 'subject'),
    threshold: numberField(form, 'threshold'),
    destination_id: String(form.get('destination_id') || '').trim(),
    window_seconds: numberField(form, 'window_seconds'),
  }
}

export function alertUpdateFromForm(form: FormData): AlertRuleUpdateRequest {
  return alertRequestFromForm(form)
}

export function destinationRequestFromForm(form: FormData): AlertDestinationRequest {
  return {
    enabled: form.get('enabled') === 'on',
    name: String(form.get('name') || ''),
    type: String(form.get('type') || 'webhook'),
    webhook_url: String(form.get('webhook_url') || ''),
  }
}

export function destinationUpdateFromForm(form: FormData): AlertDestinationUpdateRequest {
  return destinationRequestFromForm(form)
}

export function metadataText(metadata?: Record<string, string>) {
  return Object.entries(metadata || {})
    .map(([key, value]) => `${key}=${value}`)
    .join('\n')
}

export function alertGroupByOptions(meters: Meter[]) {
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

export function groupLabel(value?: string) {
  const field = String(value || '').replace(/^metadata\./, '')
  if (!field) {
    return 'total'
  }
  if (field === 'subject') {
    return 'subject'
  }
  return field
}

export function comparatorLabel(value: string) {
  return comparators.find(([key]) => key === value)?.[1] ?? value
}

export function durationLabel(seconds: number) {
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

function DetailItem({ label, mono = false, value, wide = false }: { label: string; mono?: boolean; value: string; wide?: boolean }) {
  return (
    <div className={wide ? 'alert-event-detail-item wide' : 'alert-event-detail-item'}>
      <span>{label}</span>
      <strong className={mono ? 'mono' : undefined}>{value}</strong>
    </div>
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

function optionalSelectField(form: FormData, name: string, emptyValue: string) {
  const value = String(form.get(name) || '').trim()
  return value === emptyValue ? '' : value
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
