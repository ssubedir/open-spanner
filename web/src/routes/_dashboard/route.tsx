import { createFileRoute, redirect } from '@tanstack/react-router'

import { loadAuthUser } from '../../api'
import { AppShell } from '../../layouts/AppShell'

export const Route = createFileRoute('/_dashboard')({
  beforeLoad: async () => {
    if (!(await loadAuthUser())) {
      throw redirect({ to: '/login' })
    }
  },
  component: AppShell,
})
