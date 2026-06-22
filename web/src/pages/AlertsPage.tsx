import { useRouter } from '@tanstack/react-router'
import { useSelector } from '@tanstack/react-store'
import { ArrowRight, BellRing, Copy, KeyRound, Loader2, Pencil, Play, Plus, Trash2 } from 'lucide-react'
import { type FormEvent, useCallback } from 'react'

import { appStore, appStoreActions } from '../app-store'
import { DataTable, Modal, PageHeader } from '../components/dashboard'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { Checkbox } from '../components/ui/checkbox'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../components/ui/select'
import { Textarea } from '../components/ui/textarea'
import { formatDate, formatNumber } from '../lib/format'
import { useInitialLoad } from '../lib/hooks'
import {
  DestinationName,
  DestinationSigning,
  RuleDestination,
  RuleName,
  RuleState,
  alertGroupByOptions,
  alertRequestFromForm,
  alertUpdateFromForm,
  comparatorLabel,
  comparators,
  copyText,
  destinationRequestFromForm,
  destinationUpdateFromForm,
  durationLabel,
  metadataText,
  noAlertGroupByValue,
} from './AlertPageParts'

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
