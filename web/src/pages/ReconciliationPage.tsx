import { ScanSearch } from 'lucide-react'
import { useCallback, useState } from 'react'

import { listQuotaCounterRepairs, listReconciliationRuns, reconcileQuotaRecords, repairQuotaCounter, type CounterRepair, type ReconciliationIssue, type ReconciliationResult, type ReconciliationRun } from '../api'
import { DataTable, MetricCard, Modal, PageHeader } from '../components/dashboard'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { formatDate, formatNumber } from '../lib/format'
import { useInitialLoad } from '../lib/hooks'

export function ReconciliationPage() {
	const [result, setResult] = useState<ReconciliationResult | null>(null)
	const [loading, setLoading] = useState(false)
	const [error, setError] = useState('')
	const [preview, setPreview] = useState<CounterRepair | null>(null)
	const [repairRuns, setRepairRuns] = useState<CounterRepair[]>([])
	const [scheduledRuns, setScheduledRuns] = useState<ReconciliationRun[]>([])
	const load = useCallback(async () => {
		setLoading(true)
		setError('')
		try {
			const [scan, repairs, scheduled] = await Promise.all([reconcileQuotaRecords(), listQuotaCounterRepairs(), listReconciliationRuns()])
			setResult(scan); setRepairRuns(repairs.items); setScheduledRuns(scheduled.items)
		}
		catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to reconcile quota records') }
		finally { setLoading(false) }
	}, [])
	useInitialLoad(load)

	async function previewRepair(issue: ReconciliationIssue) {
		if (!issue.subject || !issue.meter || !issue.period || !issue.period_start) return
		setLoading(true); setError('')
		try { setPreview(await repairQuotaCounter({ subject: issue.subject, meter: issue.meter, period: issue.period, period_start: issue.period_start, dry_run: true })) }
		catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to preview repair') }
		finally { setLoading(false) }
	}

	async function applyRepair() {
		if (!preview) return
		setLoading(true); setError('')
		try {
			await repairQuotaCounter({ subject: preview.subject, meter: preview.meter, period: preview.period, period_start: preview.period_start, dry_run: false, expected_updated_at: preview.counter_updated_at })
			setPreview(null); await load()
		} catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to apply repair') }
		finally { setLoading(false) }
	}

	return <>
		<PageHeader eyebrow="Operations" icon={<ScanSearch />} title="Quota reconciliation" description="Compare recent consumption decisions and active quota counters with source usage. This scan never modifies data." action={<Button disabled={loading} onClick={() => void load()} type="button">{loading ? 'Scanning…' : 'Run scan'}</Button>} />
		{error ? <div className="error-banner">{error}</div> : null}
		<section className="mb-4 grid gap-4 md:grid-cols-3">
			<MetricCard icon={<ScanSearch />} label="Issues" loading={loading && !result} value={result?.issues.length ?? 0} helper={result ? `${result.status === 'healthy' ? 'Healthy' : 'Drift detected'} · checked ${formatDate(result.checked_at)}` : 'Run a read-only scan'} />
			<MetricCard icon={<ScanSearch />} label="Decisions checked" loading={loading && !result} value={result?.decisions_checked ?? 0} helper={`${result?.lookback_hours ?? 24}-hour lookback`} />
			<MetricCard icon={<ScanSearch />} label="Active counters" loading={loading && !result} value={result?.counters_checked ?? 0} helper={result?.truncated ? 'Result was bounded; narrow the scan' : 'Current quota periods'} />
		</section>
		{result ? <div className="mb-3 flex items-center gap-2"><Badge variant={result.status === 'healthy' ? 'success' : 'warning'}>{result.status === 'healthy' ? 'No drift found' : `${formatNumber(result.issues.length)} issues`}</Badge><span className="text-sm text-muted">Read-only result</span></div> : null}
		<DataTable emptyLabel={loading ? 'Running reconciliation scan' : 'No quota inconsistencies detected.'} headers={['Severity', 'Issue', 'Subject', 'Meter', 'Expected', 'Actual', 'Details', '']} rows={(result?.issues ?? []).map((issue) => [
			<Badge variant="warning">{issue.severity}</Badge>, <span className="font-mono text-xs">{issue.kind}</span>, issue.subject ?? '—', issue.meter ?? '—', issue.expected, issue.actual, issue.message,
			issue.kind.startsWith('counter_') ? <Button disabled={loading} onClick={() => void previewRepair(issue)} size="sm" type="button" variant="outline">Preview repair</Button> : <span className="text-xs text-muted">Manual review</span>,
		])} />
		<h2 className="mb-3 mt-6 text-lg font-semibold">Scheduled scan history</h2>
		<DataTable emptyLabel="No scheduled reconciliation runs yet." headers={['Time', 'Status', 'Issues', 'Decisions', 'Counters', 'Duration', 'Details']} rows={scheduledRuns.map((run) => [
			formatDate(run.created_at), <Badge variant={run.status === 'healthy' ? 'success' : 'warning'}>{run.status.replaceAll('_', ' ')}</Badge>, formatNumber(run.issue_count), formatNumber(run.decisions_checked), formatNumber(run.counters_checked), `${formatNumber(run.duration_ms)} ms`, run.error || (run.truncated ? 'Bounded result' : 'Complete bounded scan'),
		])} />
		<h2 className="mb-3 mt-6 text-lg font-semibold">Repair history</h2>
		<DataTable emptyLabel="No repair previews or applications yet." headers={['Time', 'Mode', 'Subject', 'Meter', 'Period', 'Events before', 'Events after']} rows={repairRuns.map((run) => [
			formatDate(run.created_at), <Badge variant={run.applied ? 'success' : 'muted'}>{run.applied ? 'Applied' : 'Preview'}</Badge>, run.subject, run.meter, run.period, formatNumber(run.before.event_count), formatNumber(run.after.event_count),
		])} />
		{preview ? <RepairPreviewModal loading={loading} onApply={() => void applyRepair()} onClose={() => setPreview(null)} preview={preview} /> : null}
	</>
}

function RepairPreviewModal({ loading, onApply, onClose, preview }: { loading: boolean; onApply: () => void; onClose: () => void; preview: CounterRepair }) {
	return <Modal title="Preview quota counter repair" onClose={onClose}>
		<p className="mb-4 text-sm text-muted">This will update only <strong>{preview.subject}</strong> / <strong>{preview.meter}</strong> for the active {preview.period} period. The apply will fail if usage changes after this preview.</p>
		<DataTable emptyLabel="" headers={['Field', 'Current', 'Recalculated']} rows={[
			['Event count', formatNumber(preview.before.event_count), formatNumber(preview.after.event_count)],
			['Quantity sum', formatNumber(preview.before.quantity_sum), formatNumber(preview.after.quantity_sum)],
			['Minimum', formatNumber(preview.before.quantity_min), formatNumber(preview.after.quantity_min)],
			['Maximum', formatNumber(preview.before.quantity_max), formatNumber(preview.after.quantity_max)],
		]} />
		<div className="mt-4 flex justify-end gap-2"><Button onClick={onClose} type="button" variant="outline">Cancel</Button><Button disabled={loading} onClick={onApply} type="button">Apply audited repair</Button></div>
	</Modal>
}
