import { Ban, Download, Loader2, RotateCcw } from 'lucide-react'

import { appStoreActions } from '../app-store'
import type { UsageExportJob } from '../api'
import { exportJobKindLabel, exportJobStatusLabel, exportJobStatusVariant, formatBytes } from '../lib/export-jobs'
import { formatDate } from '../lib/format'
import { Badge } from './ui/badge'
import { Button } from './ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from './ui/card'

export function ExportJobsCard({
  downloadingID,
  emptyLabel = 'Queued exports will appear here.',
  error,
  jobs,
  mutatingID,
  status,
  title = 'Export Jobs',
}: {
  downloadingID: string
  emptyLabel?: string
  error: string
  jobs: UsageExportJob[]
  mutatingID: string
  status: string
  title?: string
}) {
  const label = status === 'loading' && jobs.length === 0 ? 'Loading' : `${jobs.length} jobs`

  return (
    <Card className="usage-export-card">
      <CardHeader className="usage-card-header">
        <div>
          <CardTitle>{title}</CardTitle>
          <CardDescription>Queued CSV exports handled by the worker.</CardDescription>
        </div>
        <Badge variant={jobs.length > 0 ? 'success' : 'muted'}>{label}</Badge>
      </CardHeader>
      <CardContent className="export-jobs-content">
        {error ? <div className="inline-error">{error}</div> : null}
        {jobs.length > 0 ? (
          <div className="export-job-list">
            {jobs.map((job) => (
              <article className="export-job-row" key={job.id}>
                <ExportJobSummary job={job} />
                <ExportJobActions downloadingID={downloadingID} job={job} mutatingID={mutatingID} />
              </article>
            ))}
          </div>
        ) : (
          <div className="breakdown-empty">{emptyLabel}</div>
        )}
      </CardContent>
    </Card>
  )
}

export function ExportJobSummary({ job }: { job: UsageExportJob }) {
  return (
    <div className="export-job-main">
      <div className="export-job-title">
        <strong>{exportJobKindLabel(job.kind)}</strong>
        <Badge variant={exportJobStatusVariant(job.status)}>{exportJobStatusLabel(job.status)}</Badge>
      </div>
      <div className="export-job-meta">
        <span>{job.query.meter}</span>
        <span>{job.query.bucket_size}</span>
        <span>{job.query.group_by?.length ? `${job.query.group_by.length} groups` : 'no groups'}</span>
        <span>{formatDate(job.created_at)}</span>
        {job.artifact_size ? <span>{formatBytes(job.artifact_size)}</span> : null}
      </div>
      {job.error ? <p className="export-job-error">{job.error}</p> : null}
    </div>
  )
}

export function ExportJobActions({
  downloadingID,
  job,
  mutatingID,
}: {
  downloadingID: string
  job: UsageExportJob
  mutatingID: string
}) {
  return (
    <div className="export-job-actions">
      {job.status === 'completed' ? (
        <Button
          disabled={downloadingID === job.id}
          onClick={() => void appStoreActions.downloadUsageExport(job)}
          size="sm"
          type="button"
          variant="outline"
        >
          {downloadingID === job.id ? <Loader2 className="spin" aria-hidden="true" /> : <Download aria-hidden="true" />}
          Download
        </Button>
      ) : null}
      {job.status === 'queued' || job.status === 'running' ? (
        <Button
          disabled={mutatingID === job.id}
          onClick={() => void appStoreActions.cancelUsageExport(job)}
          size="sm"
          type="button"
          variant="outline"
        >
          {mutatingID === job.id ? <Loader2 className="spin" aria-hidden="true" /> : <Ban aria-hidden="true" />}
          Cancel
        </Button>
      ) : null}
      {job.status === 'failed' || job.status === 'canceled' ? (
        <Button
          disabled={mutatingID === job.id}
          onClick={() => void appStoreActions.retryUsageExport(job)}
          size="sm"
          type="button"
          variant="secondary"
        >
          {mutatingID === job.id ? <Loader2 className="spin" aria-hidden="true" /> : <RotateCcw aria-hidden="true" />}
          Retry
        </Button>
      ) : null}
    </div>
  )
}
