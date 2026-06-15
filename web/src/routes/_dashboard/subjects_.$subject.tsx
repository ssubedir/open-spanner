import { createFileRoute } from '@tanstack/react-router'

import { SubjectRoutePage } from '../../pages/SubjectsPage'

export const Route = createFileRoute('/_dashboard/subjects_/$subject')({
  component: SubjectRoutePage,
})
