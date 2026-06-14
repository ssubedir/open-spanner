import { createFileRoute, redirect } from '@tanstack/react-router'

import { appStoreActions } from '../app-store'

export const Route = createFileRoute('/')({
  beforeLoad: async () => {
    throw redirect({ to: await appStoreActions.ensureAuthUser() ? '/overview' : '/login' })
  },
})
