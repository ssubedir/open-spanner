import { useRouter } from '@tanstack/react-router'
import { useSelector } from '@tanstack/react-store'
import { Activity, BarChart3, Boxes, Clock, Pin } from 'lucide-react'
import { useCallback } from 'react'

import { appStore, appStoreActions, type PinnedUsageQuerySummary } from '../app-store'
import { DataTable, MetricCard, PageHeader } from '../components/dashboard'
import { Badge } from '../components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { formatDate, formatNumber } from '../lib/format'
import { useInitialLoad } from '../lib/hooks'

export function OverviewPage() {
  const { error, ingestions, pinnedUsageQueries, stats, status, subjects } = useSelector(appStore, (state) => state.overview)
  const router = useRouter()
  const load = useCallback(() => appStoreActions.loadOverview(), [])

  useInitialLoad(load)

  const metricsLoading = !stats && (status === 'idle' || status === 'loading')
  const lastPrune = stats?.last_prune_run
  const lastPruneLabel = lastPrune?.dry_run ? 'Retention dry run' : lastPrune ? 'Retention cleanup' : 'Retention'
  const lastPruneHelper = lastPrune
    ? `${formatNumber(lastPrune.deleted)} deleted on ${formatDate(lastPrune.created_at)}`
    : 'No retention runs yet'

  function openPinnedQuery(summary: PinnedUsageQuerySummary) {
    appStoreActions.applySavedUsageQuery(summary.query)
    void router.navigate({ to: '/usage' })
  }

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

      <section className="metric-grid overview-metrics" aria-label="Operational metrics">
        <MetricCard icon={<Boxes />} label="Meters" loading={metricsLoading} value={stats?.meters ?? 0} helper="Configured billable signals" />
        <MetricCard icon={<BarChart3 />} label="Usage Events" loading={metricsLoading} value={stats?.usage_events ?? 0} helper="Raw events accepted" />
        <MetricCard icon={<Clock />} label={lastPruneLabel} loading={metricsLoading} value={lastPrune?.deleted ?? 0} helper={lastPruneHelper} />
      </section>

      {pinnedUsageQueries.length > 0 ? (
        <section className="pinned-query-grid overview-pinned-grid" aria-label="Pinned usage queries">
          {pinnedUsageQueries.map((summary) => (
            <PinnedQueryCard
              key={summary.query.id}
              onOpen={() => openPinnedQuery(summary)}
              summary={summary}
            />
          ))}
        </section>
      ) : null}

      <section className="overview-grid">
        <Card className="activity-card overview-card">
          <CardHeader className="overview-card-header">
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

        <Card className="activity-card overview-card">
          <CardHeader className="overview-card-header">
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

function PinnedQueryCard({ onOpen, summary }: { onOpen: () => void; summary: PinnedUsageQuerySummary }) {
  const groupBy = summary.query.group_by.length > 0 ? summary.query.group_by.join(', ') : 'Ungrouped'
  const footer = summary.error || (summary.lastBucket ? formatDate(summary.lastBucket) : 'No results')

  return (
    <button
      aria-label={`Open ${summary.query.name}`}
      className="card pinned-query-card"
      onClick={onOpen}
      type="button"
    >
      <div className="pinned-query-header">
        <span className="pinned-query-icon"><Pin aria-hidden="true" /></span>
        <div>
          <strong>{summary.query.name}</strong>
          <small>{groupBy}</small>
        </div>
        <Badge variant={summary.error ? 'warning' : 'muted'}>{summary.bucketSize}</Badge>
      </div>
      <div className="pinned-query-total">
        <strong>{summary.error ? '--' : formatNumber(summary.total)}</strong>
        <span>{summary.unit || 'units'}</span>
      </div>
      <div className="pinned-query-footer">
        <span>{footer}</span>
      </div>
    </button>
  )
}
