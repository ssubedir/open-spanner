import { createFileRoute } from '@tanstack/react-router'

import { PlanRoutePage } from '../../pages/PlansPage'

export const Route = createFileRoute('/_dashboard/plans_/$planId')({
  component: PlanRoutePage,
})
