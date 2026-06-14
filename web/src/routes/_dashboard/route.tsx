import { createFileRoute, redirect } from '@tanstack/react-router'

import { appStoreActions } from '../../app-store'
import { AppShell } from '../../layouts/AppShell'

export const Route = createFileRoute('/_dashboard')({
  beforeLoad: async () => {
    if (!(await appStoreActions.ensureAuthUser())) {
      throw redirect({ to: '/login' })
    }
  },
  component: AppShell,
})
