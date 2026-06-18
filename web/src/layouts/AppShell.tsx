import { Link, Outlet, useRouter } from '@tanstack/react-router'
import { useSelector } from '@tanstack/react-store'
import { BarChart3, Boxes, FileArchive, Gauge, KeyRound, LayoutDashboard, LogOut, Users } from 'lucide-react'

import { appStore, appStoreActions } from '../app-store'
import { Button } from '../components/ui/button'

const navGroups = [
  {
    label: 'Workspace',
    items: [
      { description: 'Health and activity', icon: LayoutDashboard, label: 'Overview', to: '/overview' },
    ],
  },
  {
    label: 'Metering',
    items: [
      { description: 'Definitions', icon: Boxes, label: 'Meters', to: '/meters' },
      { description: 'Accounts and customers', icon: Users, label: 'Subjects', to: '/subjects' },
      { description: 'Query and breakdowns', icon: BarChart3, label: 'Usage', to: '/usage' },
      { description: 'CSV job history', icon: FileArchive, label: 'Exports', to: '/exports' },
    ],
  },
  {
    label: 'Access',
    items: [
      { description: 'SDK credentials', icon: KeyRound, label: 'API Keys', to: '/api-keys' },
    ],
  },
] as const

export function AppShell() {
  const router = useRouter()
  const session = useSelector(appStore, (state) => state.auth.session)
  const user = session?.user ?? null

  async function signOut() {
    await appStoreActions.logout()
    void router.navigate({ to: '/login' })
  }

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <Link className="brand" to="/overview" aria-label="Open Spanner overview">
          <span className="brand-mark"><Gauge aria-hidden="true" /></span>
          <span>
            <strong>Open Spanner</strong>
            <small>Admin</small>
          </span>
        </Link>

        <nav className="nav" aria-label="Admin navigation">
          {navGroups.map((group) => (
            <div className="nav-group" key={group.label}>
              <span className="nav-group-label">{group.label}</span>
              <div className="nav-group-links">
                {group.items.map((item) => {
                  const Icon = item.icon
                  return (
                    <Link
                      activeProps={{ className: 'nav-link active' }}
                      className="nav-link"
                      key={item.to}
                      to={item.to}
                    >
                      <Icon aria-hidden="true" />
                      <span>
                        <strong>{item.label}</strong>
                        <small>{item.description}</small>
                      </span>
                    </Link>
                  )
                })}
              </div>
            </div>
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
