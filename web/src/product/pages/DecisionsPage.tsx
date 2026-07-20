import { ShieldCheck } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'

import { listConsumptionDecisions, type ConsumptionDecision } from '../api'
import { DataTable, Modal, PageHeader } from '../components/dashboard'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { Input } from '../components/ui/input'
import { formatDate, formatNumber } from '../lib/format'

export function DecisionsPage() {
	const [items, setItems] = useState<ConsumptionDecision[]>([])
	const [cursor, setCursor] = useState('')
	const [subject, setSubject] = useState('')
	const [meter, setMeter] = useState('')
	const [outcome, setOutcome] = useState('all')
	const [loading, setLoading] = useState(false)
	const [error, setError] = useState('')
	const [selected, setSelected] = useState<ConsumptionDecision | null>(null)

	const load = useCallback(async (next = '') => {
		setLoading(true)
		setError('')
		try {
			const result = await listConsumptionDecisions({ subject, meter, outcome, limit: 50, cursor: next })
			setItems((current) => next ? [...current, ...result.items] : result.items)
			setCursor(result.next_cursor ?? '')
		} catch (cause) {
			setError(cause instanceof Error ? cause.message : 'Unable to load decisions')
		} finally {
			setLoading(false)
		}
	}, [meter, outcome, subject])

	useEffect(() => { void load() }, [load])

	return (
		<>
			<PageHeader eyebrow="Entitlements" icon={<ShieldCheck />} title="Consumption decisions" description="Investigate accepted and rejected quota decisions without exposing event metadata." action={<Button disabled={loading} onClick={() => void load()} type="button" variant="outline">Refresh</Button>} />
			<Card className="mb-4 max-w-[1480px]">
				<CardHeader className="!px-4 !py-3">
					<div>
						<CardTitle>Filters</CardTitle>
						<CardDescription>Narrow the audit trail by subject, meter, or outcome.</CardDescription>
					</div>
				</CardHeader>
				<CardContent className="!p-3">
					<div className="grid gap-3 md:grid-cols-[1fr_1fr_180px_auto]">
						<Input aria-label="Filter by subject" onChange={(event) => setSubject(event.target.value)} placeholder="Subject" value={subject} />
						<Input aria-label="Filter by meter" onChange={(event) => setMeter(event.target.value)} placeholder="Meter" value={meter} />
						<select className="h-9 rounded-md border border-input bg-card px-3 text-sm" onChange={(event) => setOutcome(event.target.value)} value={outcome}>
							<option value="all">All outcomes</option><option value="accepted">Accepted</option><option value="rejected">Rejected</option>
						</select>
						<Button disabled={loading} onClick={() => void load()} type="button">Apply</Button>
					</div>
				</CardContent>
			</Card>

			{error ? <div className="error-banner">{error}</div> : null}

			<Card className="max-w-[1480px]">
				<CardHeader className="!px-4 !py-3">
					<div>
						<CardTitle>Decision log</CardTitle>
						<CardDescription>Accepted and rejected atomic consumption attempts.</CardDescription>
					</div>
				</CardHeader>
				<CardContent>
					<DataTable emptyLabel={loading ? 'Loading decisions' : 'No decisions match these filters.'} headers={['Time', 'Outcome', 'Subject', 'Meter', 'State', 'Projected', 'Key', '']} rows={items.map((item) => [
						formatDate(item.created_at), <Badge variant={item.accepted ? 'success' : 'warning'}>{item.accepted ? 'Accepted' : 'Rejected'}</Badge>,
						item.quota.subject, item.quota.meter, item.quota.state, `${formatNumber(item.quota.projected)} / ${formatNumber(item.quota.limit)}`,
						<span className="font-mono text-xs">{item.idempotency_key}</span>, <Button onClick={() => setSelected(item)} size="sm" type="button" variant="outline">Inspect</Button>,
					])} />
					{cursor ? <div className="pagination-actions"><Button disabled={loading} onClick={() => void load(cursor)} type="button" variant="outline">Load more</Button></div> : null}
				</CardContent>
			</Card>
			{selected ? <DecisionModal decision={selected} onClose={() => setSelected(null)} /> : null}
		</>
	)
}

function DecisionModal({ decision, onClose }: { decision: ConsumptionDecision; onClose: () => void }) {
	return <Modal title="Consumption decision" onClose={onClose}><dl className="grid gap-3 text-sm sm:grid-cols-2">
		{[
			['Idempotency key', decision.idempotency_key], ['Outcome', decision.accepted ? 'Accepted' : 'Rejected'],
			['Subject', decision.quota.subject], ['Meter', decision.quota.meter], ['Enforcement', decision.quota.enforcement],
			['Failure policy', decision.quota.failure_policy], ['State', decision.quota.state], ['Quantity', formatNumber(decision.quota.quantity)],
			['Current', formatNumber(decision.quota.current)], ['Projected', formatNumber(decision.quota.projected)],
			['Limit', formatNumber(decision.quota.limit)], ['Created', formatDate(decision.created_at)],
		].map(([label, value]) => <div className="rounded-md border bg-secondary p-3" key={label}><dt className="text-muted-foreground">{label}</dt><dd className="mt-1 break-all font-medium">{value}</dd></div>)}
	</dl></Modal>
}
