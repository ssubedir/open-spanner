import { useSelector } from '@tanstack/react-store'
import { Boxes, Loader2, Pencil, Plus, Trash2 } from 'lucide-react'
import { type FormEvent, useCallback } from 'react'

import { appStore, appStoreActions, type MeterDimensionDraft } from '../app-store'
import { EmptyRow, Modal, PageHeader } from '../components/dashboard'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table'
import { formatDate, formatNumber } from '../lib/format'
import { useInitialLoad } from '../lib/hooks'
import { metadataSchemaFromRows } from '../lib/metadata'

const aggregations = ['sum', 'count', 'avg', 'min', 'max', 'first', 'last', 'rate']
const metadataTypes = ['string', 'number', 'boolean']

export function MetersPage() {
  const { createDimensions, deleting, editDimensions, editing, error, items: meters, saving, stats } = useSelector(appStore, (state) => state.meters)
  const load = useCallback(() => appStoreActions.loadMeters(), [])

  useInitialLoad(load)

  async function submitCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const formElement = event.currentTarget
    const form = new FormData(formElement)

    const metadataSchema = readMetadataSchema(createDimensions)
    if (!metadataSchema) {
      return
    }

    try {
      await appStoreActions.createMeter({
        aggregation: String(form.get('aggregation') || 'sum'),
        description: String(form.get('description') || ''),
        event_retention_days: Number(form.get('event_retention_days') || 90),
        metadata_schema: metadataSchema,
        name: String(form.get('name') || ''),
        unit: String(form.get('unit') || ''),
      })
      formElement.reset()
      appStoreActions.resetMeterCreateDimensions()
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

    const metadataSchema = readMetadataSchema(editDimensions)
    if (!metadataSchema) {
      return
    }

    try {
      await appStoreActions.updateEditingMeter({
        aggregation: String(form.get('aggregation') || editing.aggregation),
        description: String(form.get('description') || ''),
        event_retention_days: Number(form.get('event_retention_days') || editing.event_retention_days),
        metadata_schema: metadataSchema,
        unit: String(form.get('unit') || ''),
      })
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
        action={null}
      />

      {error ? <div className="error-banner">{error}</div> : null}

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
              <DimensionSchemaEditor
                rows={createDimensions}
                onAdd={() => appStoreActions.addMeterCreateDimension()}
                onRemove={(id) => appStoreActions.removeMeterCreateDimension(id)}
                onUpdate={(id, update) => appStoreActions.updateMeterCreateDimension(id, update)}
              />
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
                        <TableCell><DimensionChips schema={meter.metadata_schema} /></TableCell>
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
              Unit
              <input defaultValue={editing.unit} name="unit" required />
            </label>
            <label>
              Aggregation
              <select defaultValue={editing.aggregation} name="aggregation" required>
                {aggregations.map((item) => <option key={item} value={item}>{item}</option>)}
              </select>
            </label>
            <label>
              Retention Days
              <input defaultValue={editing.event_retention_days} max="3650" min="1" name="event_retention_days" required type="number" />
            </label>
            <label>
              Description
              <input defaultValue={editing.description} name="description" />
            </label>
            <DimensionSchemaEditor
              rows={editDimensions}
              onAdd={() => appStoreActions.addMeterEditDimension()}
              onRemove={(id) => appStoreActions.removeMeterEditDimension(id)}
              onUpdate={(id, update) => appStoreActions.updateMeterEditDimension(id, update)}
            />
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

function readMetadataSchema(rows: MeterDimensionDraft[]) {
  try {
    return metadataSchemaFromRows(rows)
  } catch (err) {
    appStoreActions.setMetersError(err instanceof Error ? err.message : 'Unable to read meter dimensions')
    return null
  }
}

function DimensionSchemaEditor({
  onAdd,
  onRemove,
  onUpdate,
  rows,
}: {
  onAdd: () => void
  onRemove: (id: string) => void
  onUpdate: (id: string, update: Partial<Omit<MeterDimensionDraft, 'id'>>) => void
  rows: MeterDimensionDraft[]
}) {
  return (
    <div className="schema-builder wide">
      <div className="schema-builder-header">
        <span>Dimensions</span>
        <Button onClick={onAdd} size="sm" type="button" variant="outline">
          <Plus aria-hidden="true" />
          Add
        </Button>
      </div>
      <div className="schema-rows">
        {rows.map((row) => (
          <div className="schema-row" key={row.id}>
            <input
              aria-label="Dimension name"
              onChange={(event) => onUpdate(row.id, { name: event.currentTarget.value })}
              placeholder="region"
              value={row.name}
            />
            <select
              aria-label="Dimension type"
              onChange={(event) => onUpdate(row.id, { type: event.currentTarget.value })}
              value={row.type}
            >
              {metadataTypes.map((type) => <option key={type} value={type}>{type}</option>)}
            </select>
            <Button aria-label={`Remove ${row.name || 'dimension'}`} onClick={() => onRemove(row.id)} size="icon" type="button" variant="ghost">
              <Trash2 aria-hidden="true" />
            </Button>
          </div>
        ))}
      </div>
    </div>
  )
}

function DimensionChips({ schema }: { schema: Record<string, string> }) {
  const dimensions = Object.entries(schema || {}).sort(([left], [right]) => left.localeCompare(right))
  if (dimensions.length === 0) {
    return <span className="muted">No dimensions</span>
  }

  return (
    <div className="schema-chips">
      {dimensions.map(([name, type]) => (
        <span className="schema-chip" key={name}>
          <span>{name}</span>
          <strong>{type}</strong>
        </span>
      ))}
    </div>
  )
}
