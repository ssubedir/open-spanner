import { createFileRoute } from '@tanstack/react-router'

import { ExportsPage } from '../../../pages/ExportsPage'

export const Route = createFileRoute('/_dashboard/exports')({
  component: ExportsPage,
})
