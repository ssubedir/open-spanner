import { createFileRoute } from '@tanstack/react-router'

import { SubjectsPage } from '../../../pages/SubjectsPage'

export const Route = createFileRoute('/_dashboard/subjects')({
  component: SubjectsPage,
})
