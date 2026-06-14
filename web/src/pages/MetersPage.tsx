import { useSelector } from '@tanstack/react-store'
import { BarChart3, Boxes, Clock, Loader2, Pencil, Plus, RefreshCw, Rows3, Trash2 } from 'lucide-react'
import { type FormEvent, useCallback } from 'react'

import type { Meter, MeterStats } from '../api'
import { appStore, appStoreActions } from '../app-store'
import { EmptyRow, MetricCard, Modal, PageHeader } from '../components/dashboard'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table'
import { formatDate, formatNumber } from '../lib/format'
import { useInitialLoad } from '../lib/hooks'
import { parseMetadataSchema } from '../lib/metadata'

const aggregations = ['sum', 'count', 'avg', 'min', 'max', 'first', 'last', 'rate']

export function MetersPage() {
  const { deleting, editing, error, items: meters, saving, stats, status } = useSelector(appStore, (state) => state.meters)
  const load = useCallback(() => appStoreActions.loadMeters(), [])

  useInitialLoad(load)

  async function submitCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const formElement = event.currentTarget
    const form = new FormData(formElement)

    try {
      await appStoreActions.createMeter({
        aggregation: String(form.get('aggregation') || 'sum'),
        description: String(form.get('description') || ''),
        event_retention_days: Number(form.get('event_retention_days') || 90),
        metadata_schema: parseMetadataSchema(String(form.get('metadata_schema') || '{}')),
        name: String(form.get('name') || ''),
        unit: String(form.get('unit') || ''),
      })
      formElement.reset()
      const metadata = formElement.elements.namedItem('metadata_schema')
      if (metadata instanceof HTMLTextAreaElement) {
        metadata.value = '{}'
      }
    } catch {
      // Store owns the visible meters error state.
    }
  }

  async function submitEdit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!editing) {
      return
    }
    const form = new FormData(event.currentTarget)

    try {
      await appStoreActions.updateEditingMeter({ description: String(form.get('description') || '') })
    } catch {
      // Store owns the visible meters error state.
    }
  }

  async function confirmDelete() {
    try {
      await appStoreActions.deleteSelectedMeter()
    } catch {
      // Store owns the visible meters error state.
    }
  }

  return (
    <>
      <PageHeader
        eyebrow="Meters"
        icon={<Boxes />}
        title="Meter definitions"
        description="Create and maintain the billable signals accepted by the usage API."
        action={(
          <Button disabled={status === 'loading'} onClick={() => void load()} type="button" variant="outline">
            {status === 'loading' ? <Loader2 className="spin" aria-hidden="true" /> : <RefreshCw aria-hidden="true" />}
            Refresh
          </Button>
        )}
      />

      {error ? <div className="error-banner">{error}</div> : null}

      <section className="metric-grid meters-metrics" aria-label="Meter metrics">
        <MetricCard icon={<Boxes />} label="Meters" value={meters.length} helper="Definitions configured" />
        <MetricCard icon={<BarChart3 />} label="Usage Events" value={sumMeterEvents(stats)} helper="Events attached to meters" />
        <MetricCard icon={<Rows3 />} label="Aggregations" value={new Set(meters.map((meter) => meter.aggregation)).size} helper="Aggregation modes in use" />
        <MetricCard icon={<Clock />} label="Avg Retention" value={averageRetention(meters)} helper="Days across meters" />
      </section>

      <section className="meters-grid">
        <Card>
          <CardHeader>
            <div>
              <CardTitle>Create Meter</CardTitle>
              <CardDescription>Define a signal, its unit, aggregation, and metadata contract.</CardDescription>
            </div>
          </CardHeader>
          <CardContent className="form-card">
            <form className="form-grid" onSubmit={(event) => void submitCreate(event)}>
              <label>
                Name
                <input id="meter-name" name="name" placeholder="api_calls" required />
              </label>
              <label>
                Unit
                <input id="meter-unit" name="unit" placeholder="request" required />
              </label>
              <label>
                Aggregation
                <select name="aggregation" required>
                  {aggregations.map((item) => <option key={item} value={item}>{item}</option>)}
                </select>
              </label>
              <label>
                Retention Days
                <input defaultValue="90" max="3650" min="1" name="event_retention_days" required type="number" />
              </label>
              <label className="wide">
                Description
                <input id="meter-description" name="description" placeholder="API requests accepted by the platform" />
              </label>
              <label className="wide" htmlFor="meter-metadata-schema">
                Metadata Schema JSON
                <textarea aria-label="Metadata Schema JSON" defaultValue="{}" id="meter-metadata-schema" name="metadata_schema" rows={5} />
              </label>
              <Button disabled={saving} type="submit">
                {saving ? <Loader2 className="spin" aria-hidden="true" /> : <Plus aria-hidden="true" />}
                Create
              </Button>
            </form>
          </CardContent>
        </Card>

        <Card className="meter-table-card">
          <CardHeader>
            <div>
              <CardTitle>Meters</CardTitle>
              <CardDescription>Configured meter definitions and current activity.</CardDescription>
            </div>
            <Badge variant={meters.length > 0 ? 'success' : 'muted'}>{meters.length} rows</Badge>
          </CardHeader>
          <CardContent>
            <div className="table-wrap">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Name</TableHead>
                    <TableHead>Aggregation</TableHead>
                    <TableHead>Unit</TableHead>
                    <TableHead>Retention</TableHead>
                    <TableHead>Events</TableHead>
                    <TableHead>Last Event</TableHead>
                    <TableHead>Schema</TableHead>
                    <TableHead>Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {meters.length === 0 ? (
                    <EmptyRow colSpan={8} label="No meters yet" />
                  ) : meters.map((meter) => {
                    const stat = stats[meter.name]
                    return (
                      <TableRow key={meter.id}>
                        <TableCell>
                          <div className="stack-cell">
                            <strong>{meter.name}</strong>
                            <small>{meter.description || 'No description'}</small>
                          </div>
                        </TableCell>
                        <TableCell><Badge variant="muted">{meter.aggregation}</Badge></TableCell>
                        <TableCell>{meter.unit}</TableCell>
                        <TableCell>{meter.event_retention_days} days</TableCell>
                        <TableCell>{formatNumber(stat?.usage_events ?? 0)}</TableCell>
                        <TableCell>{stat?.last_event_at ? formatDate(stat.last_event_at) : 'Never'}</TableCell>
                        <TableCell className="mono truncate">{JSON.stringify(meter.metadata_schema || {})}</TableCell>
                        <TableCell>
                          <div className="table-actions">
                            <Button aria-label={`Edit ${meter.name}`} onClick={() => appStoreActions.setMeterEditing(meter)} size="icon" type="button" variant="ghost">
                              <Pencil aria-hidden="true" />
                            </Button>
                            <Button aria-label={`Delete ${meter.name}`} onClick={() => appStoreActions.setMeterDeleting(meter)} size="icon" type="button" variant="ghost">
                              <Trash2 aria-hidden="true" />
                            </Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    )
                  })}
                </TableBody>
              </Table>
            </div>
          </CardContent>
        </Card>
      </section>

      {editing ? (
        <Modal title="Edit Meter" onClose={() => appStoreActions.setMeterEditing(null)}>
          <form className="modal-form" onSubmit={(event) => void submitEdit(event)}>
            <label>
              Name
              <input disabled value={editing.name} />
            </label>
            <label>
              Description
              <textarea defaultValue={editing.description} name="description" rows={5} />
            </label>
            <div className="modal-actions">
              <Button onClick={() => appStoreActions.setMeterEditing(null)} type="button" variant="outline">Cancel</Button>
              <Button disabled={saving} type="submit">Save</Button>
            </div>
          </form>
        </Modal>
      ) : null}

      {deleting ? (
        <Modal title="Delete Meter" onClose={() => appStoreActions.setMeterDeleting(null)}>
          <div className="modal-copy">Delete <strong>{deleting.name}</strong>?</div>
          <div className="modal-actions">
            <Button onClick={() => appStoreActions.setMeterDeleting(null)} type="button" variant="outline">Cancel</Button>
            <Button disabled={saving} onClick={() => void confirmDelete()} type="button">Delete</Button>
          </div>
        </Modal>
      ) : null}
    </>
  )
}

function sumMeterEvents(stats: Record<string, MeterStats>) {
  return Object.values(stats).reduce((sum, item) => sum + Number(item.usage_events || 0), 0)
}

function averageRetention(meters: Meter[]) {
  if (meters.length === 0) {
    return 0
  }
  return Math.round(meters.reduce((sum, meter) => sum + meter.event_retention_days, 0) / meters.length)
}
