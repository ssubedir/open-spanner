import { useSelector } from '@tanstack/react-store'
import { GaugeCircle, Loader2, PackageCheck, Pencil, Plus, Trash2 } from 'lucide-react'
import { type FormEvent, useCallback, useEffect, useState } from 'react'

import { appStore, appStoreActions } from '../app-store'
import { DataTable, Modal, PageHeader } from '../components/dashboard'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import type { Meter, Plan, PlanLimit, PlanSaveRequest, SubjectPlanProgress } from '../api'
import { formatDate, formatNumber } from '../lib/format'
import { useInitialLoad } from '../lib/hooks'

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

export function PlansPage() {
  const { assigning, assignments, creating, deleting, editing, error, items, meters, progress, progressStatus, progressSubject, saving } = useSelector(appStore, (state) => state.plans)
  const [assignOpen, setAssignOpen] = useState(false)
  const [progressOpen, setProgressOpen] = useState(false)
  const load = useCallback(() => appStoreActions.loadPlans(), [])

  useInitialLoad(load)

  async function submitAssignment(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    try {
      await appStoreActions.assignSubjectPlan(
        String(form.get('subject') || ''),
        String(form.get('plan_id') || ''),
      )
      setAssignOpen(false)
    } catch {
      // Store owns the visible error state.
    }
  }

  async function submitProgress(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    await appStoreActions.loadSubjectPlanProgress(String(form.get('subject') || ''))
    setProgressOpen(false)
  }

  async function submitCreate(input: PlanSaveRequest) {
    await appStoreActions.createPlan(input)
  }

  async function submitUpdate(input: PlanSaveRequest) {
    await appStoreActions.updateEditingPlan(input)
  }

  async function confirmDelete() {
    try {
      await appStoreActions.deleteSelectedPlan()
    } catch {
      // Store owns the visible error state.
    }
  }

  return (
    <>
      <PageHeader
        eyebrow="Plans"
        icon={<PackageCheck />}
        title="Plans and entitlements"
        description="Define quota packages, assign subjects, and check current usage against limits."
        action={(
          <Button disabled={saving || meters.length === 0} onClick={() => appStoreActions.setPlanCreating(true)} type="button">
            <Plus aria-hidden="true" />
            New plan
          </Button>
        )}
      />

      {error ? <div className="error-banner">{error}</div> : null}

      {meters.length === 0 ? (
        <div className="info-banner">Create a meter before defining plan limits.</div>
      ) : null}

      <div className="plans-layout">
        <Card className="plans-table-card">
          <CardHeader className="api-key-card-header">
            <div>
              <CardTitle>Plans</CardTitle>
              <CardDescription>Named packages with per-meter quota limits.</CardDescription>
            </div>
          </CardHeader>
          <CardContent>
            <DataTable
              emptyLabel="No plans yet"
              headers={['Plan', 'Limits', 'Updated', 'Actions']}
              rows={items.map((plan) => [
                <span className="stack-cell">
                  <strong>{plan.name}</strong>
                  {plan.description ? <small>{plan.description}</small> : null}
                </span>,
                <LimitChips limits={plan.limits} />,
                formatDate(plan.updated_at),
                <span className="table-actions">
                  <Button aria-label={`Edit ${plan.name}`} disabled={saving} onClick={() => appStoreActions.setPlanEditing(plan)} size="icon" type="button" variant="ghost">
                    <Pencil aria-hidden="true" />
                  </Button>
                  <Button aria-label={`Delete ${plan.name}`} disabled={saving} onClick={() => appStoreActions.setPlanDeleting(plan)} size="icon" type="button" variant="ghost">
                    <Trash2 aria-hidden="true" />
                  </Button>
                </span>,
              ])}
            />
          </CardContent>
        </Card>

        <aside className="plans-side-column">
          <Card className="plan-subject-panel-card">
            <CardHeader>
              <CardTitle>Subject entitlements</CardTitle>
              <CardDescription>Assign a subject to a plan, then inspect current quota progress.</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="plan-subject-actions">
                <Button disabled={assigning || items.length === 0} onClick={() => setAssignOpen(true)} type="button">
                  <Plus aria-hidden="true" />
                  Assign subject
                </Button>
                <Button disabled={progressStatus === 'loading'} onClick={() => setProgressOpen(true)} type="button" variant="outline">
                  {progressStatus === 'loading' ? <Loader2 className="spin" aria-hidden="true" /> : <GaugeCircle aria-hidden="true" />}
                  Check progress
                </Button>
              </div>
            </CardContent>
          </Card>

          {progress ? (
            <Card className="plan-progress-result-card">
              <CardHeader>
                <CardTitle>Usage Progress</CardTitle>
                <CardDescription>Current window usage against assigned plan limits.</CardDescription>
              </CardHeader>
              <CardContent>
                <ProgressList progress={progress} />
              </CardContent>
            </Card>
          ) : null}
        </aside>

        <Card className="plan-assignment-table-card">
          <CardHeader className="api-key-card-header">
            <div>
              <CardTitle>Assignments</CardTitle>
              <CardDescription>Subjects currently tied to a plan.</CardDescription>
            </div>
          </CardHeader>
          <CardContent>
            <DataTable
              emptyLabel="No assignments yet"
              headers={['Subject', 'Plan', 'Updated', 'Actions']}
              rows={assignments.map((assignment) => [
                <span className="mono">{assignment.subject}</span>,
                assignment.plan_name,
                formatDate(assignment.updated_at),
                <span className="table-actions">
                  <Button
                    aria-label={`View ${assignment.subject} progress`}
                    disabled={progressStatus === 'loading'}
                    onClick={() => void appStoreActions.loadSubjectPlanProgress(assignment.subject)}
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
          </CardContent>
        </Card>
      </div>

      {creating ? (
        <PlanModal
          meters={meters}
          onClose={() => appStoreActions.setPlanCreating(false)}
          onSubmit={submitCreate}
          saving={saving}
          title="Create Plan"
        />
      ) : null}

      {assignOpen ? (
        <Modal className="plan-small-modal" title="Assign Subject" onClose={() => setAssignOpen(false)}>
          <form className="modal-form plan-single-column-form" onSubmit={(event) => void submitAssignment(event)}>
            <label>
              Subject
              <input name="subject" placeholder="org_123" required />
            </label>
            <label>
              Plan
              <select name="plan_id" required>
                <option value="">Select plan</option>
                {items.map((plan) => <option key={plan.id} value={plan.id}>{plan.name}</option>)}
              </select>
            </label>
            <div className="modal-actions">
              <Button onClick={() => setAssignOpen(false)} type="button" variant="outline">Cancel</Button>
              <Button disabled={assigning || items.length === 0} type="submit">
                {assigning ? <Loader2 className="spin" aria-hidden="true" /> : <Plus aria-hidden="true" />}
                Assign
              </Button>
            </div>
          </form>
        </Modal>
      ) : null}

      {progressOpen ? (
        <Modal className="plan-small-modal" title="Check Usage Progress" onClose={() => setProgressOpen(false)}>
          <form className="modal-form plan-single-column-form" onSubmit={(event) => void submitProgress(event)}>
            <label>
              Subject
              <input
                name="subject"
                onChange={(event) => appStoreActions.setPlanProgressSubject(event.currentTarget.value)}
                placeholder="org_123"
                value={progressSubject}
              />
            </label>
            <div className="modal-actions">
              <Button onClick={() => setProgressOpen(false)} type="button" variant="outline">Cancel</Button>
              <Button disabled={progressStatus === 'loading'} type="submit">
                {progressStatus === 'loading' ? <Loader2 className="spin" aria-hidden="true" /> : <GaugeCircle aria-hidden="true" />}
                Check
              </Button>
            </div>
          </form>
        </Modal>
      ) : null}

      {editing ? (
        <PlanModal
          meters={meters}
          onClose={() => appStoreActions.setPlanEditing(null)}
          onSubmit={submitUpdate}
          plan={editing}
          saving={saving}
          title="Edit Plan"
        />
      ) : null}

      {deleting ? (
        <Modal title="Delete Plan" onClose={() => appStoreActions.setPlanDeleting(null)}>
          <div className="modal-body">
            <p>Delete <strong>{deleting.name}</strong>? Assigned subjects must be removed before a plan can be deleted.</p>
            <div className="modal-actions">
              <Button onClick={() => appStoreActions.setPlanDeleting(null)} type="button" variant="outline">Cancel</Button>
              <Button disabled={saving} onClick={() => void confirmDelete()} type="button">Delete</Button>
            </div>
          </div>
        </Modal>
      ) : null}
    </>
  )
}

function PlanModal({ meters, onClose, onSubmit, plan, saving, title }: { meters: Meter[]; onClose: () => void; onSubmit: (input: PlanSaveRequest) => Promise<void>; plan?: Plan; saving: boolean; title: string }) {
  const [limits, setLimits] = useState<LimitDraft[]>(() => draftLimits(plan?.limits, meters))

  useEffect(() => {
    setLimits(draftLimits(plan?.limits, meters))
  }, [plan, meters])

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    await onSubmit({
      description: String(form.get('description') || ''),
      limits: limits.map((limit) => ({
        limit: Number(limit.limit),
        meter: limit.meter,
        period: limit.period,
        warning_percent: limit.warningPercent ? Number(limit.warningPercent) : undefined,
      })),
      name: String(form.get('name') || ''),
    })
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
    <Modal className="plan-modal" title={title} onClose={onClose}>
      <form className="modal-form plan-modal-form" onSubmit={(event) => void submit(event)}>
        <label>
          Name
          <input defaultValue={plan?.name || ''} name="name" placeholder="Pro" required />
        </label>
        <label>
          Description
          <input defaultValue={plan?.description || ''} name="description" placeholder="Higher monthly quota for growing teams" />
        </label>

        <div className="plan-limit-editor">
          <div className="plan-limit-editor-header">
            <span className="field-label">Limits</span>
            <Button onClick={addLimit} size="sm" type="button" variant="outline">
              <Plus aria-hidden="true" />
              Add limit
            </Button>
          </div>
          {limits.map((limit) => (
            <div className="plan-limit-row" key={limit.id}>
              <label>
                Meter
                <select required value={limit.meter} onChange={(event) => updateLimit(limit.id, { meter: event.currentTarget.value })}>
                  <option value="">Select meter</option>
                  {meters.map((meter) => <option key={meter.id} value={meter.name}>{meter.name}</option>)}
                </select>
              </label>
              <label>
                Period
                <select value={limit.period} onChange={(event) => updateLimit(limit.id, { period: event.currentTarget.value })}>
                  {periodOptions.map((period) => <option key={period.value} value={period.value}>{period.label}</option>)}
                </select>
              </label>
              <label>
                Limit
                <input min="0" required step="any" type="number" value={limit.limit} onChange={(event) => updateLimit(limit.id, { limit: event.currentTarget.value })} />
              </label>
              <label>
                Warn %
                <input min="1" max="100" step="any" type="number" value={limit.warningPercent} onChange={(event) => updateLimit(limit.id, { warningPercent: event.currentTarget.value })} />
              </label>
              <Button aria-label="Remove limit" disabled={limits.length <= 1} onClick={() => removeLimit(limit.id)} size="icon" type="button" variant="ghost">
                <Trash2 aria-hidden="true" />
              </Button>
            </div>
          ))}
        </div>

        <div className="modal-actions">
          <Button onClick={onClose} type="button" variant="outline">Cancel</Button>
          <Button disabled={saving || meters.length === 0} type="submit">
            {saving ? <Loader2 className="spin" aria-hidden="true" /> : null}
            Save
          </Button>
        </div>
      </form>
    </Modal>
  )
}

function LimitChips({ limits }: { limits: PlanLimit[] }) {
  if (limits.length === 0) {
    return <span className="muted">No limits</span>
  }
  return (
    <span className="plan-limit-chips">
      {limits.map((limit) => (
        <Badge key={limit.id} variant="muted">
          <strong>{limit.meter}</strong>
          <span>{formatNumber(limit.limit)} / {limit.period}</span>
        </Badge>
      ))}
    </span>
  )
}

function ProgressList({ progress }: { progress: SubjectPlanProgress }) {
  return (
    <div className="plan-progress-list">
      <div className="plan-progress-heading">
        <div>
          <strong>{progress.subject}</strong>
          <small>{progress.plan.name}</small>
        </div>
      </div>
      {progress.items.map((item) => (
        <div className="plan-progress-item" key={`${item.meter}-${item.period}`}>
          <div>
            <strong>{item.meter}</strong>
            <StateBadge state={item.state} />
          </div>
          <div className="quota-bar" aria-label={`${item.meter} quota progress`}>
            <span style={{ width: `${Math.min(item.percent, 100)}%` }} />
          </div>
          <small>
            {formatNumber(item.current)} / {formatNumber(item.limit)} {item.unit} this {item.period}
            {item.remaining > 0 ? `, ${formatNumber(item.remaining)} remaining` : ''}
          </small>
        </div>
      ))}
    </div>
  )
}

function StateBadge({ state }: { state: string }) {
  if (state === 'exceeded') {
    return <Badge variant="warning">Exceeded</Badge>
  }
  if (state === 'warning') {
    return <Badge variant="warning">Warning</Badge>
  }
  return <Badge variant="success">OK</Badge>
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
