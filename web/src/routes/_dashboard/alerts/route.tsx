import { createFileRoute } from '@tanstack/react-router'

import { AlertsPage } from '../../../pages/AlertsPage'

export const Route = createFileRoute('/_dashboard/alerts')({
  component: AlertsPage,
})
