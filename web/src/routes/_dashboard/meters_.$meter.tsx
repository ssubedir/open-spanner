import { createFileRoute, useParams } from '@tanstack/react-router'

import { MeterDetailPage } from '../../pages/MeterDetailPage'

export const Route = createFileRoute('/_dashboard/meters_/$meter')({
  component: MeterRoute,
})

function MeterRoute() {
  const { meter } = useParams({ from: '/_dashboard/meters_/$meter' })

  return <MeterDetailPage routeMeter={meter} />
}
