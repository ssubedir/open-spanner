import { createFileRoute, redirect } from '@tanstack/react-router'

import { loadAuthUser } from '../api'

export const Route = createFileRoute('/')({
  beforeLoad: async () => {
    throw redirect({ to: await loadAuthUser() ? '/overview' : '/login' })
  },
})
