import { useSelector } from '@tanstack/react-store'
import { AlertTriangle, CheckCircle2, Clock3, FileArchive } from 'lucide-react'
import { useCallback, useEffect, useMemo, useState } from 'react'

import { appStore, appStoreActions } from '../app-store'
import type { UsageExportJob } from '../api'
import { ExportJobsCard } from '../components/export-jobs-card'
import { MetricCard, PageHeader } from '../components/dashboard'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { exportJobStatusLabel, isActiveExportJob } from '../lib/export-jobs'
import { useInitialLoad } from '../lib/hooks'

const exportJobPageSize = 50
const statusFilters = ['all', 'queued', 'running', 'completed', 'failed', 'canceled'] as const
type StatusFilter = typeof statusFilters[number]

export function ExportsPage() {
  const {
    exportJobDownloading,
    exportJobError,
    exportJobMutating,
    exportJobStatus,
    exportJobs,
  } = useSelector(appStore, (state) => state.usage)
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all')
  const load = useCallback(() => appStoreActions.loadUsageExportJobs(exportJobPageSize), [])
  const hasActiveExportJobs = useMemo(() => exportJobs.some(isActiveExportJob), [exportJobs])
  const filteredJobs = useMemo(
    () => statusFilter === 'all' ? exportJobs : exportJobs.filter((job) => job.status === statusFilter),
    [exportJobs, statusFilter],
  )
  const stats = useMemo(() => exportJobStats(exportJobs), [exportJobs])

  useInitialLoad(load)

  useEffect(() => {
    if (!hasActiveExportJobs) {
      return
    }

    const poll = window.setInterval(() => {
      void appStoreActions.loadUsageExportJobs(exportJobPageSize)
    }, 5000)
    return () => window.clearInterval(poll)
  }, [hasActiveExportJobs])

  return (
    <>
      <PageHeader
        eyebrow="Exports"
        icon={<FileArchive />}
        title="Export jobs"
        description="Track queued CSV exports, downloads, retries, and cancellations."
        action={null}
      />

      <section className="exports-grid">
        <MetricCard icon={<Clock3 />} label="Active" value={stats.active} helper="Queued or running" />
        <MetricCard icon={<CheckCircle2 />} label="Completed" value={stats.completed} helper="Ready files" />
        <MetricCard icon={<AlertTriangle />} label="Needs Action" value={stats.needsAction} helper="Failed or canceled" />
      </section>

      <div className="exports-filter-bar" aria-label="Export status filters">
        {statusFilters.map((status) => (
          <Button
            aria-pressed={statusFilter === status}
            key={status}
            onClick={() => setStatusFilter(status)}
            size="sm"
            type="button"
            variant={statusFilter === status ? 'secondary' : 'outline'}
          >
            {status === 'all' ? 'All' : exportJobStatusLabel(status)}
            <Badge variant={statusFilter === status ? 'default' : 'muted'}>{statusCount(exportJobs, status)}</Badge>
          </Button>
        ))}
      </div>

      <ExportJobsCard
        downloadingID={exportJobDownloading}
        emptyLabel={exportJobStatus === 'loading' ? 'Loading export jobs' : 'No export jobs match this filter.'}
        error={exportJobError}
        jobs={filteredJobs}
        mutatingID={exportJobMutating}
        status={exportJobStatus}
        title="Jobs"
      />
    </>
  )
}

function exportJobStats(jobs: UsageExportJob[]) {
  return jobs.reduce(
    (stats, job) => {
      if (job.status === 'queued' || job.status === 'running') {
        stats.active += 1
      }
      if (job.status === 'completed') {
        stats.completed += 1
      }
      if (job.status === 'failed' || job.status === 'canceled') {
        stats.needsAction += 1
      }
      return stats
    },
    { active: 0, completed: 0, needsAction: 0 },
  )
}

function statusCount(jobs: UsageExportJob[], status: StatusFilter) {
  if (status === 'all') {
    return jobs.length
  }
  return jobs.filter((job) => job.status === status).length
}
