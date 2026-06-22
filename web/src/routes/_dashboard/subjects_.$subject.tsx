import { createFileRoute, useParams } from '@tanstack/react-router'

import { SubjectDetailPage } from '../../pages/SubjectDetailPage'

export const Route = createFileRoute('/_dashboard/subjects_/$subject')({
  component: SubjectRoute,
})

function SubjectRoute() {
  const { subject } = useParams({ from: '/_dashboard/subjects_/$subject' })

  return <SubjectDetailPage routeSubject={subject} />
}
