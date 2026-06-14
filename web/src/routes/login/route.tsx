import { createFileRoute, redirect } from '@tanstack/react-router'

import { appStoreActions } from '../../app-store'
import { LoginPage } from '../../pages/LoginPage'

export const Route = createFileRoute('/login')({
  beforeLoad: async () => {
    if (await appStoreActions.ensureAuthUser()) {
      throw redirect({ to: '/overview' })
    }
  },
  component: LoginPage,
})
