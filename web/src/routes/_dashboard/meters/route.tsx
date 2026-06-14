import { createFileRoute } from '@tanstack/react-router'

import { MetersPage } from '../../../pages/MetersPage'

export const Route = createFileRoute('/_dashboard/meters')({
  component: MetersPage,
})
