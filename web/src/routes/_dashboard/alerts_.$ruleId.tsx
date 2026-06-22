import { createFileRoute, useParams } from '@tanstack/react-router'

import { AlertDetailPage } from '../../pages/AlertDetailPage'

export const Route = createFileRoute('/_dashboard/alerts_/$ruleId')({
  component: AlertRoute,
})

function AlertRoute() {
  const { ruleId } = useParams({ from: '/_dashboard/alerts_/$ruleId' })

  return <AlertDetailPage ruleId={ruleId} />
}
