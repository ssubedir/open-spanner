import { useRouter } from '@tanstack/react-router'
import { useSelector } from '@tanstack/react-store'
import { ArrowRight, PackageCheck, Pencil, Plus, Trash2 } from 'lucide-react'
import { useCallback, useMemo } from 'react'

import { appStore, appStoreActions } from '../app-store'
import { DataTable, PageHeader } from '../components/dashboard'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import type { PlanSaveRequest } from '../api'
import { formatDate, formatNumber } from '../lib/format'
import { useInitialLoad } from '../lib/hooks'
import { DeletePlanModal, LimitChips, PlanModal, countAssignmentsByPlan } from './PlanPageParts'

export function PlansPage() {
  const router = useRouter()
  const {
    assignments,
    creating,
    deleting,
    editing,
    error,
    items,
    meters,
    saving,
    status,
  } = useSelector(appStore, (state) => state.plans)
  const assignmentCounts = useMemo(() => countAssignmentsByPlan(assignments), [assignments])
  const load = useCallback(() => appStoreActions.loadPlans(), [])

  useInitialLoad(load)

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
        title="Plans"
        description="Keep quota packages small and open a plan when you need assignments or usage progress."
        action={(
          <Button disabled={saving || meters.length === 0} onClick={() => appStoreActions.setPlanCreating(true)} type="button">
            <Plus aria-hidden="true" />
            New plan
          </Button>
        )}
      />

      {error ? <div className="error-banner">{error}</div> : null}

      <Card className="max-w-[1480px] min-w-0">
        <CardHeader className="!px-4 !py-3">
          <div>
            <CardTitle>Plan catalog</CardTitle>
            <CardDescription>Named quota packages with the limits that define each tier.</CardDescription>
          </div>
        </CardHeader>
        <CardContent>
          <DataTable
            emptyLabel={status === 'loading' ? 'Loading plans' : 'No plans yet'}
            headers={['Plan', 'Limits', 'Subjects', 'Updated', 'Actions']}
            rows={items.map((plan) => [
              <span className="grid min-w-[220px] gap-1">
                <span className="flex min-w-0 items-center gap-2">
                  <strong>{plan.name}</strong>
                  <Badge variant="muted">v{plan.version}</Badge>
                </span>
                {plan.description ? <small className="max-w-[420px] truncate text-xs text-muted">{plan.description}</small> : null}
              </span>,
              <LimitChips limits={plan.limits} />,
              <Badge variant="muted">{formatNumber(assignmentCounts.get(plan.id) ?? 0)} subjects</Badge>,
              formatDate(plan.updated_at),
              <span className="table-actions">
                <Button aria-label={`Open ${plan.name}`} onClick={() => void router.navigate({ to: '/plans/$planId', params: { planId: plan.id } })} size="sm" type="button" variant="outline">
                  Open
                  <ArrowRight aria-hidden="true" />
                </Button>
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

      {creating ? (
        <PlanModal
          meters={meters}
          onClose={() => appStoreActions.setPlanCreating(false)}
          onSubmit={submitCreate}
          saving={saving}
          title="Create Plan"
        />
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
        <DeletePlanModal onConfirm={confirmDelete} plan={deleting} saving={saving} />
      ) : null}
    </>
  )
}
