import { createFileRoute, redirect } from '@tanstack/react-router'

import { appStore, appStoreActions } from '../../app-store'
import { RegisterPage } from '../../pages/RegisterPage'

export const Route = createFileRoute('/register')({
  beforeLoad: async () => {
    if (await appStoreActions.ensureAuthUser()) {
      throw redirect({ to: '/overview' })
    }
    if (!appStore.state.auth.registrationEnabled) {
      throw redirect({ to: '/login' })
    }
  },
  component: RegisterPage,
})
