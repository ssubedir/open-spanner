import { createFileRoute, useParams } from '@tanstack/react-router'

import { PlanDetailPage } from '../../pages/PlanDetailPage'

export const Route = createFileRoute('/_dashboard/plans_/$planId')({
  component: PlanRoute,
})

function PlanRoute() {
  const { planId } = useParams({ from: '/_dashboard/plans_/$planId' })

  return <PlanDetailPage planId={planId} />
}
