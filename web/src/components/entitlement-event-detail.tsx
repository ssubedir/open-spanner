import { Copy } from 'lucide-react'

import type { EntitlementEvent } from '../api'
import { formatDate, formatNumber } from '../lib/format'
import { Badge } from './ui/badge'
import { Button } from './ui/button'

export function EntitlementEventDetail({ event }: { event: EntitlementEvent }) {
  const payload = JSON.stringify({ event }, null, 2)
  return (
    <div className="grid max-h-[calc(100vh-180px)] gap-4 overflow-auto p-4">
      <section className="border-b border-border pb-4">
        <div className="flex items-start justify-between gap-4">
          <div className="grid min-w-0 gap-2">
            <EntitlementStateBadge state={event.state} />
            <h3 className="text-lg font-semibold leading-tight">{event.plan_name} entitlement changed</h3>
            <p className="text-sm text-muted">{event.message || 'No entitlement message was recorded.'}</p>
          </div>
          <span className="shrink-0 text-xs font-bold text-muted">{formatDate(event.created_at)}</span>
        </div>
      </section>

      <section className="grid gap-2 md:grid-cols-3" aria-label="Entitlement event details">
        <DetailItem label="Subject" value={event.subject} mono />
        <DetailItem label="Meter" value={event.meter} mono />
        <DetailItem label="Plan" value={event.plan_name} />
        <DetailItem label="State" value={event.state} />
        <DetailItem label="Previous" value={event.previous_state || 'none'} />
        <DetailItem label="Period" value={event.period} />
        <DetailItem label="Current" value={formatNumber(event.current)} />
        <DetailItem label="Limit" value={formatNumber(event.limit)} />
        <DetailItem label="Remaining" value={formatNumber(event.remaining)} />
        <DetailItem label="Warning" value={`${formatNumber(event.warning_percent)}%`} />
        <DetailItem label="Plan ID" value={event.plan_id} mono wide />
        <DetailItem label="Event ID" value={event.id} mono wide />
      </section>

      <section className="grid gap-3">
        <div className="flex items-center justify-between gap-3">
          <div className="grid gap-1">
            <h3 className="text-base font-semibold leading-tight">Event JSON</h3>
            <p className="text-sm text-muted">Recorded entitlement transition payload.</p>
          </div>
          <Button onClick={() => void copyText(payload)} type="button" variant="outline">
            <Copy aria-hidden="true" />
            Copy
          </Button>
        </div>
        <pre className="max-h-[260px] overflow-auto rounded-md border border-border bg-[#f8fafc] p-3 font-mono text-sm leading-6 text-foreground">{payload}</pre>
      </section>
    </div>
  )
}

export function EntitlementEventType({ event }: { event: EntitlementEvent }) {
  return (
    <span>
      <EntitlementStateBadge state={event.state} />
      <small className="muted block">{event.previous_state ? `${event.previous_state} -> ${event.state}` : event.type}</small>
    </span>
  )
}

export function EntitlementStateBadge({ state }: { state: string }) {
  if (state === 'exceeded') {
    return <Badge variant="warning">Exceeded</Badge>
  }
  if (state === 'warning') {
    return <Badge variant="warning">Warning</Badge>
  }
  return <Badge variant="success">OK</Badge>
}

function DetailItem({ label, mono = false, value, wide = false }: { label: string; mono?: boolean; value: string; wide?: boolean }) {
  return (
    <div className={wide ? 'grid gap-1 rounded-md border border-border bg-[#f8fafc] p-3 md:col-span-3' : 'grid gap-1 rounded-md border border-border bg-[#f8fafc] p-3'}>
      <span className="text-xs font-bold text-muted">{label}</span>
      <strong className={mono ? 'mono min-w-0 break-all text-sm' : 'min-w-0 break-words text-sm'}>{value}</strong>
    </div>
  )
}

async function copyText(value: string) {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(value)
    return
  }
  const textarea = document.createElement('textarea')
  textarea.value = value
  textarea.setAttribute('readonly', 'true')
  textarea.style.left = '-9999px'
  textarea.style.position = 'fixed'
  document.body.appendChild(textarea)
  textarea.select()
  document.execCommand('copy')
  textarea.remove()
}
