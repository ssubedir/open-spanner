import type { UsageExportJob } from '../api'
import { formatNumber } from './format'

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

export function formatBytes(value: number) {
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
