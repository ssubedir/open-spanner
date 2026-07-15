import { ScanSearch } from 'lucide-react'
import { useCallback, useState } from 'react'

import { reconcileQuotaRecords, type ReconciliationResult } from '../api'
import { DataTable, MetricCard, PageHeader } from '../components/dashboard'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { formatDate, formatNumber } from '../lib/format'
import { useInitialLoad } from '../lib/hooks'

export function ReconciliationPage() {
	const [result, setResult] = useState<ReconciliationResult | null>(null)
	const [loading, setLoading] = useState(false)
	const [error, setError] = useState('')
	const load = useCallback(async () => {
		setLoading(true)
		setError('')
		try { setResult(await reconcileQuotaRecords()) }
		catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to reconcile quota records') }
		finally { setLoading(false) }
	}, [])
	useInitialLoad(load)

	return <>
		<PageHeader eyebrow="Operations" icon={<ScanSearch />} title="Quota reconciliation" description="Compare recent consumption decisions and active quota counters with source usage. This scan never modifies data." action={<Button disabled={loading} onClick={() => void load()} type="button">{loading ? 'Scanning…' : 'Run scan'}</Button>} />
		{error ? <div className="error-banner">{error}</div> : null}
		<section className="mb-4 grid gap-4 md:grid-cols-3">
			<MetricCard icon={<ScanSearch />} label="Issues" loading={loading && !result} value={result?.issues.length ?? 0} helper={result ? `${result.status === 'healthy' ? 'Healthy' : 'Drift detected'} · checked ${formatDate(result.checked_at)}` : 'Run a read-only scan'} />
			<MetricCard icon={<ScanSearch />} label="Decisions checked" loading={loading && !result} value={result?.decisions_checked ?? 0} helper={`${result?.lookback_hours ?? 24}-hour lookback`} />
			<MetricCard icon={<ScanSearch />} label="Active counters" loading={loading && !result} value={result?.counters_checked ?? 0} helper={result?.truncated ? 'Result was bounded; narrow the scan' : 'Current quota periods'} />
		</section>
		{result ? <div className="mb-3 flex items-center gap-2"><Badge variant={result.status === 'healthy' ? 'success' : 'warning'}>{result.status === 'healthy' ? 'No drift found' : `${formatNumber(result.issues.length)} issues`}</Badge><span className="text-sm text-muted">Read-only result</span></div> : null}
		<DataTable emptyLabel={loading ? 'Running reconciliation scan' : 'No quota inconsistencies detected.'} headers={['Severity', 'Issue', 'Subject', 'Meter', 'Expected', 'Actual', 'Details']} rows={(result?.issues ?? []).map((issue) => [
			<Badge variant="warning">{issue.severity}</Badge>, <span className="font-mono text-xs">{issue.kind}</span>, issue.subject ?? '—', issue.meter ?? '—', issue.expected, issue.actual, issue.message,
		])} />
	</>
}
