import { createFileRoute } from '@tanstack/react-router'

import { PlansPage } from '../../../pages/PlansPage'

export const Route = createFileRoute('/_dashboard/plans')({
  component: PlansPage,
})
