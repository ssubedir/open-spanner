import { useRouter } from '@tanstack/react-router'
import { useSelector } from '@tanstack/react-store'
import { ArrowLeft, GaugeCircle, Loader2, PackageCheck, Pencil, Plus, Trash2 } from 'lucide-react'
import { type FormEvent, useCallback, useEffect, useState } from 'react'

import { appStore, appStoreActions } from '../app-store'
import { DetailLoadingPage, DetailStatePage, Modal, PageHeader } from '../components/dashboard'
import { EntitlementEventDetail } from '../components/entitlement-event-detail'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../components/ui/select'
import { previewPlan as previewPlanRequest, type PlanPreview, type PlanSaveRequest } from '../api'
import { useInitialLoad } from '../lib/hooks'
import {
  AssignmentHistoryTable,
  AssignmentTable,
  DeletePlanModal,
  EntitlementEventTable,
  EntitlementStateTable,
  LimitChips,
  PeriodSnapshotTable,
  PlanModal,
  PlanPreviewModal,
  ProgressModal,
} from './PlanPageParts'

export function PlanDetailPage({ planId }: { planId: string }) {
  const router = useRouter()
  const {
    assigning,
    assignments,
    assignmentHistory,
    deleting,
    editing,
    entitlementEventLoadingMore,
    entitlementEventNextCursor,
    entitlementEventStatus,
    entitlementEvents,
    entitlementPeriodSnapshots,
    entitlementStates,
    error,
    items,
    meters,
    progress,
    progressStatus,
    progressSubject,
    saving,
    selectedEntitlementEvent,
    status,
  } = useSelector(appStore, (state) => state.plans)
  const [assignOpen, setAssignOpen] = useState(false)
  const [assignmentTiming, setAssignmentTiming] = useState<'now' | 'scheduled'>('now')
  const [previewError, setPreviewError] = useState('')
  const [planPreview, setPlanPreview] = useState<PlanPreview | null>(null)
  const [previewing, setPreviewing] = useState(false)
  const [progressOpen, setProgressOpen] = useState(false)
  const [progressResultOpen, setProgressResultOpen] = useState(false)
  const load = useCallback(() => appStoreActions.loadPlans(), [])
  const pollEntitlementActivity = useCallback(() => appStoreActions.loadPlanEntitlementActivity({ quiet: true }), [])
  const plan = items.find((item) => item.id === planId) ?? null
  const planFamilyName = plan?.name ?? ''
  const planAssignments = assignments.filter((assignment) => assignment.plan_id === planId)
  const planAssignmentHistory = assignmentHistory.filter((assignment) => assignment.plan_id === planId || (planFamilyName !== '' && assignment.plan_name === planFamilyName))
  const planStates = entitlementStates.filter((state) => state.plan_id === planId)
  const planEvents = entitlementEvents.filter((event) => event.plan_id === planId)
  const planSnapshots = entitlementPeriodSnapshots.filter((snapshot) => snapshot.plan_id === planId)
  const visibleProgress = progress?.plan.id === planId ? progress : null

  useInitialLoad(load)

  useEffect(() => {
    const poll = window.setInterval(() => {
      void pollEntitlementActivity()
    }, 5000)

    return () => window.clearInterval(poll)
  }, [pollEntitlementActivity])

  async function submitAssignment(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    const scheduledValue = String(form.get('effective_at') || '')
    const effectiveAt = assignmentTiming === 'scheduled' && scheduledValue ? new Date(scheduledValue).toISOString() : undefined
    try {
      await appStoreActions.assignSubjectPlan(
        String(form.get('subject') || ''),
        planId,
        effectiveAt,
      )
      setAssignOpen(false)
      setAssignmentTiming('now')
    } catch {
      // Store owns the visible error state.
    }
  }

  async function submitProgress(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    await openProgressForSubject(String(form.get('subject') || ''))
    setProgressOpen(false)
  }

  async function openProgressForSubject(subject: string) {
    await appStoreActions.loadSubjectPlanProgress(subject)
    setProgressResultOpen(true)
  }

  async function submitUpdate(input: PlanSaveRequest) {
    const updatedPlan = await appStoreActions.updateEditingPlan(input)
    if (updatedPlan && updatedPlan.id !== planId) {
      void router.navigate({ to: '/plans/$planId', params: { planId: updatedPlan.id } })
    }
  }

  async function previewUpdate(input: PlanSaveRequest) {
    setPreviewError('')
    setPreviewing(true)
    try {
      const preview = await previewPlanRequest(planId, input)
      setPlanPreview(preview)
    } catch (err) {
      setPreviewError(errorMessage(err, 'Could not preview plan changes'))
    } finally {
      setPreviewing(false)
    }
  }

  async function confirmDelete() {
    try {
      await appStoreActions.deleteSelectedPlan()
      void router.navigate({ to: '/plans' })
    } catch {
      // Store owns the visible error state.
    }
  }

  if (!plan && status !== 'ready' && status !== 'error') {
    return (
      <DetailLoadingPage
        eyebrow="Plans"
        icon={<PackageCheck />}
        title="Loading plan"
        description="Loading plan details before showing assignments and usage progress."
        action={(
          <Button onClick={() => void router.navigate({ to: '/plans' })} type="button" variant="outline">
            <ArrowLeft aria-hidden="true" />
            Back to plans
          </Button>
        )}
      />
    )
  }

  if (!plan) {
    const loadFailed = status === 'error'
    return (
      <DetailStatePage
        icon={<PackageCheck />}
        title={loadFailed ? 'Could not load plan' : 'Plan not found'}
        description={loadFailed ? 'Try again from the plans list.' : 'This plan may have been deleted or belongs to another workspace.'}
        action={(
          <Button onClick={() => void router.navigate({ to: '/plans' })} type="button" variant="outline">
            <ArrowLeft aria-hidden="true" />
            Back to plans
          </Button>
        )}
      />
    )
  }

  return (
    <>
      <PageHeader
        eyebrow="Plans"
        icon={<PackageCheck />}
        title={plan?.name ?? 'Plan details'}
        description={plan?.description || 'Inspect assignments, period progress, and entitlement changes for this plan.'}
        action={(
          <Button onClick={() => void router.navigate({ to: '/plans' })} type="button" variant="outline">
            <ArrowLeft aria-hidden="true" />
            Back
          </Button>
        )}
      />

      {error ? <div className="error-banner">{error}</div> : null}

      <div className="grid max-w-[1480px] gap-4">
        <section className="grid items-stretch gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(320px,380px)]">
          <Card className="min-w-0">
            <CardHeader className="!px-4 !py-3">
              <div>
                <CardTitle>Limits</CardTitle>
                <CardDescription>Versioned quota limits attached to this plan.</CardDescription>
              </div>
            </CardHeader>
            <CardContent className="grid gap-3 !p-4">
              <div className="flex flex-wrap items-center gap-2 text-xs text-muted">
                <Badge variant={plan.is_current ? 'success' : 'muted'}>{plan.is_current ? 'Current version' : 'Historical version'}</Badge>
                <Badge variant="muted">v{plan.version}</Badge>
                {plan.parent_plan_id ? <span>Created from a previous version.</span> : <span>Original version.</span>}
              </div>
              {plan ? <LimitChips limits={plan.limits} /> : <span className="muted">Loading limits</span>}
            </CardContent>
          </Card>

          <Card className="min-w-0">
            <CardHeader className="!grid !justify-start !gap-1 !px-4 !py-3">
              <CardTitle>Subject tools</CardTitle>
              <CardDescription>Attach subjects or inspect current quota progress.</CardDescription>
            </CardHeader>
            <CardContent className="grid gap-2 !p-3">
              <Button disabled={assigning || !plan} onClick={() => setAssignOpen(true)} type="button">
                <Plus aria-hidden="true" />
                Assign subject
              </Button>
              <Button disabled={progressStatus === 'loading' || !plan} onClick={() => setProgressOpen(true)} type="button" variant="outline">
                {progressStatus === 'loading' ? <Loader2 className="spin" aria-hidden="true" /> : <GaugeCircle aria-hidden="true" />}
                Check progress
              </Button>
              {plan ? (
                <div className="flex gap-2 pt-1">
                  <Button className="flex-1" disabled={saving} onClick={() => appStoreActions.setPlanEditing(plan)} type="button" variant="outline">
                    <Pencil aria-hidden="true" />
                    Edit
                  </Button>
                  <Button className="flex-1" disabled={saving} onClick={() => appStoreActions.setPlanDeleting(plan)} type="button" variant="outline">
                    <Trash2 aria-hidden="true" />
                    Delete
                  </Button>
                </div>
              ) : null}
            </CardContent>
          </Card>
        </section>

        <Card className="min-w-0">
          <CardHeader className="!px-4 !py-3">
            <div>
              <CardTitle>Assignments</CardTitle>
              <CardDescription>Subjects currently tied to this plan.</CardDescription>
            </div>
          </CardHeader>
          <CardContent>
            <AssignmentTable assignments={planAssignments} assigning={assigning} onProgress={openProgressForSubject} progressStatus={progressStatus} />
          </CardContent>
        </Card>

        <Card className="min-w-0">
          <CardHeader className="!px-4 !py-3">
            <div>
              <CardTitle>Period Snapshots</CardTitle>
              <CardDescription>Auditable usage totals for evaluated periods.</CardDescription>
            </div>
          </CardHeader>
          <CardContent>
            <PeriodSnapshotTable snapshots={planSnapshots} />
          </CardContent>
        </Card>

        <div className="grid gap-4 xl:grid-cols-2">
          <Card className="min-w-0">
            <CardHeader className="!px-4 !py-3">
              <div>
                <CardTitle>Current Entitlements</CardTitle>
                <CardDescription>Latest quota state by subject and meter.</CardDescription>
              </div>
            </CardHeader>
            <CardContent>
              <EntitlementStateTable states={planStates} />
            </CardContent>
          </Card>

          <Card className="min-w-0">
            <CardHeader className="!px-4 !py-3">
              <div>
                <CardTitle>Recent Entitlement Changes</CardTitle>
                <CardDescription>Warning, exceeded, and recovered transitions.</CardDescription>
              </div>
            </CardHeader>
            <CardContent>
              <EntitlementEventTable
                events={planEvents}
                loading={entitlementEventStatus === 'loading'}
                onSelect={(event) => appStoreActions.setPlanSelectedEntitlementEvent(event)}
              />
              {entitlementEventNextCursor ? (
                <div className="pagination-actions">
                  <Button disabled={entitlementEventLoadingMore} onClick={() => void appStoreActions.loadMorePlanEntitlementEvents()} type="button" variant="outline">
                    {entitlementEventLoadingMore ? <Loader2 className="spin" aria-hidden="true" /> : null}
                    Load more changes
                  </Button>
                </div>
              ) : null}
            </CardContent>
          </Card>
        </div>

        <Card className="min-w-0">
          <CardHeader className="!px-4 !py-3">
            <div>
              <CardTitle>Assignment History</CardTitle>
              <CardDescription>Versions of this plan assigned to subjects over time.</CardDescription>
            </div>
          </CardHeader>
          <CardContent>
            <AssignmentHistoryTable assignments={planAssignmentHistory} />
          </CardContent>
        </Card>
      </div>

      {assignOpen ? (
        <Modal className="!w-full !max-w-[520px]" title="Assign Subject" onClose={() => {
          setAssignOpen(false)
          setAssignmentTiming('now')
        }}>
          <form className="modal-form !grid-cols-1" onSubmit={(event) => void submitAssignment(event)}>
            <Label className="grid gap-1.5">
              Subject
              <Input name="subject" placeholder="org_123" required />
            </Label>
            <Label className="grid gap-1.5">
              Plan
              <Input disabled value={plan?.name ?? ''} />
            </Label>
            <Label className="grid gap-1.5">
              Effective
              <Select onValueChange={(value) => setAssignmentTiming(value as 'now' | 'scheduled')} value={assignmentTiming}>
                <SelectTrigger>
                  <SelectValue placeholder="Choose timing" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="now">Now</SelectItem>
                  <SelectItem value="scheduled">Schedule change</SelectItem>
                </SelectContent>
              </Select>
            </Label>
            {assignmentTiming === 'scheduled' ? (
              <Label className="grid gap-1.5">
                Effective at
                <Input name="effective_at" required type="datetime-local" />
              </Label>
            ) : null}
            <div className="modal-actions">
              <Button onClick={() => {
                setAssignOpen(false)
                setAssignmentTiming('now')
              }} type="button" variant="outline">Cancel</Button>
              <Button disabled={assigning || !plan} type="submit">
                {assigning ? <Loader2 className="spin" aria-hidden="true" /> : <Plus aria-hidden="true" />}
                Assign
              </Button>
            </div>
          </form>
        </Modal>
      ) : null}

      {progressOpen ? (
        <Modal className="!w-full !max-w-[480px]" title="Check Usage Progress" onClose={() => setProgressOpen(false)}>
          <form className="modal-form !grid-cols-1" onSubmit={(event) => void submitProgress(event)}>
            <Label className="grid gap-1.5">
              Subject
              <Input
                name="subject"
                onChange={(event) => appStoreActions.setPlanProgressSubject(event.currentTarget.value)}
                placeholder="org_123"
                value={progressSubject}
              />
            </Label>
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

      {progressResultOpen && visibleProgress ? (
        <ProgressModal onClose={() => setProgressResultOpen(false)} progress={visibleProgress} />
      ) : null}

      {editing ? (
        <PlanModal
          meters={meters}
          onClose={() => {
            setPreviewError('')
            appStoreActions.setPlanEditing(null)
          }}
          onPreview={previewUpdate}
          onSubmit={submitUpdate}
          plan={editing}
          previewError={previewError}
          previewing={previewing}
          saving={saving}
          title="Edit Plan"
        />
      ) : null}

      {planPreview ? (
        <PlanPreviewModal onClose={() => setPlanPreview(null)} preview={planPreview} />
      ) : null}

      {deleting ? (
        <DeletePlanModal onConfirm={confirmDelete} plan={deleting} saving={saving} />
      ) : null}

      {selectedEntitlementEvent ? (
        <Modal className="!w-full !max-w-[760px]" title="Entitlement Change" onClose={() => appStoreActions.setPlanSelectedEntitlementEvent(null)}>
          <EntitlementEventDetail event={selectedEntitlementEvent} />
          <div className="modal-actions">
            <Button onClick={() => appStoreActions.setPlanSelectedEntitlementEvent(null)} type="button" variant="outline">Close</Button>
          </div>
        </Modal>
      ) : null}
    </>
  )
}

function errorMessage(err: unknown, fallback: string) {
  return err instanceof Error && err.message ? err.message : fallback
}
