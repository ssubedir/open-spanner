import { Link, Outlet, useRouter } from '@tanstack/react-router'
import { Gauge, LogOut } from 'lucide-react'
import { useState } from 'react'

import { deleteAuthSession, readAuthUser, type AuthUser } from '../api'
import { Button } from '../components/ui/button'

const navItems = [
  { to: '/overview', label: 'Overview' },
  { to: '/meters', label: 'Meters' },
  { to: '/usage', label: 'Usage' },
] as const

export function AppShell() {
  const router = useRouter()
  const [user, setUser] = useState<AuthUser | null>(() => readAuthUser())

  async function signOut() {
    await deleteAuthSession()
    setUser(null)
    void router.navigate({ to: '/login' })
  }

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <Link className="brand" to="/overview" aria-label="Open Spanner overview">
          <span className="brand-mark"><Gauge aria-hidden="true" /></span>
          <span>
            <strong>Open Spanner</strong>
          </span>
        </Link>

        <nav className="nav" aria-label="Admin navigation">
          {navItems.map((item) => (
            <Link
              activeProps={{ className: 'active' }}
              key={item.to}
              to={item.to}
            >
              {item.label}
            </Link>
          ))}
        </nav>

        <div className="sidebar-session">
          <div>
            <span>Signed in</span>
            <strong>{user?.email ?? 'Unknown user'}</strong>
          </div>
          <Button aria-label="Sign out" onClick={() => void signOut()} size="icon" type="button" variant="ghost">
            <LogOut aria-hidden="true" />
          </Button>
        </div>
      </aside>

      <main className="main">
        <Outlet />
      </main>
    </div>
  )
}
