import { createFileRoute } from '@tanstack/react-router'

import { AlertRoutePage } from '../../pages/AlertsPage'

export const Route = createFileRoute('/_dashboard/alerts_/$ruleId')({
  component: AlertRoutePage,
})
