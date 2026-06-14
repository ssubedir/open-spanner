import { createFileRoute } from '@tanstack/react-router'

import { APIKeysPage } from '../../../pages/APIKeysPage'

export const Route = createFileRoute('/_dashboard/api-keys')({
  component: APIKeysPage,
})
