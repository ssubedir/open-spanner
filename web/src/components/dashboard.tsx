import type React from 'react'
import { useId } from 'react'
import { Loader2 } from 'lucide-react'

import { Card } from './ui/card'
import { Button } from './ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from './ui/table'
import { formatNumber } from '../lib/format'
import { cn } from '../lib/utils'

export function PageHeader({ action, description, eyebrow, icon, title }: { action: React.ReactNode; description: string; eyebrow: string; icon: React.ReactNode; title: string }) {
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

export function DetailLoadingPage({ title = 'Loading details' }: { action?: React.ReactNode; description?: string; eyebrow?: string; icon?: React.ReactNode; title?: string }) {
  return (
    <section aria-busy="true" aria-label={title} className="grid min-h-[calc(100vh-52px)] place-items-center">
      <Loader2 aria-hidden="true" className="h-8 w-8 animate-spin text-primary" />
    </section>
  )
}

export function DetailStatePage({ action, description, icon, title }: { action?: React.ReactNode; description: string; icon: React.ReactNode; title: string }) {
  return (
    <section className="grid min-h-[calc(100vh-52px)] place-items-center">
      <Card className="grid w-full max-w-[460px] justify-items-center gap-3 p-6 text-center">
        <span className="grid h-11 w-11 place-items-center rounded-md bg-secondary text-primary">{icon}</span>
        <div className="grid gap-2">
          <h1 className="text-xl font-semibold leading-tight">{title}</h1>
          <p className="text-sm text-muted">{description}</p>
        </div>
        {action ? <div className="mt-2 flex justify-center">{action}</div> : null}
      </Card>
    </section>
  )
}

export function MetricCard({ helper, icon, label, loading = false, value }: { helper: string; icon: React.ReactNode; label: string; loading?: boolean; value: number }) {
  return (
    <Card className="metric-card">
      <div className="metric-icon">{icon}</div>
      <div>
        <span>{label}</span>
        <strong aria-busy={loading}>
          {loading ? <Loader2 aria-label="Loading metric" className="metric-loading spin" /> : formatNumber(value)}
        </strong>
        <small>{helper}</small>
      </div>
    </Card>
  )
}

export function SnapshotItem({ label, value }: { label: string; value: string }) {
  return (
    <div className="snapshot-item">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  )
}

export function DataTable({
  className,
  emptyLabel,
  headers,
  rows,
  wrapClassName,
}: {
  className?: string
  emptyLabel: string
  headers: string[]
  rows: React.ReactNode[][]
  wrapClassName?: string
}) {
  return (
    <div className={cn('table-wrap', wrapClassName)}>
      <Table className={className}>
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

export function EmptyRow({ colSpan, label }: { colSpan: number; label: string }) {
  return (
    <TableRow>
      <TableCell className="empty" colSpan={colSpan}>{label}</TableCell>
    </TableRow>
  )
}

export function Modal({ children, className = '', onClose, title }: { children: React.ReactNode; className?: string; onClose: () => void; title: string }) {
  const titleID = useId()

  return (
    <div className="modal-backdrop" role="presentation">
      <section aria-labelledby={titleID} aria-modal="true" className={`modal-panel ${className}`.trim()} role="dialog">
        <div className="modal-header">
          <h2 id={titleID}>{title}</h2>
          <Button onClick={onClose} size="sm" type="button" variant="outline">Close</Button>
        </div>
        {children}
      </section>
    </div>
  )
}
