import { Link, Outlet, useRouter } from '@tanstack/react-router'
import { useSelector } from '@tanstack/react-store'
import { BarChart3, BellRing, Boxes, FileArchive, Gauge, KeyRound, LayoutDashboard, LogOut, PackageCheck, Users } from 'lucide-react'

import { appStore, appStoreActions } from '../app-store'
import { Button } from '../components/ui/button'

const navGroups = [
  {
    description: 'Status and activity',
    label: 'Home',
    items: [
      { description: 'Workspace health', icon: LayoutDashboard, label: 'Overview', to: '/overview' },
    ],
  },
  {
    description: 'Usage model',
    items: [
      { description: 'Event definitions', icon: Boxes, label: 'Meters', to: '/meters' },
      { description: 'Customers and accounts', icon: Users, label: 'Subjects', to: '/subjects' },
    ],
    label: 'Catalog',
  },
  {
    description: 'Plans and limits',
    items: [
      { description: 'Quota packages', icon: PackageCheck, label: 'Plans', to: '/plans' },
      { description: 'Usage analysis', icon: BarChart3, label: 'Usage', to: '/usage' },
    ],
    label: 'Entitlements',
  },
  {
    description: 'Jobs and signals',
    items: [
      { description: 'Thresholds', icon: BellRing, label: 'Alerts', to: '/alerts' },
      { description: 'CSV jobs', icon: FileArchive, label: 'Exports', to: '/exports' },
    ],
    label: 'Operations',
  },
  {
    description: 'Credentials',
    label: 'Access',
    items: [
      { description: 'SDK access', icon: KeyRound, label: 'API Keys', to: '/api-keys' },
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
            <small>Usage infrastructure</small>
          </span>
        </Link>

        <nav className="nav" aria-label="Dashboard navigation">
          {navGroups.map((group) => (
            <div className="nav-group" key={group.label}>
              <div className="nav-group-heading">
                <span className="nav-group-label">{group.label}</span>
                <small>{group.description}</small>
              </div>
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
                      <span className="nav-link-icon"><Icon aria-hidden="true" /></span>
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
