import { createFileRoute } from '@tanstack/react-router'

import { UsagePage } from '../../../pages/UsagePage'

export const Route = createFileRoute('/_dashboard/usage')({
  component: UsagePage,
})
