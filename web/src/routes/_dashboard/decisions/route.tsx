import { createFileRoute } from '@tanstack/react-router'

import { DecisionsPage } from '../../../pages/DecisionsPage'

export const Route = createFileRoute('/_dashboard/decisions')({ component: DecisionsPage })
