import { createFileRoute, redirect } from '@tanstack/react-router'

import { loadAuthUser } from '../../api'
import { LoginPage } from '../../pages/LoginPage'

export const Route = createFileRoute('/login')({
  beforeLoad: async () => {
    if (await loadAuthUser()) {
      throw redirect({ to: '/overview' })
    }
  },
  component: LoginPage,
})
