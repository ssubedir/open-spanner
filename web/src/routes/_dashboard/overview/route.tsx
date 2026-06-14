import { createFileRoute } from '@tanstack/react-router'

import { OverviewPage } from '../../../pages/OverviewPage'

export const Route = createFileRoute('/_dashboard/overview')({
  component: OverviewPage,
})
