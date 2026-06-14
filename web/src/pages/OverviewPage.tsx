import { Link } from '@tanstack/react-router'
import { Activity, BarChart3, Boxes, CheckCircle2, Clock, Database, Loader2, RefreshCw, Rows3 } from 'lucide-react'
import { useCallback, useMemo, useState } from 'react'

import { getSystemStats, listIngestions, listSubjects, type IngestionRun, type SubjectStats, type SystemStats } from '../api'
import { DataTable, MetricCard, PageHeader, SnapshotItem } from '../components/dashboard'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { formatDate, formatNumber } from '../lib/format'
import { useInitialLoad } from '../lib/hooks'
import type { LoadState } from '../types'

export function OverviewPage() {
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
