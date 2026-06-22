import { Eye, Loader2, Plus, Trash2 } from 'lucide-react'
import { type FormEvent, useEffect, useRef, useState } from 'react'

import { appStoreActions } from '../app-store'
import { DataTable, Modal } from '../components/dashboard'
import { EntitlementEventType } from '../components/entitlement-event-detail'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../components/ui/select'
import type { EntitlementEvent, EntitlementPeriodSnapshot, EntitlementState, Meter, Plan, PlanAssignment, PlanLimit, PlanPreview, PlanSaveRequest, SubjectPlanProgress } from '../api'
import { formatDate, formatNumber } from '../lib/format'

const periodOptions = [
  { value: 'day', label: 'Day' },
  { value: 'week', label: 'Week' },
  { value: 'month', label: 'Month' },
  { value: 'year', label: 'Year' },
]

type LimitDraft = {
  id: string
  meter: string
  period: string
  limit: string
  warningPercent: string
}

export function PlanModal({ meters, onClose, onPreview, onSubmit, plan, previewError, previewing, saving, title }: { meters: Meter[]; onClose: () => void; onPreview?: (input: PlanSaveRequest) => Promise<void>; onSubmit: (input: PlanSaveRequest) => Promise<void>; plan?: Plan; previewError?: string; previewing?: boolean; saving: boolean; title: string }) {
  const [limits, setLimits] = useState<LimitDraft[]>(() => draftLimits(plan?.limits, meters))
  const formRef = useRef<HTMLFormElement | null>(null)

  useEffect(() => {
    setLimits(draftLimits(plan?.limits, meters))
  }, [plan, meters])

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await onSubmit(planInputFromForm(event.currentTarget, limits))
  }

  async function preview() {
    if (!formRef.current || !onPreview) {
      return
    }
    await onPreview(planInputFromForm(formRef.current, limits))
  }

  function updateLimit(id: string, update: Partial<LimitDraft>) {
    setLimits((current) => current.map((limit) => limit.id === id ? { ...limit, ...update } : limit))
  }

  function addLimit() {
    setLimits((current) => [...current, emptyLimitDraft(meters)])
  }

  function removeLimit(id: string) {
    setLimits((current) => current.length <= 1 ? current : current.filter((limit) => limit.id !== id))
  }

  return (
    <Modal className="!w-full !max-w-[780px]" title={title} onClose={onClose}>
      <form ref={formRef} className="grid max-h-[calc(100vh-128px)] min-w-0 grid-cols-2 gap-2.5 overflow-auto p-4 max-md:grid-cols-1" onSubmit={(event) => void submit(event)}>
        <Label className="grid min-w-0 gap-1.5">
          Name
          <Input defaultValue={plan?.name || ''} name="name" placeholder="Pro" required />
        </Label>
        <Label className="col-span-full grid min-w-0 gap-1.5">
          Description
          <Input defaultValue={plan?.description || ''} name="description" placeholder="Higher monthly quota for growing teams" />
        </Label>

        <div className="col-span-full grid gap-2 rounded-md border border-border bg-[#f8fafc] p-2.5">
          <div className="flex items-center justify-between gap-2.5">
            <span className="field-label">Limits</span>
            <Button onClick={addLimit} size="sm" type="button" variant="outline">
              <Plus aria-hidden="true" />
              Add limit
            </Button>
          </div>
          {limits.map((limit) => (
            <div className="grid min-w-0 grid-cols-[minmax(0,1fr)_36px] items-end gap-2 rounded-md border border-[#e7ebf1] bg-white p-2 lg:grid-cols-[minmax(160px,1fr)_minmax(120px,140px)_minmax(120px,140px)_minmax(105px,125px)_36px]" key={limit.id}>
              <Label className="col-span-full grid min-w-0 gap-1.5 lg:col-span-1">
                Meter
                <Select onValueChange={(value) => updateLimit(limit.id, { meter: value })} required value={limit.meter || undefined}>
                  <SelectTrigger className="min-h-[38px] w-full">
                    <SelectValue placeholder="Select meter" />
                  </SelectTrigger>
                  <SelectContent position="popper">
                    {meters.map((meter) => <SelectItem key={meter.id} value={meter.name}>{meter.name}</SelectItem>)}
                  </SelectContent>
                </Select>
              </Label>
              <Label className="col-span-full grid min-w-0 gap-1.5 lg:col-span-1">
                Period
                <Select onValueChange={(value) => updateLimit(limit.id, { period: value })} value={limit.period}>
                  <SelectTrigger className="min-h-[38px] w-full">
                    <SelectValue placeholder="Select period" />
                  </SelectTrigger>
                  <SelectContent position="popper">
                    {periodOptions.map((period) => <SelectItem key={period.value} value={period.value}>{period.label}</SelectItem>)}
                  </SelectContent>
                </Select>
              </Label>
              <Label className="col-span-full grid min-w-0 gap-1.5 lg:col-span-1">
                Limit
                <Input min="0" required step="any" type="number" value={limit.limit} onChange={(event) => updateLimit(limit.id, { limit: event.currentTarget.value })} />
              </Label>
              <Label className="col-span-full grid min-w-0 gap-1.5 lg:col-span-1">
                Warn %
                <Input min="1" max="100" step="any" type="number" value={limit.warningPercent} onChange={(event) => updateLimit(limit.id, { warningPercent: event.currentTarget.value })} />
              </Label>
              <Button aria-label="Remove limit" className="col-start-2 self-end lg:col-start-auto" disabled={limits.length <= 1} onClick={() => removeLimit(limit.id)} size="icon" type="button" variant="ghost">
                <Trash2 aria-hidden="true" />
              </Button>
            </div>
          ))}
        </div>

        {previewError ? <div className="error-banner col-span-full !mb-0">{previewError}</div> : null}

        <div className="col-span-full flex justify-end gap-2.5 border-t border-border pt-4">
          <Button onClick={onClose} type="button" variant="outline">Cancel</Button>
          {onPreview ? (
            <Button disabled={previewing || saving || meters.length === 0} onClick={() => void preview()} type="button" variant="outline">
              {previewing ? <Loader2 className="spin" aria-hidden="true" /> : null}
              Preview changes
            </Button>
          ) : null}
          <Button disabled={saving || meters.length === 0} type="submit">
            {saving ? <Loader2 className="spin" aria-hidden="true" /> : null}
            Save
          </Button>
        </div>
      </form>
    </Modal>
  )
}

export function PlanPreviewModal({ onClose, preview }: { onClose: () => void; preview: PlanPreview }) {
  const summaryItems = [
    { label: 'Subjects', value: preview.summary.subjects, variant: 'muted' as const },
    { label: 'OK', value: preview.summary.ok, variant: 'success' as const },
    { label: 'Warning', value: preview.summary.warning, variant: 'warning' as const },
    { label: 'Exceeded', value: preview.summary.exceeded, variant: 'warning' as const },
  ]

  return (
    <Modal className="!w-full !max-w-[640px]" title="Plan Change Impact" onClose={onClose}>
      <div className="grid gap-4 p-4">
        <div className="grid gap-2">
          <p className="text-sm text-muted">
            Projected impact across current and scheduled subjects. Assignments are not changed by this preview.
          </p>
          <div className="flex flex-wrap items-center gap-2 text-xs text-muted">
            <Badge variant="muted">{preview.current.name} v{preview.current.version}</Badge>
            <span>to</span>
            <Badge variant="muted">{preview.proposed.name} v{preview.proposed.version}</Badge>
          </div>
        </div>

        <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
          {summaryItems.map((item) => (
            <div className="grid gap-1 rounded-md border border-border bg-[#f8fafc] p-3" key={item.label}>
              <span className="text-xs font-semibold uppercase text-muted">{item.label}</span>
              <strong className="text-2xl leading-none">{formatNumber(item.value)}</strong>
              <Badge className="w-fit" variant={item.variant}>{item.label.toLowerCase()}</Badge>
            </div>
          ))}
        </div>

        {preview.summary.removed_limits > 0 ? (
          <div className="rounded-md border border-warning/30 bg-warning/10 p-3 text-sm">
            <strong>{formatNumber(preview.summary.removed_limits)} removed limit impacts.</strong>
            <span className="ml-1 text-muted">Some assigned subjects currently use meters that this plan version no longer limits.</span>
          </div>
        ) : null}

        <div className="modal-actions">
          <Button onClick={onClose} type="button" variant="outline">Close</Button>
        </div>
      </div>
    </Modal>
  )
}

export function DeletePlanModal({ onConfirm, plan, saving }: { onConfirm: () => Promise<void>; plan: Plan; saving: boolean }) {
  return (
    <Modal title="Delete Plan" onClose={() => appStoreActions.setPlanDeleting(null)}>
      <div className="modal-body">
        <p>Delete <strong>{plan.name}</strong>? Assigned subjects must be removed before a plan can be deleted.</p>
        <div className="modal-actions">
          <Button onClick={() => appStoreActions.setPlanDeleting(null)} type="button" variant="outline">Cancel</Button>
          <Button disabled={saving} onClick={() => void onConfirm()} type="button">Delete</Button>
        </div>
      </div>
    </Modal>
  )
}

export function LimitChips({ limits }: { limits: PlanLimit[] }) {
  if (limits.length === 0) {
    return <span className="muted">No limits</span>
  }
  return (
    <span className="flex max-w-[520px] flex-wrap gap-1.5">
      {limits.map((limit) => (
        <Badge className="inline-flex gap-1.5" key={limit.id} variant="muted">
          <strong>{limit.meter}</strong>
          <span>{formatNumber(limit.limit)} / {limit.period}</span>
        </Badge>
      ))}
    </span>
  )
}

export function AssignmentTable({ assigning, assignments, onProgress, progressStatus }: { assigning: boolean; assignments: PlanAssignment[]; onProgress: (subject: string) => Promise<void> | void; progressStatus: string }) {
  return (
    <DataTable
      emptyLabel="No assignments yet"
      headers={['Subject', 'Plan', 'Status', 'Effective', 'Actions']}
      rows={assignments.map((assignment) => [
        <span className="mono">{assignment.subject}</span>,
        <span className="flex items-center gap-2">
          {assignment.plan_name}
          <Badge variant="muted">v{assignment.plan_version}</Badge>
        </span>,
        <AssignmentStatusBadge assignment={assignment} />,
        formatDate(assignment.assigned_at),
        <span className="table-actions">
          <Button
            aria-label={`View ${assignment.subject} progress`}
            disabled={progressStatus === 'loading'}
            onClick={() => void onProgress(assignment.subject)}
            size="sm"
            type="button"
            variant="outline"
          >
            Progress
          </Button>
          <Button
            aria-label={`Remove ${assignment.subject} assignment`}
            disabled={assigning}
            onClick={() => void appStoreActions.deleteSubjectPlanAssignment(assignment.subject)}
            size="icon"
            type="button"
            variant="ghost"
          >
            <Trash2 aria-hidden="true" />
          </Button>
        </span>,
      ])}
    />
  )
}

export function ProgressModal({ onClose, progress }: { onClose: () => void; progress: SubjectPlanProgress }) {
  return (
    <Modal className="!w-full !max-w-[760px]" title="Usage Progress" onClose={onClose}>
      <div className="grid gap-4 p-4">
        <ProgressList progress={progress} />
        <div className="modal-actions">
          <Button onClick={onClose} type="button" variant="outline">Close</Button>
        </div>
      </div>
    </Modal>
  )
}

export function ProgressList({ progress }: { progress: SubjectPlanProgress }) {
  return (
    <div className="grid gap-3">
      <div className="flex justify-between gap-3 text-xs text-muted">
        <div className="flex min-w-0 items-center gap-2">
          <strong>{progress.subject}</strong>
          <small>{progress.plan.name}</small>
        </div>
      </div>
      {progress.items.map((item) => (
        <div className="grid gap-2 rounded-md border border-border bg-[#f8fafc] p-3" key={`${item.meter}-${item.period}`}>
          <div className="flex items-center justify-between gap-2">
            <strong>{item.meter}</strong>
            <StateBadge state={item.state} />
          </div>
          <div className="h-2 overflow-hidden rounded-full bg-[#e2e8f0]" aria-label={`${item.meter} quota progress`}>
            <span className="block h-full rounded-[inherit] bg-primary" style={{ width: `${Math.min(item.percent, 100)}%` }} />
          </div>
          <small className="text-xs text-muted">
            {formatNumber(item.current)} / {formatNumber(item.limit)} {item.unit} this {item.period}
            {item.remaining > 0 ? `, ${formatNumber(item.remaining)} remaining` : ''}
            {item.overage > 0 ? `, ${formatNumber(item.overage)} over` : ''}
          </small>
          <small className="text-xs text-muted">
            Current period: {formatPeriodRange(item.from, item.to)}
            {item.period_reset_at ? ` - resets ${formatDate(item.period_reset_at)}` : ''}
          </small>
        </div>
      ))}
    </div>
  )
}

export function EntitlementStateTable({ states }: { states: EntitlementState[] }) {
  return (
    <DataTable
      emptyLabel="No entitlement states yet"
      headers={['Subject', 'Meter', 'Plan', 'Usage', 'Remaining', 'State', 'Updated']}
      rows={states.map((state) => [
        <span className="mono">{state.subject}</span>,
        <Badge variant="muted">{state.meter}</Badge>,
        state.plan_name,
        <span>{formatNumber(state.current)} / {formatNumber(state.limit)}</span>,
        state.remaining > 0 ? formatNumber(state.remaining) : <span className="muted">0</span>,
        <StateBadge state={state.state} />,
        formatDate(state.updated_at),
      ])}
    />
  )
}

export function PeriodSnapshotTable({ snapshots }: { snapshots: EntitlementPeriodSnapshot[] }) {
  return (
    <DataTable
      emptyLabel="No period snapshots yet"
      headers={['Subject', 'Plan', 'Meter', 'Period', 'Usage', 'Included', 'Overage', 'State', 'Updated']}
      rows={snapshots.map((snapshot) => [
        <span className="mono">{snapshot.subject}</span>,
        <span className="flex items-center gap-2">
          {snapshot.plan_name}
          <Badge variant="muted">v{snapshot.plan_version}</Badge>
        </span>,
        <Badge variant="muted">{snapshot.meter}</Badge>,
        <span className="grid gap-0.5">
          <strong>{titleCase(snapshot.period)}</strong>
          <small className="text-xs text-muted">{formatPeriodRange(snapshot.from, snapshot.to)}</small>
        </span>,
        <span>{formatNumber(snapshot.current)} / {formatNumber(snapshot.limit)}</span>,
        formatNumber(snapshot.included),
        snapshot.overage > 0 ? <strong>{formatNumber(snapshot.overage)}</strong> : <span className="muted">0</span>,
        <StateBadge state={snapshot.state} />,
        formatDate(snapshot.updated_at),
      ])}
    />
  )
}

export function AssignmentHistoryTable({ assignments }: { assignments: PlanAssignment[] }) {
  return (
    <DataTable
      emptyLabel="No assignment history yet"
      headers={['Subject', 'Plan', 'Status', 'Effective', 'Anchor', 'Ended']}
      rows={assignments.map((assignment) => [
        <span className="mono">{assignment.subject}</span>,
        <span className="flex items-center gap-2">
          {assignment.plan_name}
          <Badge variant="muted">v{assignment.plan_version}</Badge>
        </span>,
        <AssignmentStatusBadge assignment={assignment} />,
        formatDate(assignment.assigned_at),
        formatDate(assignment.period_anchor_at),
        assignment.unassigned_at ? formatDate(assignment.unassigned_at) : <span className="muted">-</span>,
      ])}
    />
  )
}

export function EntitlementEventTable({ events, loading, onSelect }: { events: EntitlementEvent[]; loading: boolean; onSelect: (event: EntitlementEvent) => void }) {
  return (
    <DataTable
      emptyLabel={loading ? 'Loading entitlement changes' : 'No entitlement changes yet'}
      headers={['Type', 'Subject', 'Meter', 'Usage', 'Message', 'Created', 'Actions']}
      rows={events.map((event) => [
        <EntitlementEventType event={event} />,
        <span className="mono">{event.subject}</span>,
        <Badge variant="muted">{event.meter}</Badge>,
        <span>{formatNumber(event.current)} / {formatNumber(event.limit)}</span>,
        <span className="max-w-[320px] truncate">{event.message}</span>,
        formatDate(event.created_at),
        <span className="table-actions">
          <Button aria-label={`View ${event.type} entitlement change`} onClick={() => onSelect(event)} size="icon" type="button" variant="ghost">
            <Eye aria-hidden="true" />
          </Button>
        </span>,
      ])}
    />
  )
}

export function countAssignmentsByPlan(assignments: PlanAssignment[]) {
  const counts = new Map<string, number>()
  for (const assignment of assignments) {
    counts.set(assignment.plan_id, (counts.get(assignment.plan_id) ?? 0) + 1)
  }
  return counts
}

function StateBadge({ state }: { state: string }) {
  if (state === 'exceeded') {
    return <Badge variant="warning">Exceeded</Badge>
  }
  if (state === 'warning') {
    return <Badge variant="warning">Warning</Badge>
  }
  if (state === 'not_in_plan') {
    return <Badge variant="warning">Not in plan</Badge>
  }
  if (state === 'no_plan') {
    return <Badge variant="warning">No plan</Badge>
  }
  return <Badge variant="success">OK</Badge>
}

function AssignmentStatusBadge({ assignment }: { assignment: PlanAssignment }) {
  if (assignment.status === 'scheduled') {
    return <Badge variant="warning">Scheduled</Badge>
  }
  if (assignment.status === 'active' || assignment.active) {
    return <Badge variant="success">Active</Badge>
  }
  return <Badge variant="muted">Ended</Badge>
}

function draftLimits(limits: PlanLimit[] | undefined, meters: Meter[]): LimitDraft[] {
  if (!limits || limits.length === 0) {
    return [emptyLimitDraft(meters)]
  }
  return limits.map((limit) => ({
    id: limit.id || draftID(),
    limit: String(limit.limit),
    meter: limit.meter,
    period: limit.period || 'month',
    warningPercent: String(limit.warning_percent || 80),
  }))
}

function emptyLimitDraft(meters: Meter[]): LimitDraft {
  return {
    id: draftID(),
    limit: '',
    meter: meters[0]?.name || '',
    period: 'month',
    warningPercent: '80',
  }
}

function draftID() {
  return `limit_${Date.now()}_${Math.random().toString(16).slice(2)}`
}

function planInputFromForm(form: HTMLFormElement, limits: LimitDraft[]): PlanSaveRequest {
  const data = new FormData(form)
  return {
    description: String(data.get('description') || ''),
    limits: limits.map((limit) => ({
      limit: Number(limit.limit),
      meter: limit.meter,
      period: limit.period,
      warning_percent: limit.warningPercent ? Number(limit.warningPercent) : undefined,
    })),
    name: String(data.get('name') || ''),
  }
}

function formatPeriodRange(from: string, to: string) {
  return `${formatDate(from)} - ${formatDate(to)}`
}

function titleCase(value: string) {
  return value ? value.slice(0, 1).toUpperCase() + value.slice(1) : value
}
