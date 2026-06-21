import { useSelector } from '@tanstack/react-store'
import { Boxes, ChevronDown, Loader2, Lock, Pencil, Plus, Trash2 } from 'lucide-react'
import { type FormEvent, useCallback, useState } from 'react'

import { appStore, appStoreActions, type MeterDimensionDraft } from '../app-store'
import type { Meter, MeterDimension } from '../api'
import { EmptyRow, Modal, PageHeader } from '../components/dashboard'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { Checkbox } from '../components/ui/checkbox'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table'
import { formatDate, formatNumber } from '../lib/format'
import { useInitialLoad } from '../lib/hooks'
import { metadataDimensionNameError, meterDimensionsFromRows } from '../lib/metadata'

const aggregations = ['sum', 'count', 'avg', 'min', 'max', 'first', 'last', 'rate']
const metadataTypes = ['string', 'number', 'boolean']

export function MetersPage() {
  const { creating, createDimensions, deleting, editDimensions, editing, error, items: meters, saving, stats } = useSelector(appStore, (state) => state.meters)
  const load = useCallback(() => appStoreActions.loadMeters(), [])

  useInitialLoad(load)

  const editingUsageEvents = editing ? stats[editing.name]?.usage_events ?? 0 : 0
  const editingDimensionsLocked = editingUsageEvents > 0

  async function submitCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const formElement = event.currentTarget
    const form = new FormData(formElement)

    const dimensions = readMeterDimensions(createDimensions)
    if (!dimensions) {
      return
    }

    try {
      await appStoreActions.createMeter({
        aggregation: String(form.get('aggregation') || 'sum'),
        description: String(form.get('description') || ''),
        dimensions,
        event_retention_days: Number(form.get('event_retention_days') || 90),
        name: String(form.get('name') || ''),
        unit: String(form.get('unit') || ''),
      })
      formElement.reset()
      appStoreActions.resetMeterCreateDimensions()
      appStoreActions.setMeterCreating(false)
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

    if (editingDimensionsLocked) {
      const lockedError = lockedDimensionDraftError(editDimensions)
      if (lockedError) {
        appStoreActions.setMetersError(lockedError)
        return
      }
    }

    const dimensions = readMeterDimensions(editDimensions)
    if (!dimensions) {
      return
    }

    try {
      await appStoreActions.updateEditingMeter({
        aggregation: String(form.get('aggregation') || editing.aggregation),
        description: String(form.get('description') || ''),
        dimensions,
        event_retention_days: Number(form.get('event_retention_days') || editing.event_retention_days),
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

      <Card className="min-w-0">
        <CardHeader className="!px-4 !py-3">
          <div>
            <CardTitle>Meters</CardTitle>
            <CardDescription>Configured meter definitions and current activity.</CardDescription>
          </div>
          <div className="flex flex-wrap items-center justify-end gap-2">
            <Button disabled={saving} onClick={() => appStoreActions.setMeterCreating(true)} type="button">
              <Plus aria-hidden="true" />
              New meter
            </Button>
          </div>
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
                        <div className="grid min-w-[180px] gap-1">
                          <strong>{meter.name}</strong>
                          <small className="max-w-[360px] truncate text-xs text-muted">{meter.description || 'No description'}</small>
                        </div>
                      </TableCell>
                      <TableCell><Badge variant="muted">{meter.aggregation}</Badge></TableCell>
                      <TableCell>{meter.unit}</TableCell>
                      <TableCell>{meter.event_retention_days} days</TableCell>
                      <TableCell>{formatNumber(stat?.usage_events ?? 0)}</TableCell>
                      <TableCell>{stat?.last_event_at ? formatDate(stat.last_event_at) : 'Never'}</TableCell>
                      <TableCell><DimensionChips meter={meter} /></TableCell>
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

      {creating ? (
        <Modal className="!w-full !max-w-[760px]" title="Create Meter" onClose={() => appStoreActions.setMeterCreating(false)}>
          <form className="form-grid meter-schema-form max-h-[calc(100vh-140px)] overflow-auto !p-4" onSubmit={(event) => void submitCreate(event)}>
            <Label className="grid gap-1.5">
              Name
              <Input id="meter-name" name="name" placeholder="api_calls" required />
            </Label>
            <Label className="grid gap-1.5">
              Unit
              <Input id="meter-unit" name="unit" placeholder="request" required />
            </Label>
            <Label className="grid gap-1.5">
              Aggregation
              <Select defaultValue="sum" name="aggregation" required>
                <SelectTrigger className="min-h-[38px] w-full">
                  <SelectValue placeholder="Select aggregation" />
                </SelectTrigger>
                <SelectContent position="popper">
                  {aggregations.map((item) => <SelectItem key={item} value={item}>{item}</SelectItem>)}
                </SelectContent>
              </Select>
            </Label>
            <Label className="grid gap-1.5">
              Retention Days
              <Input defaultValue="90" max="3650" min="1" name="event_retention_days" required type="number" />
            </Label>
            <Label className="wide grid gap-1.5">
              Description
              <Input id="meter-description" name="description" placeholder="API requests accepted by the platform" />
            </Label>
            <DimensionSchemaEditor
              rows={createDimensions}
              onAdd={() => appStoreActions.addMeterCreateDimension()}
              onRemove={(id) => appStoreActions.removeMeterCreateDimension(id)}
              onUpdate={(id, update) => appStoreActions.updateMeterCreateDimension(id, update)}
              showDeprecated={false}
            />
            <div className="modal-actions wide">
              <Button onClick={() => appStoreActions.setMeterCreating(false)} type="button" variant="outline">Cancel</Button>
              <Button disabled={saving} type="submit">
                {saving ? <Loader2 className="spin" aria-hidden="true" /> : <Plus aria-hidden="true" />}
                Create meter
              </Button>
            </div>
          </form>
        </Modal>
      ) : null}

      {editing ? (
        <Modal className="!w-full !max-w-[760px]" title="Edit Meter" onClose={() => appStoreActions.setMeterEditing(null)}>
          <form className="modal-form meter-schema-form max-h-[calc(100vh-140px)] overflow-auto !p-4" onSubmit={(event) => void submitEdit(event)}>
            <Label className="grid gap-1.5">
              Name
              <Input disabled value={editing.name} />
            </Label>
            <Label className="grid gap-1.5">
              Unit
              <Input defaultValue={editing.unit} name="unit" required />
            </Label>
            <Label className="grid gap-1.5">
              Aggregation
              <Select defaultValue={editing.aggregation} name="aggregation" required>
                <SelectTrigger className="min-h-[38px] w-full">
                  <SelectValue placeholder="Select aggregation" />
                </SelectTrigger>
                <SelectContent position="popper">
                  {aggregations.map((item) => <SelectItem key={item} value={item}>{item}</SelectItem>)}
                </SelectContent>
              </Select>
            </Label>
            <Label className="grid gap-1.5">
              Retention Days
              <Input defaultValue={editing.event_retention_days} max="3650" min="1" name="event_retention_days" required type="number" />
            </Label>
            <Label className="wide grid gap-1.5">
              Description
              <Input defaultValue={editing.description} name="description" />
            </Label>
            <DimensionSchemaEditor
              lockedByUsage={editingDimensionsLocked}
              rows={editDimensions}
              usageEvents={editingUsageEvents}
              onAdd={() => appStoreActions.addMeterEditDimension()}
              onRemove={(id) => appStoreActions.removeMeterEditDimension(id)}
              onUpdate={(id, update) => appStoreActions.updateMeterEditDimension(id, update)}
            />
            <div className="modal-actions wide">
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

function readMeterDimensions(rows: MeterDimensionDraft[]) {
  try {
    return meterDimensionsFromRows(rows)
  } catch (err) {
    appStoreActions.setMetersError(err instanceof Error ? err.message : 'Unable to read meter dimensions')
    return null
  }
}

function lockedDimensionDraftError(rows: MeterDimensionDraft[]) {
  for (const row of rows) {
    if (!row.originalName) {
      if (!row.name.trim()) {
        continue
      }
      if (row.required && !row.deprecated) {
        return 'New dimensions must be optional after usage has been recorded.'
      }
      continue
    }

    if (row.name !== row.originalName) {
      return 'Existing dimension names cannot change after usage has been recorded.'
    }
    if (row.originalType && row.type !== row.originalType) {
      return 'Existing dimension types cannot change after usage has been recorded.'
    }
    if ((row.originalRequired === false || row.originalDeprecated) && row.required && !row.deprecated) {
      return 'Optional dimensions cannot become required after usage has been recorded.'
    }
  }
  return ''
}

function DimensionSchemaEditor({
  lockedByUsage = false,
  onAdd,
  onRemove,
  onUpdate,
  rows,
  showDeprecated = true,
  usageEvents = 0,
}: {
  lockedByUsage?: boolean
  onAdd: () => void
  onRemove: (id: string) => void
  onUpdate: (id: string, update: Partial<Omit<MeterDimensionDraft, 'id'>>) => void
  rows: MeterDimensionDraft[]
  showDeprecated?: boolean
  usageEvents?: number
}) {
  const [expanded, setExpanded] = useState(false)
  const dimensionCount = rows.filter((row) => row.name.trim()).length
  const requiredCount = rows.filter((row) => row.name.trim() && row.required && !row.deprecated).length
  const deprecatedCount = rows.filter((row) => row.name.trim() && row.deprecated).length
  const summary =
    dimensionCount === 0
      ? 'No dimensions'
      : [
          `${formatNumber(dimensionCount)} ${dimensionCount === 1 ? 'dimension' : 'dimensions'}`,
          `${formatNumber(requiredCount)} required`,
          deprecatedCount > 0 ? `${formatNumber(deprecatedCount)} deprecated` : '',
        ]
          .filter(Boolean)
          .join(' · ')

  return (
    <div className="schema-builder wide">
      <div className="schema-builder-header">
        <button
          aria-expanded={expanded}
          className="schema-toggle"
          data-testid="meter-dimensions-toggle"
          onClick={() => setExpanded((current) => !current)}
          type="button"
        >
          <ChevronDown aria-hidden="true" />
          <span>Dimensions</span>
          <small>{summary}</small>
        </button>
        <div className="schema-builder-actions">
          <Button onClick={() => setExpanded((current) => !current)} size="sm" type="button" variant="outline">
            {expanded ? 'Hide' : 'Edit'}
          </Button>
          <Button
            onClick={() => {
              onAdd()
              setExpanded(true)
            }}
            size="sm"
            type="button"
            variant="outline"
          >
            <Plus aria-hidden="true" />
            Add
          </Button>
        </div>
      </div>
      {lockedByUsage ? (
        <div className="schema-lock-note">
          <Lock aria-hidden="true" />
          <span>{formatNumber(usageEvents)} usage events recorded. Existing dimension identity is locked.</span>
        </div>
      ) : null}
      <div className="schema-rows" hidden={!expanded}>
        {rows.map((row) => {
          const isExisting = Boolean(row.originalName)
          const existingLocked = lockedByUsage && isExisting
          const requiredLocked = lockedByUsage && (!isExisting || row.originalRequired === false || row.originalDeprecated)
          const identityLockTitle = existingLocked ? 'Existing dimension identity is locked after usage exists' : undefined
          const requiredLockTitle = requiredLocked ? 'New dimensions and previously optional dimensions cannot become required after usage exists' : undefined
          const nameError = metadataDimensionNameError(row.name)
          const nameErrorId = `${row.id}-dimension-name-error`
          return (
            <div className="schema-row" key={row.id}>
              <Label className="grid gap-1.5">
                Name
                <Input
                  aria-label="Dimension name"
                  aria-describedby={nameError ? nameErrorId : undefined}
                  aria-invalid={nameError ? 'true' : undefined}
                  disabled={existingLocked}
                  onChange={(event) => onUpdate(row.id, { name: event.currentTarget.value })}
                  placeholder="region"
                  title={identityLockTitle}
                  value={row.name}
                />
                {nameError ? <small className="schema-row-error" id={nameErrorId}>{nameError}</small> : null}
              </Label>
              <Label className="grid gap-1.5">
                Display
                <Input
                  aria-label="Dimension display name"
                  onChange={(event) => onUpdate(row.id, { displayName: event.currentTarget.value })}
                  placeholder="Region"
                  value={row.displayName}
                />
              </Label>
              <Label className="grid gap-1.5">
                Type
                <Select
                  disabled={existingLocked}
                  onValueChange={(value) => onUpdate(row.id, { type: value })}
                  value={row.type}
                >
                  <SelectTrigger aria-label="Dimension type" className="min-h-[38px] w-full" title={identityLockTitle}>
                    <SelectValue placeholder="Select type" />
                  </SelectTrigger>
                  <SelectContent position="popper">
                    {metadataTypes.map((type) => <SelectItem key={type} value={type}>{type}</SelectItem>)}
                  </SelectContent>
                </Select>
              </Label>
              <Label className="schema-required">
                <Checkbox
                  checked={row.required}
                  disabled={requiredLocked}
                  onCheckedChange={(checked) => onUpdate(row.id, { required: checked === true })}
                  title={requiredLockTitle}
                />
                Required
              </Label>
              {showDeprecated ? (
                <Label className="schema-required schema-deprecated">
                  <Checkbox
                    checked={row.deprecated}
                    onCheckedChange={(checked) => onUpdate(row.id, { deprecated: checked === true })}
                  />
                  Deprecated
                </Label>
              ) : null}
              <Button
                aria-label={`Remove ${row.name || 'dimension'}`}
                disabled={existingLocked}
                onClick={() => onRemove(row.id)}
                size="icon"
                title={identityLockTitle}
                type="button"
                variant="ghost"
              >
                <Trash2 aria-hidden="true" />
              </Button>
              <Label className="schema-description grid gap-1.5">
                Description
                <Input
                  aria-label="Dimension description"
                  onChange={(event) => onUpdate(row.id, { description: event.currentTarget.value })}
                  placeholder="Deployment region"
                  value={row.description}
                />
              </Label>
            </div>
          )
        })}
      </div>
    </div>
  )
}

function DimensionChips({ meter }: { meter: Meter }) {
  const dimensions = normalizedMeterDimensions(meter)
  if (dimensions.length === 0) {
    return <span className="muted">No dimensions</span>
  }

  return (
    <div className="schema-chips">
      {dimensions.map((dimension) => (
        <span className="schema-chip" key={dimension.name}>
          <span>{dimension.display_name || humanizeField(dimension.name)}</span>
          <strong>{dimension.deprecated ? `${dimension.type} deprecated` : dimension.required ? dimension.type : `${dimension.type} optional`}</strong>
        </span>
      ))}
    </div>
  )
}

function normalizedMeterDimensions(meter: Meter): MeterDimension[] {
  return meter.dimensions || []
}

function humanizeField(key: string) {
  return key
    .replace(/^metadata\./, '')
    .split(/[._-]/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ')
}
