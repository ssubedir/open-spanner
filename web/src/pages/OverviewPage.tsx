import { useSelector } from '@tanstack/react-store'
import { Activity, BarChart3, Boxes, Clock } from 'lucide-react'
import { useCallback } from 'react'

import { appStore, appStoreActions } from '../app-store'
import { DataTable, MetricCard, PageHeader } from '../components/dashboard'
import { Badge } from '../components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { formatDate, formatNumber } from '../lib/format'
import { useInitialLoad } from '../lib/hooks'

export function OverviewPage() {
  const { error, ingestions, stats, subjects } = useSelector(appStore, (state) => state.overview)
  const load = useCallback(() => appStoreActions.loadOverview(), [])

  useInitialLoad(load)

  const lastPrune = stats?.last_prune_run
  const lastPruneLabel = lastPrune?.dry_run ? 'Retention dry run' : lastPrune ? 'Retention cleanup' : 'Retention'
  const lastPruneHelper = lastPrune
    ? `${formatNumber(lastPrune.deleted)} deleted on ${formatDate(lastPrune.created_at)}`
    : 'No retention runs yet'

  return (
    <>
      <PageHeader
        eyebrow="Overview"
        icon={<Activity />}
        title="Metering operations"
        description="Monitor core usage activity, recent ingestion, and subject volume."
        action={null}
      />

      {error ? <div className="error-banner">{error}</div> : null}

      <section className="metric-grid" aria-label="Operational metrics">
        <MetricCard icon={<Boxes />} label="Meters" value={stats?.meters ?? 0} helper="Configured billable signals" />
        <MetricCard icon={<BarChart3 />} label="Usage Events" value={stats?.usage_events ?? 0} helper="Raw events accepted" />
        <MetricCard icon={<Clock />} label={lastPruneLabel} value={lastPrune?.deleted ?? 0} helper={lastPruneHelper} />
      </section>

      <section className="content-grid">
        <Card className="activity-card span">
          <CardHeader>
            <div>
              <CardTitle>Subjects</CardTitle>
              <CardDescription>Highest recent subject activity.</CardDescription>
            </div>
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

        <Card className="activity-card span">
          <CardHeader>
            <div>
              <CardTitle>Ingestion History</CardTitle>
              <CardDescription>Recent single and bulk ingestion runs.</CardDescription>
            </div>
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
