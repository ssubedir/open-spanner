import { createFileRoute } from '@tanstack/react-router'

import { ReconciliationPage } from '../../../pages/ReconciliationPage'

export const Route = createFileRoute('/_dashboard/reconciliation')({ component: ReconciliationPage })
