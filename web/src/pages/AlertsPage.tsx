import { useParams, useRouter } from '@tanstack/react-router'
import { useSelector } from '@tanstack/react-store'
import { ArrowLeft, ArrowRight, BellRing, Copy, Eye, KeyRound, Loader2, Pencil, Play, Plus, Trash2 } from 'lucide-react'
import { type FormEvent, useCallback, useEffect } from 'react'

import { appStore, appStoreActions } from '../app-store'
import { DataTable, Modal, PageHeader } from '../components/dashboard'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { Checkbox } from '../components/ui/checkbox'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../components/ui/select'
import { Textarea } from '../components/ui/textarea'
import { formatDate, formatNumber } from '../lib/format'
import { useInitialLoad } from '../lib/hooks'
import type { AlertDestination, AlertDestinationRequest, AlertDestinationUpdateRequest, AlertEvent, AlertRule, AlertRuleRequest, AlertRuleUpdateRequest, Meter } from '../api'

const noAlertGroupByValue = '__total__'

const comparators = [
  ['gte', '>='],
  ['gt', '>'],
  ['lte', '<='],
  ['lt', '<'],
  ['eq', '='],
  ['neq', '!='],
] as const

export function AlertsPage() {
  const router = useRouter()
  const {
    creating,
    deleting,
    destinationCreating,
    destinationDeleting,
    destinationEditing,
    destinations,
    editing,
    error,
    items,
    meters,
    saving,
    signingSecret,
  } = useSelector(appStore, (state) => state.alerts)
  const load = useCallback(() => appStoreActions.loadAlerts(), [])
  const groupByOptions = alertGroupByOptions(meters)

  useInitialLoad(load)

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
        title="Alerts"
        description="Manage delivery destinations and threshold rules. Open a rule to inspect events."
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

      <Card className="mb-3 min-w-0">
        <CardHeader className="!px-4 !py-3">
          <div>
            <CardTitle>Destinations</CardTitle>
            <CardDescription>Reusable delivery targets for alert notifications.</CardDescription>
          </div>
          <div className="flex flex-wrap items-center justify-end gap-2">
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

      <Card className="mb-3 min-w-0">
        <CardHeader className="!px-4 !py-3">
          <div>
            <CardTitle>Rules</CardTitle>
            <CardDescription>Active and inactive threshold definitions.</CardDescription>
          </div>
          <div className="flex flex-wrap items-center justify-end gap-2">
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
                <Button aria-label={`Open ${rule.name}`} onClick={() => void router.navigate({ to: '/alerts/$ruleId', params: { ruleId: rule.id } })} size="sm" type="button" variant="outline">
                  Open
                  <ArrowRight aria-hidden="true" />
                </Button>
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

      {creating ? (
        <Modal className="!w-full !max-w-[760px]" title="Create Alert Rule" onClose={() => appStoreActions.setAlertCreating(false)}>
          <form className="form-grid alert-rule-modal-form" onSubmit={(event) => void submitCreate(event)}>
            <Label className="wide grid gap-1.5">
              Name
              <Input name="name" placeholder="High API traffic" required />
            </Label>
            <Label className="grid gap-1.5">
              Meter
              <Select name="meter" required>
                <SelectTrigger className="min-h-[38px] w-full">
                  <SelectValue placeholder="Select meter" />
                </SelectTrigger>
                <SelectContent position="popper">
                  {meters.map((meter) => <SelectItem key={meter.id} value={meter.name}>{meter.name}</SelectItem>)}
                </SelectContent>
              </Select>
            </Label>
            <Label className="grid gap-1.5">
              Threshold
              <Input name="threshold" placeholder="1000" required step="any" type="number" />
            </Label>
            <Label className="grid gap-1.5">
              Comparator
              <Select defaultValue="gte" name="comparator">
                <SelectTrigger className="min-h-[38px] w-full">
                  <SelectValue placeholder="Select comparator" />
                </SelectTrigger>
                <SelectContent position="popper">
                  {comparators.map(([value, label]) => <SelectItem key={value} value={value}>{label}</SelectItem>)}
                </SelectContent>
              </Select>
            </Label>
            <Label className="grid gap-1.5">
              Window
              <Select defaultValue="3600" name="window_seconds">
                <SelectTrigger className="min-h-[38px] w-full">
                  <SelectValue placeholder="Select window" />
                </SelectTrigger>
                <SelectContent position="popper">
                  <SelectItem value="300">5 minutes</SelectItem>
                  <SelectItem value="900">15 minutes</SelectItem>
                  <SelectItem value="3600">1 hour</SelectItem>
                  <SelectItem value="86400">1 day</SelectItem>
                </SelectContent>
              </Select>
            </Label>
            <Label className="grid gap-1.5">
              Evaluate Every
              <Select defaultValue="60" name="evaluation_interval_seconds">
                <SelectTrigger className="min-h-[38px] w-full">
                  <SelectValue placeholder="Select interval" />
                </SelectTrigger>
                <SelectContent position="popper">
                  <SelectItem value="30">30 seconds</SelectItem>
                  <SelectItem value="60">1 minute</SelectItem>
                  <SelectItem value="300">5 minutes</SelectItem>
                  <SelectItem value="900">15 minutes</SelectItem>
                </SelectContent>
              </Select>
            </Label>
            <Label className="grid gap-1.5">
              Evaluate Per
              <Select defaultValue={noAlertGroupByValue} name="group_by">
                <SelectTrigger className="min-h-[38px] w-full">
                  <SelectValue placeholder="Select grouping" />
                </SelectTrigger>
                <SelectContent position="popper">
                  {groupByOptions.map((option) => (
                    <SelectItem key={option.value || noAlertGroupByValue} value={option.value || noAlertGroupByValue}>{option.label}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </Label>
            <Label className="grid gap-1.5">
              Destination
              <Select name="destination_id" required>
                <SelectTrigger className="min-h-[38px] w-full">
                  <SelectValue placeholder="Select destination" />
                </SelectTrigger>
                <SelectContent position="popper">
                  {destinations.map((destination) => <SelectItem key={destination.id} value={destination.id}>{destination.name}</SelectItem>)}
                </SelectContent>
              </Select>
            </Label>
            <Label className="grid gap-1.5">
              Subject
              <Input name="subject" placeholder="Optional subject" />
            </Label>
            <Label className="grid gap-1.5">
              Metadata Filters
              <Textarea name="metadata" placeholder={'region=us-east\nplan=enterprise'} rows={3} />
            </Label>
            <Label className="checkbox-row wide">
              <Checkbox defaultChecked name="enabled" />
              Enabled
            </Label>
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
        <Modal className="!w-full !max-w-[760px]" title="Edit Alert Rule" onClose={() => appStoreActions.setAlertEditing(null)}>
          <form className="form-grid alert-rule-modal-form" onSubmit={(event) => void submitUpdate(event)}>
            <Label className="wide grid gap-1.5">
              Name
              <Input defaultValue={editing.name} name="name" required />
            </Label>
            <Label className="grid gap-1.5">
              Meter
              <Select defaultValue={editing.meter} name="meter" required>
                <SelectTrigger className="min-h-[38px] w-full">
                  <SelectValue placeholder="Select meter" />
                </SelectTrigger>
                <SelectContent position="popper">
                  {meters.map((meter) => <SelectItem key={meter.id} value={meter.name}>{meter.name}</SelectItem>)}
                </SelectContent>
              </Select>
            </Label>
            <Label className="grid gap-1.5">
              Threshold
              <Input defaultValue={editing.threshold} name="threshold" required step="any" type="number" />
            </Label>
            <Label className="grid gap-1.5">
              Comparator
              <Select defaultValue={editing.comparator} name="comparator">
                <SelectTrigger className="min-h-[38px] w-full">
                  <SelectValue placeholder="Select comparator" />
                </SelectTrigger>
                <SelectContent position="popper">
                  {comparators.map(([value, label]) => <SelectItem key={value} value={value}>{label}</SelectItem>)}
                </SelectContent>
              </Select>
            </Label>
            <Label className="grid gap-1.5">
              Window
              <Input defaultValue={editing.window_seconds} min="60" name="window_seconds" required type="number" />
            </Label>
            <Label className="grid gap-1.5">
              Evaluate Every
              <Input defaultValue={editing.evaluation_interval_seconds} min="1" name="evaluation_interval_seconds" required type="number" />
            </Label>
            <Label className="grid gap-1.5">
              Evaluate Per
              <Select defaultValue={editing.group_by || noAlertGroupByValue} name="group_by">
                <SelectTrigger className="min-h-[38px] w-full">
                  <SelectValue placeholder="Select grouping" />
                </SelectTrigger>
                <SelectContent position="popper">
                  {groupByOptions.map((option) => (
                    <SelectItem key={option.value || noAlertGroupByValue} value={option.value || noAlertGroupByValue}>{option.label}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </Label>
            <Label className="wide grid gap-1.5">
              Destination
              <Select defaultValue={editing.destination_id || undefined} name="destination_id" required>
                <SelectTrigger className="min-h-[38px] w-full">
                  <SelectValue placeholder="Select destination" />
                </SelectTrigger>
                <SelectContent position="popper">
                  {destinations.map((destination) => <SelectItem key={destination.id} value={destination.id}>{destination.name}</SelectItem>)}
                </SelectContent>
              </Select>
            </Label>
            <Label className="wide grid gap-1.5">
              Subject
              <Input defaultValue={editing.subject || ''} name="subject" />
            </Label>
            <Label className="wide grid gap-1.5">
              Metadata Filters
              <Textarea defaultValue={metadataText(editing.metadata)} name="metadata" rows={3} />
            </Label>
            <Label className="checkbox-row wide">
              <Checkbox defaultChecked={editing.enabled} name="enabled" />
              Enabled
            </Label>
            <div className="modal-actions wide">
              <Button onClick={() => appStoreActions.setAlertEditing(null)} type="button" variant="outline">Cancel</Button>
              <Button disabled={saving} type="submit">Save</Button>
            </div>
          </form>
        </Modal>
      ) : null}

      {destinationCreating ? (
        <Modal className="!w-full !max-w-[760px]" title="Create Alert Destination" onClose={() => appStoreActions.setAlertDestinationCreating(false)}>
          <form className="form-grid alert-rule-modal-form" onSubmit={(event) => void submitDestinationCreate(event)}>
            <Label className="wide grid gap-1.5">
              Name
              <Input name="name" placeholder="Primary webhook" required />
            </Label>
            <Label className="grid gap-1.5">
              Type
              <Select defaultValue="webhook" name="type">
                <SelectTrigger className="min-h-[38px] w-full">
                  <SelectValue placeholder="Select type" />
                </SelectTrigger>
                <SelectContent position="popper">
                  <SelectItem value="webhook">Webhook</SelectItem>
                </SelectContent>
              </Select>
            </Label>
            <Label className="wide grid gap-1.5">
              Webhook URL
              <Input name="webhook_url" placeholder="https://example.com/open-spanner/alerts" required type="url" />
            </Label>
            <Label className="checkbox-row wide">
              <Checkbox defaultChecked name="enabled" />
              Enabled
            </Label>
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
        <Modal className="!w-full !max-w-[760px]" title="Edit Alert Destination" onClose={() => appStoreActions.setAlertDestinationEditing(null)}>
          <form className="form-grid alert-rule-modal-form" onSubmit={(event) => void submitDestinationUpdate(event)}>
            <Label className="wide grid gap-1.5">
              Name
              <Input defaultValue={destinationEditing.name} name="name" required />
            </Label>
            <Label className="grid gap-1.5">
              Type
              <Select defaultValue={destinationEditing.type || 'webhook'} name="type">
                <SelectTrigger className="min-h-[38px] w-full">
                  <SelectValue placeholder="Select type" />
                </SelectTrigger>
                <SelectContent position="popper">
                  <SelectItem value="webhook">Webhook</SelectItem>
                </SelectContent>
              </Select>
            </Label>
            <Label className="wide grid gap-1.5">
              Webhook URL
              <Input defaultValue={destinationEditing.webhook_url} name="webhook_url" required type="url" />
            </Label>
            <Label className="checkbox-row wide">
              <Checkbox defaultChecked={destinationEditing.enabled} name="enabled" />
              Enabled
            </Label>
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

    </>
  )
}

export function AlertRoutePage() {
  const { ruleId } = useParams({ from: '/_dashboard/alerts_/$ruleId' })

  return <AlertDetailPage ruleId={ruleId} />
}

function AlertDetailPage({ ruleId }: { ruleId: string }) {
  const router = useRouter()
  const {
    error,
    eventLoadingMore,
    eventNextCursor,
    eventStatus,
    events,
    items,
    saving,
    selectedEvent,
  } = useSelector(appStore, (state) => state.alerts)
  const load = useCallback(() => appStoreActions.loadAlerts(), [])
  const pollEvents = useCallback(() => appStoreActions.loadAlertEvents({ quiet: true }), [])
  const rule = items.find((item) => item.id === ruleId) ?? null
  const ruleEvents = events.filter((event) => event.rule_id === ruleId)
  const selectedEventRule = selectedEvent ? ruleForEvent(items, selectedEvent) : null

  useInitialLoad(load)

  useEffect(() => {
    const poll = window.setInterval(() => {
      void pollEvents()
    }, 5000)

    return () => window.clearInterval(poll)
  }, [pollEvents])

  if (!rule && eventStatus !== 'loading') {
    return (
      <>
        <PageHeader
          eyebrow="Alerts"
          icon={<BellRing />}
          title="Alert not found"
          description="This threshold rule may have been deleted or belongs to another workspace."
          action={(
            <Button onClick={() => void router.navigate({ to: '/alerts' })} type="button" variant="outline">
              <ArrowLeft aria-hidden="true" />
              Back to alerts
            </Button>
          )}
        />
        {error ? <div className="error-banner">{error}</div> : null}
      </>
    )
  }

  return (
    <>
      <PageHeader
        eyebrow="Alerts"
        icon={<BellRing />}
        title={rule?.name || 'Alert rule'}
        description={rule ? `${rule.meter} ${comparatorLabel(rule.comparator)} ${formatNumber(rule.threshold)} over ${durationLabel(rule.window_seconds)}` : 'Loading alert rule.'}
        action={(
          <div className="flex flex-wrap justify-end gap-2">
            <Button onClick={() => void router.navigate({ to: '/alerts' })} type="button" variant="outline">
              <ArrowLeft aria-hidden="true" />
              Back
            </Button>
            {rule ? (
              <Button disabled={saving} onClick={() => void appStoreActions.evaluateAlert(rule)} type="button">
                {saving ? <Loader2 className="spin" aria-hidden="true" /> : <Play aria-hidden="true" />}
                Evaluate
              </Button>
            ) : null}
          </div>
        )}
      />

      {error ? <div className="error-banner">{error}</div> : null}

      <div className="grid max-w-[1480px] gap-4">
        <section className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(320px,420px)]">
          <Card className="min-w-0">
            <CardHeader className="!px-4 !py-3">
              <div>
                <CardTitle>Rule</CardTitle>
                <CardDescription>Threshold definition and current state.</CardDescription>
              </div>
            </CardHeader>
            <CardContent>
              {rule ? (
                <DataTable
                  className="!min-w-0"
                  emptyLabel="No rule details"
                  headers={['Meter', 'Condition', 'Window', 'Evaluate Per', 'State']}
                  rows={[[
                    <span className="mono">{rule.meter}</span>,
                    <span>{comparatorLabel(rule.comparator)} {formatNumber(rule.threshold)}</span>,
                    durationLabel(rule.window_seconds),
                    rule.group_by ? groupLabel(rule.group_by) : 'total',
                    <RuleState rule={rule} />,
                  ]]}
                />
              ) : (
                <p className="subject-empty">Loading alert rule.</p>
              )}
            </CardContent>
          </Card>

          <Card className="min-w-0">
            <CardHeader className="!px-4 !py-3">
              <div>
                <CardTitle>Destination</CardTitle>
                <CardDescription>Delivery target used when this rule changes state.</CardDescription>
              </div>
            </CardHeader>
            <CardContent className="grid gap-3 !p-4">
              {rule ? <RuleDestinationDetail rule={rule} /> : <p className="subject-empty">Loading destination.</p>}
            </CardContent>
          </Card>
        </section>

        <Card className="min-w-0">
          <CardHeader className="!px-4 !py-3">
            <div>
              <CardTitle>Recent Events</CardTitle>
              <CardDescription>Triggered, resolved, and failed evaluations for this rule.</CardDescription>
            </div>
          </CardHeader>
          <CardContent>
            <AlertEventTable events={ruleEvents} loading={eventStatus === 'loading'} rules={items} />
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
      </div>

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

function RuleDestinationDetail({ rule }: { rule: AlertRule }) {
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

function AlertEventTable({ events, loading, rules }: { events: AlertEvent[]; loading: boolean; rules: AlertRule[] }) {
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
