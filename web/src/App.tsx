import {
  Link,
  Outlet,
  RouterProvider,
  createRoute,
  createRouter,
  createRootRoute,
  redirect,
} from '@tanstack/react-router'
import {
  Activity,
  BarChart3,
  Boxes,
  CheckCircle2,
  Clock,
  Database,
  Gauge,
  Loader2,
  LogIn,
  LogOut,
  Pencil,
  Plus,
  RefreshCw,
  Rows3,
  Search,
  ShieldCheck,
  Trash2,
} from 'lucide-react'
import { type FormEvent, useCallback, useEffect, useMemo, useState } from 'react'
import {
  QueryBuilder,
  type Field,
  type Operator,
  type RuleGroupType,
  type RuleType,
} from 'react-querybuilder'
import 'react-querybuilder/dist/query-builder.css'

import {
  createAuthSession,
  createMeter,
  createUsage,
  deleteAuthSession,
  deleteMeter,
  getSystemStats,
  listIngestions,
  listMeterStats,
  listMeters,
  listSubjects,
  listUsageBuckets,
  loadAuthUser,
  readAuthUser,
  setAuthUser,
  updateMeter,
  type AuthUser,
  type IngestionRun,
  type Meter,
  type MeterStats,
  type SubjectStats,
  type SystemStats,
  type UsageBucket,
  type UsageFilter,
  type UsageFilterCondition,
} from './api'
import { Badge } from './components/ui/badge'
import { Button } from './components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from './components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from './components/ui/table'

type LoadState = 'idle' | 'loading' | 'ready' | 'error'

const navItems = [
  { to: '/overview', label: 'Overview' },
  { to: '/meters', label: 'Meters' },
  { to: '/usage', label: 'Usage' },
] as const

const aggregations = ['sum', 'count', 'avg', 'min', 'max', 'first', 'last', 'rate']

function defaultFilterQuery(): RuleGroupType {
  const dates = defaultQueryDates()
  return {
    combinator: 'and',
    rules: [
      { field: 'subject', operator: '=', value: 'org_123' },
      { field: 'meter', operator: '=', value: '' },
      { field: 'timestamp', operator: '>=', value: dates.from },
      { field: 'timestamp', operator: '<=', value: dates.to },
    ],
  }
}

const rootRoute = createRootRoute({
  component: RootShell,
})

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  beforeLoad: async () => {
    throw redirect({ to: await loadAuthUser() ? '/overview' : '/login' })
  },
})

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/login',
  beforeLoad: async () => {
    if (await loadAuthUser()) {
      throw redirect({ to: '/overview' })
    }
  },
  component: LoginPage,
})

const dashboardRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: 'dashboard',
  beforeLoad: async () => {
    if (!(await loadAuthUser())) {
      throw redirect({ to: '/login' })
    }
  },
  component: AppShell,
})

const overviewRoute = createRoute({
  getParentRoute: () => dashboardRoute,
  path: '/overview',
  component: OverviewPage,
})

const metersRoute = createRoute({
  getParentRoute: () => dashboardRoute,
  path: '/meters',
  component: MetersPage,
})

const usageRoute = createRoute({
  getParentRoute: () => dashboardRoute,
  path: '/usage',
  component: UsagePage,
})

const routeTree = rootRoute.addChildren([
  indexRoute,
  loginRoute,
  dashboardRoute.addChildren([overviewRoute, metersRoute, usageRoute]),
])

const router = createRouter({
  routeTree,
})

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}

function App() {
  return <RouterProvider router={router} />
}

function RootShell() {
  return <Outlet />
}

function AppShell() {
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

function LoginPage() {
  const [status, setStatus] = useState<LoadState>('idle')
  const [error, setError] = useState('')

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    const email = String(form.get('email') || '')
    const password = String(form.get('password') || '')

    setStatus('loading')
    setError('')
    try {
      const session = await createAuthSession({ email, password })
      setAuthUser(session.user)
      await router.navigate({ to: '/overview' })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unable to authenticate')
      setStatus('error')
    }
  }

  return (
    <main className="auth-page">
      <section className="auth-panel" aria-labelledby="auth-title">
        <div className="auth-heading">
          <div className="auth-icon"><ShieldCheck aria-hidden="true" /></div>
          <div>
            <h1 id="auth-title">Sign in</h1>
          </div>
        </div>

        {error ? <div className="error-banner">{error}</div> : null}

        <form className="auth-form" onSubmit={(event) => void submit(event)}>
          <label>
            Email
            <input autoComplete="email" name="email" placeholder="admin@example.com" required type="email" />
          </label>
          <label>
            Password
            <input autoComplete="current-password" minLength={8} name="password" required type="password" />
          </label>
          <Button disabled={status === 'loading'} type="submit">
            {status === 'loading' ? <Loader2 className="spin" aria-hidden="true" /> : <LogIn aria-hidden="true" />}
            Sign in
          </Button>
        </form>
      </section>
    </main>
  )
}

function OverviewPage() {
  const [status, setStatus] = useState<LoadState>('idle')
  const [error, setError] = useState('')
  const [stats, setStats] = useState<SystemStats | null>(null)
  const [subjects, setSubjects] = useState<SubjectStats[]>([])
  const [ingestions, setIngestions] = useState<IngestionRun[]>([])

  const load = useCallback(async () => {
    setStatus('loading')
    setError('')
    try {
      const [nextStats, nextSubjects, nextIngestions] = await Promise.all([
        getSystemStats(),
        listSubjects(),
        listIngestions(),
      ])
      setStats(nextStats)
      setSubjects(nextSubjects.items)
      setIngestions(nextIngestions.items)
      setStatus('ready')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unable to load overview')
      setStatus('error')
    }
  }, [])

  useInitialLoad(load)

  const topSubject = useMemo(() => subjects[0], [subjects])

  return (
    <>
      <PageHeader
        eyebrow="Overview"
        icon={<Activity />}
        title="Metering operations"
        description="Monitor core usage activity, recent ingestion, and subject volume."
        action={(
          <Button disabled={status === 'loading'} onClick={() => void load()} type="button" variant="outline">
            {status === 'loading' ? <Loader2 className="spin" aria-hidden="true" /> : <RefreshCw aria-hidden="true" />}
            Refresh
          </Button>
        )}
      />

      {error ? <div className="error-banner">{error}</div> : null}

      <section className="metric-grid" aria-label="Operational metrics">
        <MetricCard icon={<Boxes />} label="Meters" value={stats?.meters ?? 0} helper="Configured billable signals" />
        <MetricCard icon={<BarChart3 />} label="Usage Events" value={stats?.usage_events ?? 0} helper="Raw events accepted" />
        <MetricCard icon={<Rows3 />} label="Prune Runs" value={stats?.prune_runs ?? 0} helper="Retention jobs recorded" />
        <MetricCard
          icon={<Clock />}
          label="Last Prune"
          value={stats?.last_prune_run ? stats.last_prune_run.deleted : 0}
          helper={stats?.last_prune_run ? formatDate(stats.last_prune_run.created_at) : 'No prune runs yet'}
        />
      </section>

      <section className="content-grid">
        <Card className="activity-card">
          <CardHeader>
            <div>
              <CardTitle>Subjects</CardTitle>
              <CardDescription>Highest recent subject activity.</CardDescription>
            </div>
            <Badge variant={subjects.length > 0 ? 'success' : 'muted'}>{subjects.length} rows</Badge>
          </CardHeader>
          <CardContent>
            <DataTable
              emptyLabel="No subject activity yet"
              headers={['Subject', 'Events', 'Meters', 'Last Event']}
              rows={subjects.map((subject) => [
                <span className="mono strong">{subject.subject}</span>,
                formatNumber(subject.usage_events),
                formatNumber(subject.meters),
                formatDate(subject.last_event_at),
              ])}
            />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <div>
              <CardTitle>Snapshot</CardTitle>
              <CardDescription>Current service posture.</CardDescription>
            </div>
            <Badge variant="warning">Live</Badge>
          </CardHeader>
          <CardContent className="snapshot">
            <SnapshotItem label="Top subject" value={topSubject?.subject ?? 'None'} />
            <SnapshotItem label="Top subject events" value={topSubject ? formatNumber(topSubject.usage_events) : '0'} />
            <SnapshotItem label="Last prune mode" value={stats?.last_prune_run?.dry_run ? 'Dry run' : stats?.last_prune_run ? 'Delete' : 'None'} />
            <div className="quick-actions">
              <Link className="quick-action" to="/meters"><Database aria-hidden="true" /> Meters</Link>
              <Link className="quick-action" to="/usage"><CheckCircle2 aria-hidden="true" /> Usage</Link>
            </div>
          </CardContent>
        </Card>

        <Card className="activity-card span">
          <CardHeader>
            <div>
              <CardTitle>Ingestion History</CardTitle>
              <CardDescription>Recent single and bulk ingestion runs.</CardDescription>
            </div>
            <Badge variant={ingestions.length > 0 ? 'success' : 'muted'}>{ingestions.length} rows</Badge>
          </CardHeader>
          <CardContent>
            <DataTable
              emptyLabel="No ingestion history yet"
              headers={['Created', 'Kind', 'Accepted', 'Duplicates', 'Failed', 'ID']}
              rows={ingestions.map((run) => [
                formatDate(run.created_at),
                <Badge variant="muted">{run.kind}</Badge>,
                formatNumber(run.accepted),
                formatNumber(run.duplicates),
                formatNumber(run.failed),
                <span className="mono truncate">{run.id}</span>,
              ])}
            />
          </CardContent>
        </Card>
      </section>
    </>
  )
}

function MetersPage() {
  const [status, setStatus] = useState<LoadState>('idle')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [meters, setMeters] = useState<Meter[]>([])
  const [stats, setStats] = useState<Record<string, MeterStats>>({})
  const [editing, setEditing] = useState<Meter | null>(null)
  const [deleting, setDeleting] = useState<Meter | null>(null)

  const load = useCallback(async () => {
    setStatus('loading')
    setError('')
    try {
      const [nextMeters, nextStats] = await Promise.all([listMeters(), listMeterStats()])
      setMeters(nextMeters.items)
      setStats(Object.fromEntries(nextStats.items.map((item) => [item.meter, item])))
      setStatus('ready')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unable to load meters')
      setStatus('error')
    }
  }, [])

  useInitialLoad(load)

  async function submitCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const formElement = event.currentTarget
    setSaving(true)
    setError('')
    const form = new FormData(formElement)

    try {
      await createMeter({
        aggregation: String(form.get('aggregation') || 'sum'),
        description: String(form.get('description') || ''),
        event_retention_days: Number(form.get('event_retention_days') || 90),
        metadata_schema: parseMetadataSchema(String(form.get('metadata_schema') || '{}')),
        name: String(form.get('name') || ''),
        unit: String(form.get('unit') || ''),
      })
      formElement.reset()
      const metadata = formElement.elements.namedItem('metadata_schema')
      if (metadata instanceof HTMLTextAreaElement) {
        metadata.value = '{}'
      }
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unable to create meter')
    } finally {
      setSaving(false)
    }
  }

  async function submitEdit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!editing) {
      return
    }
    setSaving(true)
    setError('')
    const form = new FormData(event.currentTarget)

    try {
      await updateMeter(editing.id, { description: String(form.get('description') || '') })
      setEditing(null)
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unable to update meter')
    } finally {
      setSaving(false)
    }
  }

  async function confirmDelete() {
    if (!deleting) {
      return
    }
    setSaving(true)
    setError('')

    try {
      await deleteMeter(deleting.id)
      setDeleting(null)
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unable to delete meter')
    } finally {
      setSaving(false)
    }
  }

  return (
    <>
      <PageHeader
        eyebrow="Meters"
        icon={<Boxes />}
        title="Meter definitions"
        description="Create and maintain the billable signals accepted by the usage API."
        action={(
          <Button disabled={status === 'loading'} onClick={() => void load()} type="button" variant="outline">
            {status === 'loading' ? <Loader2 className="spin" aria-hidden="true" /> : <RefreshCw aria-hidden="true" />}
            Refresh
          </Button>
        )}
      />

      {error ? <div className="error-banner">{error}</div> : null}

      <section className="metric-grid meters-metrics" aria-label="Meter metrics">
        <MetricCard icon={<Boxes />} label="Meters" value={meters.length} helper="Definitions configured" />
        <MetricCard icon={<BarChart3 />} label="Usage Events" value={sumMeterEvents(stats)} helper="Events attached to meters" />
        <MetricCard icon={<Rows3 />} label="Aggregations" value={new Set(meters.map((meter) => meter.aggregation)).size} helper="Aggregation modes in use" />
        <MetricCard icon={<Clock />} label="Avg Retention" value={averageRetention(meters)} helper="Days across meters" />
      </section>

      <section className="meters-grid">
        <Card>
          <CardHeader>
            <div>
              <CardTitle>Create Meter</CardTitle>
              <CardDescription>Define a signal, its unit, aggregation, and metadata contract.</CardDescription>
            </div>
          </CardHeader>
          <CardContent className="form-card">
            <form className="form-grid" onSubmit={(event) => void submitCreate(event)}>
              <label>
                Name
                <input id="meter-name" name="name" placeholder="api_calls" required />
              </label>
              <label>
                Unit
                <input id="meter-unit" name="unit" placeholder="request" required />
              </label>
              <label>
                Aggregation
                <select name="aggregation" required>
                  {aggregations.map((item) => <option key={item} value={item}>{item}</option>)}
                </select>
              </label>
              <label>
                Retention Days
                <input defaultValue="90" max="3650" min="1" name="event_retention_days" required type="number" />
              </label>
              <label className="wide">
                Description
                <input id="meter-description" name="description" placeholder="API requests accepted by the platform" />
              </label>
              <label className="wide" htmlFor="meter-metadata-schema">
                Metadata Schema JSON
                <textarea aria-label="Metadata Schema JSON" defaultValue="{}" id="meter-metadata-schema" name="metadata_schema" rows={5} />
              </label>
              <Button disabled={saving} type="submit">
                {saving ? <Loader2 className="spin" aria-hidden="true" /> : <Plus aria-hidden="true" />}
                Create
              </Button>
            </form>
          </CardContent>
        </Card>

        <Card className="meter-table-card">
          <CardHeader>
            <div>
              <CardTitle>Meters</CardTitle>
              <CardDescription>Configured meter definitions and current activity.</CardDescription>
            </div>
            <Badge variant={meters.length > 0 ? 'success' : 'muted'}>{meters.length} rows</Badge>
          </CardHeader>
          <CardContent>
            <div className="table-wrap">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Name</TableHead>
                    <TableHead>Aggregation</TableHead>
                    <TableHead>Unit</TableHead>
                    <TableHead>Retention</TableHead>
                    <TableHead>Events</TableHead>
                    <TableHead>Last Event</TableHead>
                    <TableHead>Schema</TableHead>
                    <TableHead>Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {meters.length === 0 ? (
                    <EmptyRow colSpan={8} label="No meters yet" />
                  ) : meters.map((meter) => {
                    const stat = stats[meter.name]
                    return (
                      <TableRow key={meter.id}>
                        <TableCell>
                          <div className="stack-cell">
                            <strong>{meter.name}</strong>
                            <small>{meter.description || 'No description'}</small>
                          </div>
                        </TableCell>
                        <TableCell><Badge variant="muted">{meter.aggregation}</Badge></TableCell>
                        <TableCell>{meter.unit}</TableCell>
                        <TableCell>{meter.event_retention_days} days</TableCell>
                        <TableCell>{formatNumber(stat?.usage_events ?? 0)}</TableCell>
                        <TableCell>{stat?.last_event_at ? formatDate(stat.last_event_at) : 'Never'}</TableCell>
                        <TableCell className="mono truncate">{JSON.stringify(meter.metadata_schema || {})}</TableCell>
                        <TableCell>
                          <div className="table-actions">
                            <Button aria-label={`Edit ${meter.name}`} onClick={() => setEditing(meter)} size="icon" type="button" variant="ghost">
                              <Pencil aria-hidden="true" />
                            </Button>
                            <Button aria-label={`Delete ${meter.name}`} onClick={() => setDeleting(meter)} size="icon" type="button" variant="ghost">
                              <Trash2 aria-hidden="true" />
                            </Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    )
                  })}
                </TableBody>
              </Table>
            </div>
          </CardContent>
        </Card>
      </section>

      {editing ? (
        <Modal title="Edit Meter" onClose={() => setEditing(null)}>
          <form className="modal-form" onSubmit={(event) => void submitEdit(event)}>
            <label>
              Name
              <input disabled value={editing.name} />
            </label>
            <label>
              Description
              <textarea defaultValue={editing.description} name="description" rows={5} />
            </label>
            <div className="modal-actions">
              <Button onClick={() => setEditing(null)} type="button" variant="outline">Cancel</Button>
              <Button disabled={saving} type="submit">Save</Button>
            </div>
          </form>
        </Modal>
      ) : null}

      {deleting ? (
        <Modal title="Delete Meter" onClose={() => setDeleting(null)}>
          <div className="modal-copy">Delete <strong>{deleting.name}</strong>?</div>
          <div className="modal-actions">
            <Button onClick={() => setDeleting(null)} type="button" variant="outline">Cancel</Button>
            <Button disabled={saving} onClick={() => void confirmDelete()} type="button">Delete</Button>
          </div>
        </Modal>
      ) : null}
    </>
  )
}

function UsagePage() {
  const [status, setStatus] = useState<LoadState>('idle')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [meters, setMeters] = useState<Meter[]>([])
  const [buckets, setBuckets] = useState<UsageBucket[]>([])
  const [createOpen, setCreateOpen] = useState(false)
  const [groupBy, setGroupBy] = useState('')
  const [filterQuery, setFilterQuery] = useState<RuleGroupType>(() => defaultFilterQuery())

  const load = useCallback(async () => {
    setStatus('loading')
    setError('')
    try {
      const nextMeters = await listMeters()
      setMeters(nextMeters.items)
      setFilterQuery((query) => queryWithAvailableMeter(query, nextMeters.items))
      setStatus('ready')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unable to load usage page')
      setStatus('error')
    }
  }, [])

  useInitialLoad(load)

  async function submitCreateUsage(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const formElement = event.currentTarget
    const form = new FormData(formElement)
    setSaving(true)
    setError('')

    try {
      await createUsage({
        idempotency_key: String(form.get('idempotency_key') || `usage_${Date.now()}`),
        metadata: parseJSONRecord(String(form.get('metadata') || '{}'), 'Metadata'),
        meter: String(form.get('meter') || ''),
        quantity: Number(form.get('quantity') || 0),
        subject: String(form.get('subject') || ''),
        timestamp: localDateTimeToISO(String(form.get('timestamp') || '')) || new Date().toISOString(),
      })
      formElement.reset()
      const metadata = formElement.elements.namedItem('metadata')
      if (metadata instanceof HTMLTextAreaElement) {
        metadata.value = '{}'
      }
      setCreateOpen(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unable to create usage')
    } finally {
      setSaving(false)
    }
  }

  async function submitQuery(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    setStatus('loading')
    setError('')

    try {
      const filter = usageFilterFromQuery(filterQuery)
      const scope = usageScopeFromQuery(filterQuery)
      const timeRange = usageTimeRangeFromQuery(filterQuery)
      const nextBuckets = await listUsageBuckets({
        bucket_size: String(form.get('bucket_size') || 'day'),
        filter,
        from: timeRange.from,
        group_by: activeGroupBy,
        limit: Number(form.get('limit') || 500),
        meter: scope.meter,
        subject: scope.subject,
        to: timeRange.to,
      })
      setBuckets(nextBuckets)
      setStatus('ready')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unable to query usage')
      setStatus('error')
    }
  }

  const total = buckets.reduce((sum, bucket) => sum + Number(bucket.quantity || 0), 0)
  const selectedMeterName = firstEqualRuleValue(filterQuery, 'meter')
  const groupKeys = useMemo(() => selectedMeterSchemaKeys(meters, selectedMeterName), [meters, selectedMeterName])
  const activeGroupBy = groupKeys.includes(groupBy) ? groupBy : ''
  const filterFields = useMemo(() => buildFilterFields(groupKeys, meters), [groupKeys, meters])

  function resetQuery() {
    setGroupBy('')
    setFilterQuery(queryWithAvailableMeter(defaultFilterQuery(), meters))
  }

  return (
    <>
      <PageHeader
        eyebrow="Usage"
        icon={<BarChart3 />}
        title="Usage buckets"
        description="Query bucketed usage with a time window, bucket settings, and advanced filters."
        action={(
          <div className="header-actions">
            <Button disabled={status === 'loading'} onClick={() => void load()} type="button" variant="outline">
              {status === 'loading' ? <Loader2 className="spin" aria-hidden="true" /> : <RefreshCw aria-hidden="true" />}
              Refresh
            </Button>
            <Button onClick={() => setCreateOpen(true)} type="button">
              <Plus aria-hidden="true" />
              Create Usage
            </Button>
          </div>
        )}
      />

      {error ? <div className="error-banner">{error}</div> : null}

      <section className="metric-grid meters-metrics" aria-label="Usage metrics">
        <MetricCard icon={<Database />} label="Meters" value={meters.length} helper="Available for queries" />
        <MetricCard icon={<Rows3 />} label="Buckets" value={buckets.length} helper="Rows in current result" />
        <MetricCard icon={<BarChart3 />} label="Total Quantity" value={Math.round(total)} helper="Sum of visible buckets" />
        <MetricCard icon={<Clock />} label="Window Days" value={7} helper="Default query range" />
      </section>

      <section className="usage-grid">
        <Card>
          <CardHeader>
            <div>
              <CardTitle>Usage Query</CardTitle>
              <CardDescription>Filter with rules, then choose the result shape.</CardDescription>
            </div>
          </CardHeader>
          <CardContent className="form-card">
            <form className="form-grid usage-query-form" onSubmit={(event) => void submitQuery(event)}>
              <FilterBuilder
                fields={filterFields}
                onChange={setFilterQuery}
                query={filterQuery}
              />
              <div className="query-controls wide">
                <label>
                  Bucket
                  <select aria-label="Bucket" name="bucket_size">
                    <option value="day">Day</option>
                    <option value="hour">Hour</option>
                    <option value="month">Month</option>
                  </select>
                </label>
                <label>
                  Group By
                  <select aria-label="Group By" name="group_by" value={activeGroupBy} onChange={(event) => setGroupBy(event.target.value)}>
                    <option value="">None</option>
                    {groupKeys.map((key) => <option key={key} value={key}>{key}</option>)}
                  </select>
                </label>
                <label>
                  Limit
                  <input defaultValue="500" max="1000" min="1" name="limit" type="number" />
                </label>
                <div className="query-actions">
                  <Button onClick={resetQuery} type="button" variant="outline">
                    <RefreshCw aria-hidden="true" />
                    Reset
                  </Button>
                  <Button disabled={status === 'loading'} type="submit">
                    {status === 'loading' ? <Loader2 className="spin" aria-hidden="true" /> : <Search aria-hidden="true" />}
                    Run Query
                  </Button>
                </div>
              </div>
            </form>
          </CardContent>
        </Card>

        <Card className="usage-results-card">
          <CardHeader>
            <div>
              <CardTitle>Results</CardTitle>
              <CardDescription>Bucketed usage returned by the current query.</CardDescription>
            </div>
            <Badge variant={buckets.length > 0 ? 'success' : 'muted'}>{buckets.length} rows</Badge>
          </CardHeader>
          <CardContent>
            <DataTable
              emptyLabel="Run a query to view usage"
              headers={['Bucket Start', 'Subject', 'Meter', 'Aggregation', 'Unit', 'Group', 'Quantity']}
              rows={buckets.map((bucket) => [
                formatDate(bucket.bucket_start),
                <span className="mono strong">{bucket.subject}</span>,
                bucket.meter,
                <Badge variant="muted">{bucket.aggregation}</Badge>,
                bucket.unit,
                <span className="mono truncate">{JSON.stringify(bucket.group || {})}</span>,
                formatNumber(bucket.quantity),
              ])}
            />
          </CardContent>
        </Card>
      </section>

      {createOpen ? (
        <Modal title="Create Usage" onClose={() => setCreateOpen(false)}>
          <form className="modal-form usage-create-form" onSubmit={(event) => void submitCreateUsage(event)}>
            <label>
              Subject
              <input name="subject" placeholder="org_123" required />
            </label>
              <label>
                Meter
              <select aria-label="Meter" name="meter" required>
                <option value="">Select meter</option>
                {meters.map((meter) => <option key={meter.id} value={meter.name}>{meter.name}</option>)}
              </select>
            </label>
            <label>
              Quantity
              <input defaultValue="1" min="0" name="quantity" required step="0.000001" type="number" />
            </label>
              <label>
                Timestamp
              <input aria-label="Timestamp" defaultValue={toInputDateTime(new Date())} name="timestamp" type="datetime-local" />
            </label>
            <label className="wide">
              Idempotency Key
              <input name="idempotency_key" placeholder="Generated if blank" />
            </label>
            <label className="wide">
              Metadata JSON
              <textarea aria-label="Metadata JSON" defaultValue="{}" name="metadata" rows={5} />
            </label>
            <div className="modal-actions">
              <Button onClick={() => setCreateOpen(false)} type="button" variant="outline">Cancel</Button>
              <Button disabled={saving} type="submit">
                {saving ? <Loader2 className="spin" aria-hidden="true" /> : <Plus aria-hidden="true" />}
                Create
              </Button>
            </div>
          </form>
        </Modal>
      ) : null}
    </>
  )
}

function FilterBuilder({ fields, onChange, query }: { fields: Field[]; onChange: (query: RuleGroupType) => void; query: RuleGroupType }) {
  return (
    <div className="filter-builder wide">
      <div className="filter-builder-header">
        <div>
          <span>Advanced Filters</span>
          <small>{countQueryRules(query)} active</small>
        </div>
      </div>
      <QueryBuilder
        fields={fields}
        getInputType={getFilterInputType}
        getOperators={getFilterOperators}
        listsAsArrays
        onQueryChange={onChange}
        parseNumbers="native"
        query={query}
        translations={{
          addGroup: { label: '+ Group' },
          addRule: { label: '+ Rule' },
        }}
      />
    </div>
  )
}

function buildFilterFields(metadataKeys: string[], meters: Meter[]): Field[] {
  return [
    { name: 'subject', label: 'Subject' },
    {
      name: 'meter',
      label: 'Meter',
      valueEditorType: 'select',
      values: meters.map((meter) => ({ name: meter.name, label: meter.name })),
    },
    { name: 'quantity', label: 'Quantity', inputType: 'number' },
    { name: 'timestamp', label: 'Timestamp', inputType: 'datetime-local' },
    { name: 'received_at', label: 'Received At', inputType: 'datetime-local' },
    { name: 'idempotency_key', label: 'Idempotency Key' },
    ...metadataKeys.map((key) => ({ name: `metadata.${key}`, label: `Metadata: ${key}` })),
  ]
}

function getFilterOperators(field: string): Operator[] {
  if (field === 'quantity' || field === 'timestamp' || field === 'received_at') {
    return [
      { name: '=', label: 'equals' },
      { name: '!=', label: 'not equals' },
      { name: '>', label: 'greater than' },
      { name: '>=', label: 'greater or equal' },
      { name: '<', label: 'less than' },
      { name: '<=', label: 'less or equal' },
    ]
  }

  return [
    { name: '=', label: 'equals' },
    { name: '!=', label: 'not equals' },
    { name: 'contains', label: 'contains' },
    { name: 'in', label: 'in list' },
    { name: 'notNull', label: 'exists', arity: 'unary' },
  ]
}

function getFilterInputType(field: string) {
  if (field === 'quantity') {
    return 'number'
  }
  if (field === 'timestamp' || field === 'received_at') {
    return 'datetime-local'
  }
  return 'text'
}

function usageFilterFromQuery(query: RuleGroupType): UsageFilter | undefined {
  const rules = query.rules
    .map((rule) => isQueryGroup(rule) ? usageFilterFromQuery(rule) : usageFilterFromRule(rule))
    .filter((rule): rule is UsageFilter => rule !== undefined)

  if (rules.length === 0) {
    return undefined
  }
  if (rules.length === 1) {
    return rules[0]
  }
  return {
    type: 'group',
    op: query.combinator === 'or' ? 'or' : 'and',
    rules,
  }
}

function usageFilterFromRule(rule: RuleType): UsageFilter | undefined {
  if (!rule.field || !rule.operator) {
    return undefined
  }

  const op = usageOperatorFromQueryOperator(rule.operator)
  if (!op) {
    return undefined
  }

  const value = usageValueFromRule(rule)
  if (op !== 'exists' && value === undefined) {
    return undefined
  }

  return {
    type: 'condition',
    field: rule.field,
    op,
    value,
  }
}

function usageOperatorFromQueryOperator(operator: string): UsageFilterCondition['op'] | undefined {
  switch (operator) {
    case '=':
      return 'eq'
    case '!=':
      return 'neq'
    case '>':
      return 'gt'
    case '>=':
      return 'gte'
    case '<':
      return 'lt'
    case '<=':
      return 'lte'
    case 'in':
      return 'in'
    case 'contains':
      return 'contains'
    case 'notNull':
      return 'exists'
    default:
      return undefined
  }
}

function usageValueFromRule(rule: RuleType) {
  if (rule.operator === 'notNull') {
    return undefined
  }
  if (rule.operator === 'in') {
    return Array.isArray(rule.value)
      ? rule.value
      : String(rule.value || '').split(',').map((value) => value.trim()).filter(Boolean)
  }
  if (rule.field === 'timestamp' || rule.field === 'received_at') {
    return localDateTimeToISO(String(rule.value || '')) || undefined
  }
  if (rule.field === 'quantity') {
    return rule.value === '' || rule.value === undefined ? undefined : Number(rule.value)
  }
  return rule.value === '' || rule.value === undefined ? undefined : rule.value
}

function usageScopeFromQuery(query: RuleGroupType) {
  const subject = firstEqualRuleValue(query, 'subject')
  const meter = firstEqualRuleValue(query, 'meter')
  if (!subject || !meter) {
    throw new Error('Usage query needs subject and meter filters')
  }
  return { meter, subject }
}

function usageTimeRangeFromQuery(query: RuleGroupType) {
  const from = firstComparableRuleValue(query, 'timestamp', ['>=', '>'])
  const to = firstComparableRuleValue(query, 'timestamp', ['<=', '<'])
  if (!from || !to) {
    throw new Error('Usage query needs timestamp from and to filters')
  }
  return {
    from: localDateTimeToISO(from),
    to: localDateTimeToISO(to),
  }
}

function queryWithAvailableMeter(query: RuleGroupType, meters: Meter[]): RuleGroupType {
  const availableMeters = new Set(meters.map((meter) => meter.name))
  const fallbackMeter = meters[0]?.name || ''
  if (!fallbackMeter) {
    return query
  }
  return replaceRuleValue(query, 'meter', (value) => availableMeters.has(value) ? value : fallbackMeter)
}

function replaceRuleValue(query: RuleGroupType, field: string, nextValue: (value: string) => string): RuleGroupType {
  let replaced = false
  const rules = query.rules.map((rule) => {
    if (isQueryGroup(rule)) {
      return replaceRuleValue(rule, field, nextValue)
    }
    if (!replaced && rule.field === field && rule.operator === '=') {
      replaced = true
      return { ...rule, value: nextValue(String(rule.value || '')) }
    }
    return rule
  })

  if (replaced) {
    return { ...query, rules }
  }
  return {
    ...query,
    rules: [...rules, { field, operator: '=', value: nextValue('') }],
  }
}

function firstComparableRuleValue(query: RuleGroupType, field: string, operators: string[]): string {
  for (const rule of query.rules) {
    if (isQueryGroup(rule)) {
      const value = firstComparableRuleValue(rule, field, operators)
      if (value) {
        return value
      }
      continue
    }
    if (rule.field === field && operators.includes(rule.operator) && rule.value) {
      return String(rule.value)
    }
  }
  return ''
}

function firstEqualRuleValue(query: RuleGroupType, field: string): string {
  for (const rule of query.rules) {
    if (isQueryGroup(rule)) {
      const value = firstEqualRuleValue(rule, field)
      if (value) {
        return value
      }
      continue
    }
    if (rule.field === field && rule.operator === '=' && rule.value) {
      return String(rule.value)
    }
  }
  return ''
}

function countQueryRules(query: RuleGroupType): number {
  return query.rules.reduce((sum, rule) => sum + (isQueryGroup(rule) ? countQueryRules(rule) : 1), 0)
}

function isQueryGroup(rule: RuleGroupType['rules'][number]): rule is RuleGroupType {
  return Boolean(rule && typeof rule === 'object' && 'rules' in rule)
}

function PageHeader({ action, description, eyebrow, icon, title }: { action: React.ReactNode; description: string; eyebrow: string; icon: React.ReactNode; title: string }) {
  return (
    <header className="page-header">
      <div>
        <div className="eyebrow">{icon} {eyebrow}</div>
        <h1>{title}</h1>
        <p>{description}</p>
      </div>
      {action}
    </header>
  )
}

function MetricCard({ helper, icon, label, value }: { helper: string; icon: React.ReactNode; label: string; value: number }) {
  return (
    <Card className="metric-card">
      <div className="metric-icon">{icon}</div>
      <div>
        <span>{label}</span>
        <strong>{formatNumber(value)}</strong>
        <small>{helper}</small>
      </div>
    </Card>
  )
}

function SnapshotItem({ label, value }: { label: string; value: string }) {
  return (
    <div className="snapshot-item">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  )
}

function DataTable({ emptyLabel, headers, rows }: { emptyLabel: string; headers: string[]; rows: React.ReactNode[][] }) {
  return (
    <div className="table-wrap">
      <Table>
        <TableHeader>
          <TableRow>
            {headers.map((header) => <TableHead key={header}>{header}</TableHead>)}
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.length === 0 ? (
            <EmptyRow colSpan={headers.length} label={emptyLabel} />
          ) : rows.map((row, rowIndex) => (
            <TableRow key={rowIndex}>
              {row.map((cell, cellIndex) => <TableCell key={cellIndex}>{cell}</TableCell>)}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

function EmptyRow({ colSpan, label }: { colSpan: number; label: string }) {
  return (
    <TableRow>
      <TableCell className="empty" colSpan={colSpan}>{label}</TableCell>
    </TableRow>
  )
}

function Modal({ children, onClose, title }: { children: React.ReactNode; onClose: () => void; title: string }) {
  return (
    <div className="modal-backdrop" role="presentation">
      <section aria-modal="true" className="modal-panel" role="dialog">
        <div className="modal-header">
          <h2>{title}</h2>
          <Button onClick={onClose} size="sm" type="button" variant="ghost">Close</Button>
        </div>
        {children}
      </section>
    </div>
  )
}

function useInitialLoad(load: () => Promise<void>) {
  useEffect(() => {
    const id = window.setTimeout(() => {
      void load()
    }, 0)

    return () => window.clearTimeout(id)
  }, [load])
}

function parseMetadataSchema(value: string) {
  const parsed: unknown = JSON.parse(value || '{}')
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error('Metadata schema must be a JSON object')
  }
  return Object.fromEntries(Object.entries(parsed).map(([key, schemaValue]) => [key, String(schemaValue)]))
}

function parseJSONRecord(value: string, label: string) {
  const parsed: unknown = JSON.parse(value || '{}')
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error(`${label} must be a JSON object`)
  }
  return parsed as Record<string, unknown>
}

function localDateTimeToISO(value: string) {
  if (!value) {
    return ''
  }
  return new Date(value).toISOString()
}

function toInputDateTime(date: Date) {
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60000)
  return local.toISOString().slice(0, 16)
}

function defaultQueryDates() {
  const now = new Date()
  const from = new Date(now)
  from.setDate(now.getDate() - 7)
  return {
    from: toInputDateTime(from),
    to: toInputDateTime(now),
  }
}

function selectedMeterSchemaKeys(meters: Meter[], selectedMeterName?: string) {
  const selectedMeter = meters.find((meter) => meter.name === selectedMeterName)
  if (selectedMeter) {
    return Object.keys(selectedMeter.metadata_schema || {}).sort()
  }
  return Array.from(new Set(meters.flatMap((meter) => Object.keys(meter.metadata_schema || {})))).sort()
}

function sumMeterEvents(stats: Record<string, MeterStats>) {
  return Object.values(stats).reduce((sum, item) => sum + Number(item.usage_events || 0), 0)
}

function averageRetention(meters: Meter[]) {
  if (meters.length === 0) {
    return 0
  }
  return Math.round(meters.reduce((sum, meter) => sum + meter.event_retention_days, 0) / meters.length)
}

function formatNumber(value: number) {
  return new Intl.NumberFormat().format(value)
}

function formatDate(value?: string) {
  if (!value) {
    return 'Never'
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  return new Intl.DateTimeFormat(undefined, {
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    month: 'short',
  }).format(date)
}

export default App
