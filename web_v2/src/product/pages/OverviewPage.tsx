import { useRouter } from '@/compat/tanstack-router'
import { useSelector } from '@tanstack/react-store'
import { Activity, ArrowRight, BarChart3, Boxes, Clock, FileArchive, GaugeCircle, KeyRound, PackageCheck, Pin, Users } from 'lucide-react'
import type React from 'react'
import { useCallback } from 'react'

import { appStore, appStoreActions, type PinnedUsageQuerySummary } from '../app-store'
import type { IngestionRun, SystemStats } from '../api'
import { DataTable, MetricCard, PageHeader } from '../components/dashboard'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { formatDate, formatNumber } from '../lib/format'
import { formatBytes } from '../lib/export-jobs'
import { useInitialLoad } from '../lib/hooks'

export function OverviewPage() {
  const { alertDeliveryJobs, deadLetters, error, ingestions, pinnedUsageQueries, stats, status, subjects } = useSelector(appStore, (state) => state.overview)
  const router = useRouter()
  const load = useCallback(() => appStoreActions.loadOverview(), [])

  useInitialLoad(load)

  const metricsLoading = !stats && (status === 'idle' || status === 'loading')
  const lastPrune = stats?.last_prune_run
  const lastPruneLabel = lastPrune?.dry_run ? 'Retention dry run' : lastPrune ? 'Retention cleanup' : 'Retention'
  const lastPruneHelper = lastPrune
    ? `${formatNumber(lastPrune.deleted)} deleted on ${formatDate(lastPrune.created_at)}`
    : 'No retention runs yet'
  const latestIngestion = ingestions[0] ?? null
  const recentFailures = ingestions.reduce((sum, run) => sum + run.failed, 0)
  const recentAccepted = ingestions.reduce((sum, run) => sum + run.accepted, 0)

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
        description="Monitor workspace health, inspect usage, and jump into common metering workflows."
        action={null}
      />

      {error ? <div className="error-banner">{error}</div> : null}

      <section className="mb-4 grid gap-4 md:grid-cols-2 xl:grid-cols-4" aria-label="Operational metrics">
        <MetricCard icon={<Boxes />} label="Meters" loading={metricsLoading} value={stats?.meters ?? 0} helper="Configured billable signals" />
        <MetricCard icon={<BarChart3 />} label="Usage Events" loading={metricsLoading} value={stats?.usage_events ?? 0} helper="Raw events accepted" />
        <MetricCard icon={<Users />} label="Subjects" loading={metricsLoading} value={subjects.length} helper="Subjects with activity" />
        <MetricCard icon={<Clock />} label={lastPruneLabel} loading={metricsLoading} value={lastPrune?.deleted ?? 0} helper={lastPruneHelper} />
      </section>

      <section className="mb-4 grid gap-3 md:grid-cols-2 xl:grid-cols-4" aria-label="Quick actions">
        <OverviewActionCard
          description="Define the signals your backend can report."
          icon={<Boxes />}
          label="Meters"
          onOpen={() => void router.navigate({ to: '/meters' })}
          title="Create meter definitions"
        />
        <OverviewActionCard
          description="Explore buckets, filters, dimensions, and charts."
          icon={<BarChart3 />}
          label="Usage"
          onOpen={() => void router.navigate({ to: '/usage' })}
          title="Analyze usage"
        />
        <OverviewActionCard
          description="Attach subjects to quota packages."
          icon={<PackageCheck />}
          label="Plans"
          onOpen={() => void router.navigate({ to: '/plans' })}
          title="Manage entitlements"
        />
        <OverviewActionCard
          description="Issue credentials for trusted services."
          icon={<KeyRound />}
          label="API Keys"
          onOpen={() => void router.navigate({ to: '/api-keys' })}
          title="Connect an SDK"
        />
      </section>

      <Card className="mb-4 min-w-0">
        <CardHeader className="!px-4 !py-3"><div><CardTitle>Operations Health</CardTitle><CardDescription>Worker liveness, active work, and durable queue backlog. Backlogs older than five minutes are degraded.</CardDescription></div></CardHeader>
        <CardContent>
          <DataTable emptyLabel="Worker health is unavailable" headers={['Worker', 'Status', 'Pending', 'Running', 'Failed', 'Oldest pending', 'Last result', 'Last heartbeat']} rows={(stats?.worker_health ?? []).map((worker) => [
            <strong className="capitalize">{worker.name}</strong>,
            <Badge variant={worker.status === 'healthy' ? 'success' : worker.status === 'stale' || worker.status === 'degraded' ? 'warning' : 'muted'}>{worker.status.replace('_', ' ')}</Badge>,
            worker.pending_jobs,
            worker.running_jobs,
            worker.failed_jobs > 0 ? <strong>{worker.failed_jobs}</strong> : 0,
            worker.oldest_pending_at ? formatDate(worker.oldest_pending_at) : <span className="muted">—</span>,
            workerLastResult(worker),
            worker.last_heartbeat_at ? formatDate(worker.last_heartbeat_at) : <span className="muted">Never</span>,
          ])} />
        </CardContent>
      </Card>

      <Card className="mb-4 min-w-0">
        <CardHeader className="!px-4 !py-3"><div><CardTitle>Retention Rollup Coverage</CardTitle><CardDescription>Read-only verification that aggregate history is finalized through each meter retention cutoff.</CardDescription></div></CardHeader>
        <CardContent>
          <div className="mb-3 flex items-center gap-2">
            <Badge variant={stats?.rollup_health.status === 'healthy' ? 'success' : stats?.rollup_health.status === 'not_started' ? 'muted' : 'warning'}>{(stats?.rollup_health.status ?? 'not_started').replaceAll('_', ' ')}</Badge>
            <span className="text-sm text-muted">{formatNumber(stats?.rollup_health.healthy_meters ?? 0)} of {formatNumber(stats?.rollup_health.meters ?? 0)} meters covered{stats?.rollup_health.finalized_through ? ` · through ${formatDate(stats.rollup_health.finalized_through)}` : ''}</span>
          </div>
          <DataTable emptyLabel="No retention-enabled meters configured" headers={['Meter', 'Status', 'Finalized through', 'Expected through', 'Source events', 'Rollup rows', 'Details']} rows={(stats?.rollup_health.items ?? []).map((item) => [
            <strong>{item.meter}</strong>,
            <Badge variant={item.status === 'healthy' ? 'success' : item.status === 'not_started' ? 'muted' : 'warning'}>{item.status.replaceAll('_', ' ')}</Badge>,
            item.finalized_through ? formatDate(item.finalized_through) : <span className="muted">Never</span>,
            formatDate(item.expected_through),
            formatNumber(item.source_events),
            formatNumber(item.rollup_rows),
            item.issue || (item.last_run_at ? `Verified ${formatDate(item.last_run_at)}` : 'Ready'),
          ])} />
        </CardContent>
      </Card>

	  <Card className="mb-4 min-w-0">
		<CardHeader className="!px-4 !py-3"><div><CardTitle>Alert Delivery Outbox</CardTitle><CardDescription>Webhook notifications are retried with backoff and remain inspectable after terminal failure.</CardDescription></div></CardHeader>
		<CardContent>
		  <DataTable emptyLabel="No alert deliveries have been queued" headers={['Time', 'Event', 'Status', 'Attempts', 'Next attempt', 'Error', '']} rows={alertDeliveryJobs.map((item) => [
			formatDate(item.created_at),
			<span className="font-mono text-xs">{item.event_id}</span>,
			<Badge variant={item.status === 'delivered' ? 'success' : item.status === 'dead_letter' ? 'warning' : 'muted'}>{item.status.replace('_', ' ')}</Badge>,
			formatNumber(item.attempts),
			item.status === 'pending' ? formatDate(item.next_attempt_at) : <span className="muted">—</span>,
			<span className="max-w-[360px] truncate" title={item.last_error}>{item.last_error || '—'}</span>,
			item.status === 'dead_letter' ? <Button onClick={() => void appStoreActions.retryAlertDeliveryJob(item.id)} size="sm" type="button" variant="outline">Retry</Button> : <span className="muted">{item.delivered_at ? formatDate(item.delivered_at) : ''}</span>,
		  ])} />
		</CardContent>
	  </Card>

      <Card className="mb-4 min-w-0">
        <CardHeader className="!px-4 !py-3"><div><CardTitle>Worker Failure Audit</CardTitle><CardDescription>Terminal alert and entitlement failures remain here after retry for operational review.</CardDescription></div></CardHeader>
        <CardContent>
          <DataTable emptyLabel="No worker jobs have reached the dead-letter queue" headers={['Time', 'Worker', 'Job', 'Status', 'Attempts', 'Error', '']} rows={deadLetters.map((item) => [
            formatDate(item.created_at),
            <span className="capitalize">{item.worker_name}</span>,
            item.worker_name === 'entitlement' ? `${item.subject} / ${item.meter}` : item.rule_id,
            <Badge variant={item.status === 'dead_letter' ? 'warning' : 'muted'}>{item.status.replace('_', ' ')}</Badge>,
            formatNumber(item.attempts),
            <span className="max-w-[360px] truncate" title={item.last_error}>{item.last_error}</span>,
            item.status === 'dead_letter' ? <Button onClick={() => void appStoreActions.retryWorkerDeadLetter(item.id)} size="sm" type="button" variant="outline">Retry</Button> : <span className="muted">Retried {item.requeued_at ? formatDate(item.requeued_at) : ''}</span>,
          ])} />
        </CardContent>
      </Card>

      <section className="mb-4 grid gap-3 xl:grid-cols-[minmax(0,1.35fr)_minmax(340px,0.65fr)]">
        <Card className="min-w-0">
          <CardHeader className="!px-4 !py-3">
            <div>
              <CardTitle>Pinned Usage Queries</CardTitle>
              <CardDescription>Fast access to the charts and breakdowns you watch most.</CardDescription>
            </div>
          </CardHeader>
          <CardContent className="!p-3">
            {pinnedUsageQueries.length > 0 ? (
              <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
                {pinnedUsageQueries.map((summary) => (
                  <PinnedQueryCard
                    key={summary.query.id}
                    onOpen={() => openPinnedQuery(summary)}
                    summary={summary}
                  />
                ))}
              </div>
            ) : (
              <EmptyPinnedQueries onOpen={() => void router.navigate({ to: '/usage' })} />
            )}
          </CardContent>
        </Card>

        <WorkspaceHealthCard
          latestIngestion={latestIngestion}
          recentAccepted={recentAccepted}
          recentFailures={recentFailures}
          stats={stats}
          subjects={subjects.length}
        />
      </section>

      <section className="grid gap-3 xl:grid-cols-2">
        <Card className="min-w-0">
          <CardHeader className="!px-4 !py-3">
            <div>
              <CardTitle>Subjects</CardTitle>
              <CardDescription>Highest recent subject activity.</CardDescription>
            </div>
          </CardHeader>
          <CardContent>
            <DataTable
              className="!min-w-0"
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

        <Card className="min-w-0">
          <CardHeader className="!px-4 !py-3">
            <div>
              <CardTitle>Ingestion History</CardTitle>
              <CardDescription>Recent single and bulk ingestion runs.</CardDescription>
            </div>
          </CardHeader>
          <CardContent>
            <DataTable
              className="!min-w-0"
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

function workerLastResult(worker: SystemStats['worker_health'][number]) {
  if (worker.last_failure_at && (!worker.last_success_at || Date.parse(worker.last_failure_at) >= Date.parse(worker.last_success_at))) {
    return <span><strong>Failed</strong> {formatDate(worker.last_failure_at)}</span>
  }
  if (worker.last_success_at) {
    return <span>Completed {formatDate(worker.last_success_at)}</span>
  }
  return <span className="muted">—</span>
}

function OverviewActionCard({ description, icon, label, onOpen, title }: { description: string; icon: React.ReactNode; label: string; onOpen: () => void; title: string }) {
  return (
    <button
      aria-label={title}
      className="card grid cursor-pointer grid-cols-[38px_minmax(0,1fr)_auto] items-center gap-3 p-3 text-left transition hover:border-input focus-visible:outline-3 focus-visible:outline-offset-2 focus-visible:outline-ring"
      onClick={onOpen}
      type="button"
    >
      <span className="grid size-9 place-items-center rounded-md bg-[#e6f6f3] text-primary">{icon}</span>
      <span className="grid min-w-0 gap-0.5">
        <small className="text-xs font-bold uppercase text-muted">{label}</small>
        <strong className="truncate text-sm">{title}</strong>
        <span className="truncate text-xs text-muted">{description}</span>
      </span>
      <ArrowRight aria-hidden="true" className="size-4 text-muted" />
    </button>
  )
}

function EmptyPinnedQueries({ onOpen }: { onOpen: () => void }) {
  return (
    <div className="grid min-h-[132px] place-items-center rounded-md border border-dashed border-border bg-[#f8fafc] p-6 text-center">
      <div className="grid max-w-[420px] gap-3 justify-items-center">
        <span className="grid size-10 place-items-center rounded-md bg-[#e6f6f3] text-primary"><Pin aria-hidden="true" className="size-5" /></span>
        <div className="grid gap-1">
          <strong>No pinned queries yet</strong>
          <span className="text-sm text-muted">Save and pin a usage query to keep important product signals on this page.</span>
        </div>
        <Button onClick={onOpen} size="sm" type="button" variant="outline">
          Open usage
          <ArrowRight aria-hidden="true" />
        </Button>
      </div>
    </div>
  )
}

function WorkspaceHealthCard({ latestIngestion, recentAccepted, recentFailures, stats, subjects }: {
  latestIngestion: IngestionRun | null
  recentAccepted: number
  recentFailures: number
  stats: SystemStats | null
  subjects: number
}) {
  return (
    <Card className="min-w-0">
      <CardHeader className="!px-4 !py-3">
        <div>
          <CardTitle>Workspace Health</CardTitle>
          <CardDescription>Current setup and ingestion signal.</CardDescription>
        </div>
      </CardHeader>
      <CardContent className="grid gap-2 !p-3">
        <OverviewStatusItem
          icon={<Boxes />}
          label="Model"
          value={`${formatNumber(stats?.meters ?? 0)} meters, ${formatNumber(subjects)} subjects`}
          variant={(stats?.meters ?? 0) > 0 ? 'success' : 'muted'}
        />
        <OverviewStatusItem
          icon={<GaugeCircle />}
          label="Ingestion safety"
          value={stats ? `${formatNumber(stats.ingestion_safety.accepted_events)} accepted, ${formatNumber(stats.ingestion_safety.rejected_events)} rejected, ${formatNumber(stats.ingestion_safety.throttled_events)} throttled` : latestIngestion ? `${formatNumber(recentAccepted)} accepted, ${formatNumber(recentFailures)} failed` : 'No ingestion runs'}
          variant={(stats?.ingestion_safety.throttled_events ?? 0) > 0 || recentFailures > 0 ? 'warning' : latestIngestion ? 'success' : 'muted'}
        />
        <OverviewStatusItem
          icon={<Clock />}
          label="Retention"
          value={stats?.last_export_cleanup_run ? `${formatDate(stats.last_export_cleanup_run.created_at)} · ${formatBytes(stats.last_export_cleanup_run.bytes_reclaimed)} reclaimed` : stats?.last_decision_prune_run ? `${formatDate(stats.last_decision_prune_run.created_at)} · ${formatNumber(stats.consumption_decisions)} decisions` : stats?.last_prune_run ? formatDate(stats.last_prune_run.created_at) : 'No cleanup runs yet'}
          variant={stats?.last_export_cleanup_run?.failures ? 'warning' : stats?.last_export_cleanup_run || stats?.last_decision_prune_run || stats?.last_prune_run ? 'success' : 'muted'}
        />
        <OverviewStatusItem
          icon={<FileArchive />}
          label="Raw events"
          value={`${formatNumber(stats?.usage_events ?? 0)} ${pluralize(stats?.usage_events ?? 0, 'stored event', 'stored events')}`}
          variant={(stats?.usage_events ?? 0) > 0 ? 'success' : 'muted'}
        />
      </CardContent>
    </Card>
  )
}

function OverviewStatusItem({ icon, label, value, variant }: { icon: React.ReactNode; label: string; value: string; variant: 'muted' | 'success' | 'warning' }) {
  return (
    <div className="grid grid-cols-[34px_minmax(0,1fr)_auto] items-center gap-3 rounded-md border border-border bg-[#f8fafc] p-3">
      <span className="grid size-8 place-items-center rounded-md bg-white text-primary">{icon}</span>
      <span className="grid min-w-0 gap-0.5">
        <strong className="truncate text-sm">{label}</strong>
        <small className="truncate text-xs text-muted">{value}</small>
      </span>
      <Badge variant={variant}>{statusLabel(variant)}</Badge>
    </div>
  )
}

function pluralize(count: number, singular: string, plural: string) {
  return count === 1 ? singular : plural
}

function statusLabel(variant: 'muted' | 'success' | 'warning') {
  if (variant === 'success') {
    return 'Ready'
  }
  if (variant === 'warning') {
    return 'Check'
  }
  return 'Idle'
}

function PinnedQueryCard({ onOpen, summary }: { onOpen: () => void; summary: PinnedUsageQuerySummary }) {
  const groupBy = summary.query.group_by.length > 0 ? summary.query.group_by.join(', ') : 'Ungrouped'
  const footer = summary.error || (summary.lastBucket ? formatDate(summary.lastBucket) : 'No results')

  return (
    <button
      aria-label={`Open ${summary.query.name}`}
      className="card grid min-h-[132px] cursor-pointer gap-3 border-border p-3 text-left transition hover:border-input focus-visible:outline-3 focus-visible:outline-offset-2 focus-visible:outline-ring"
      onClick={onOpen}
      type="button"
    >
      <div className="grid grid-cols-[28px_minmax(0,1fr)_auto] items-start gap-2">
        <span className="grid size-7 place-items-center rounded-md bg-[#e6f6f3] text-primary"><Pin aria-hidden="true" className="size-4" /></span>
        <div className="grid min-w-0 gap-0.5">
          <strong className="truncate text-sm">{summary.query.name}</strong>
          <small className="truncate text-xs text-muted">{groupBy}</small>
        </div>
        <Badge variant={summary.error ? 'warning' : 'muted'}>{summary.bucketSize}</Badge>
      </div>
      <div className="grid gap-0.5">
        <strong className="text-2xl leading-none">{summary.error ? '--' : formatNumber(summary.total)}</strong>
        <span className="text-xs text-muted">{summary.unit || 'units'}</span>
      </div>
      <div className="truncate text-xs text-muted">
        <span>{footer}</span>
      </div>
    </button>
  )
}
