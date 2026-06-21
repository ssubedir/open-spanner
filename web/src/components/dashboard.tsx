import type React from 'react'
import { useId } from 'react'
import { Loader2 } from 'lucide-react'

import { Card } from './ui/card'
import { Button } from './ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from './ui/table'
import { formatNumber } from '../lib/format'

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

export function DataTable({ emptyLabel, headers, rows }: { emptyLabel: string; headers: string[]; rows: React.ReactNode[][] }) {
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
          <Button onClick={onClose} size="sm" type="button" variant="ghost">Close</Button>
        </div>
        {children}
      </section>
    </div>
  )
}
