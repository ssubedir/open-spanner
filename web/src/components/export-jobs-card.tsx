import { Ban, Download, Loader2, RotateCcw } from 'lucide-react'

import { appStoreActions } from '../app-store'
import type { UsageExportJob } from '../api'
import { formatDate, formatNumber } from '../lib/format'
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

export function isActiveExportJob(job: UsageExportJob) {
  return job.status === 'queued' || job.status === 'running'
}

export function exportJobStatusLabel(status: string) {
  return humanizeField(status)
}

export function exportJobStatusVariant(status: string): 'default' | 'muted' | 'success' | 'warning' {
  if (status === 'completed') {
    return 'success'
  }
  if (status === 'failed' || status === 'canceled') {
    return 'warning'
  }
  if (status === 'running') {
    return 'default'
  }
  return 'muted'
}

export function exportJobKindLabel(kind: string) {
  if (kind === 'usage_buckets') {
    return 'Usage buckets'
  }
  return humanizeField(kind)
}

export function formatExportJobBytes(value: number) {
  return formatBytes(value)
}

function formatBytes(value: number) {
  if (value < 1024) {
    return `${formatNumber(value)} B`
  }
  if (value < 1024 * 1024) {
    return `${formatNumber(Math.round(value / 102.4) / 10)} KB`
  }
  return `${formatNumber(Math.round(value / 104857.6) / 10)} MB`
}

function humanizeField(key: string) {
  return key
    .replace(/^metadata\./, '')
    .split(/[._-]/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ')
}
