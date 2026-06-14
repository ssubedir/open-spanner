import { BarChart3, Clock, Database, Loader2, Plus, RefreshCw, Rows3, Search } from 'lucide-react'
import { type FormEvent, useCallback, useMemo, useState } from 'react'
import type { RuleGroupType } from 'react-querybuilder'

import { createUsage, listMeters, listUsageBuckets, type Meter, type UsageBucket } from '../api'
import { DataTable, MetricCard, Modal, PageHeader } from '../components/dashboard'
import { FilterBuilder } from '../components/filter-builder'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { toInputDateTime } from '../lib/datetime'
import { formatDate, formatNumber } from '../lib/format'
import { useInitialLoad } from '../lib/hooks'
import { parseJSONRecord } from '../lib/metadata'
import {
  buildFilterFields,
  defaultFilterQuery,
  firstEqualRuleValue,
  queryWithAvailableMeter,
  selectedMeterSchemaKeys,
  usageFilterFromQuery,
  usageScopeFromQuery,
  usageTimeRangeFromQuery,
} from '../lib/usage-query'
import type { LoadState } from '../types'

export function UsagePage() {
  const [status, setStatus] = useState<LoadState>('idle')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [meters, setMeters] = useState<Meter[]>([])
  const [buckets, setBuckets] = useState<UsageBucket[]>([])
  const [createOpen, setCreateOpen] = useState(false)
  const [groupBy, setGroupBy] = useState('')
  const [filterQuery, setFilterQuery] = useState<RuleGroupType>(() => defaultFilterQuery())

  const load = useCallback(async () => {
    setStatus('loading')
    setError('')
    try {
      const nextMeters = await listMeters()
      setMeters(nextMeters.items)
      setFilterQuery((query) => queryWithAvailableMeter(query, nextMeters.items))
      setStatus('ready')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unable to load usage controls')
      setStatus('error')
    }
  }, [])

  useInitialLoad(load)

  async function submitCreateUsage(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setSaving(true)
    setError('')
    const form = new FormData(event.currentTarget)

    try {
      await createUsage({
        idempotency_key: String(form.get('idempotency_key') || ''),
        metadata: parseJSONRecord(String(form.get('metadata') || '{}'), 'Metadata'),
        meter: String(form.get('meter') || ''),
        quantity: Number(form.get('quantity') || 0),
        subject: String(form.get('subject') || ''),
        timestamp: localInputToOptionalISO(String(form.get('timestamp') || '')),
      })
      setCreateOpen(false)
      await submitQueryFromState(activeGroupBy)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unable to create usage')
    } finally {
      setSaving(false)
    }
  }

  async function submitQuery(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    await submitQueryFromState(String(form.get('group_by') || ''), Number(form.get('limit') || 500), String(form.get('bucket_size') || 'day'))
  }

  async function submitQueryFromState(groupByValue: string, limit = 500, bucketSize = 'day') {
    setStatus('loading')
    setError('')
    try {
      const scope = usageScopeFromQuery(filterQuery)
      const timeRange = usageTimeRangeFromQuery(filterQuery)
      const filter = usageFilterFromQuery(filterQuery)
      const nextBuckets = await listUsageBuckets({
        bucket_size: bucketSize,
        filter,
        from: timeRange.from,
        group_by: groupByValue || undefined,
        limit,
        meter: scope.meter,
        subject: scope.subject,
        to: timeRange.to,
      })
      setBuckets(nextBuckets)
      setStatus('ready')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unable to query usage')
      setStatus('error')
    }
  }

  const total = buckets.reduce((sum, bucket) => sum + Number(bucket.quantity || 0), 0)
  const selectedMeterName = firstEqualRuleValue(filterQuery, 'meter')
  const groupKeys = useMemo(() => selectedMeterSchemaKeys(meters, selectedMeterName), [meters, selectedMeterName])
  const activeGroupBy = groupKeys.includes(groupBy) ? groupBy : ''
  const filterFields = useMemo(() => buildFilterFields(groupKeys, meters), [groupKeys, meters])

  function resetQuery() {
    setGroupBy('')
    setFilterQuery(queryWithAvailableMeter(defaultFilterQuery(), meters))
  }

  return (
    <>
      <PageHeader
        eyebrow="Usage"
        icon={<BarChart3 />}
        title="Usage buckets"
        description="Query bucketed usage with a time window, bucket settings, and advanced filters."
        action={(
          <div className="header-actions">
            <Button disabled={status === 'loading'} onClick={() => void load()} type="button" variant="outline">
              {status === 'loading' ? <Loader2 className="spin" aria-hidden="true" /> : <RefreshCw aria-hidden="true" />}
              Refresh
            </Button>
            <Button onClick={() => setCreateOpen(true)} type="button">
              <Plus aria-hidden="true" />
              Create Usage
            </Button>
          </div>
        )}
      />

      {error ? <div className="error-banner">{error}</div> : null}

      <section className="metric-grid meters-metrics" aria-label="Usage metrics">
        <MetricCard icon={<Database />} label="Meters" value={meters.length} helper="Available for queries" />
        <MetricCard icon={<Rows3 />} label="Buckets" value={buckets.length} helper="Rows in current result" />
        <MetricCard icon={<BarChart3 />} label="Total Quantity" value={Math.round(total)} helper="Sum of visible buckets" />
        <MetricCard icon={<Clock />} label="Window Days" value={7} helper="Default query range" />
      </section>

      <section className="usage-grid">
        <Card>
          <CardHeader>
            <div>
              <CardTitle>Usage Query</CardTitle>
              <CardDescription>Filter with rules, then choose the result shape.</CardDescription>
            </div>
          </CardHeader>
          <CardContent className="form-card">
            <form className="form-grid usage-query-form" onSubmit={(event) => void submitQuery(event)}>
              <FilterBuilder
                fields={filterFields}
                onChange={setFilterQuery}
                query={filterQuery}
              />
              <div className="query-controls wide">
                <label>
                  Bucket
                  <select aria-label="Bucket" name="bucket_size">
                    <option value="day">Day</option>
                    <option value="hour">Hour</option>
                    <option value="month">Month</option>
                  </select>
                </label>
                <label>
                  Group By
                  <select aria-label="Group By" name="group_by" value={activeGroupBy} onChange={(event) => setGroupBy(event.target.value)}>
                    <option value="">None</option>
                    {groupKeys.map((key) => <option key={key} value={key}>{key}</option>)}
                  </select>
                </label>
                <label>
                  Limit
                  <input defaultValue="500" max="1000" min="1" name="limit" type="number" />
                </label>
                <div className="query-actions">
                  <Button onClick={resetQuery} type="button" variant="outline">
                    <RefreshCw aria-hidden="true" />
                    Reset
                  </Button>
                  <Button disabled={status === 'loading'} type="submit">
                    {status === 'loading' ? <Loader2 className="spin" aria-hidden="true" /> : <Search aria-hidden="true" />}
                    Run Query
                  </Button>
                </div>
              </div>
            </form>
          </CardContent>
        </Card>

        <Card className="usage-results-card">
          <CardHeader>
            <div>
              <CardTitle>Results</CardTitle>
              <CardDescription>Bucketed usage returned by the current query.</CardDescription>
            </div>
            <Badge variant={buckets.length > 0 ? 'success' : 'muted'}>{buckets.length} rows</Badge>
          </CardHeader>
          <CardContent>
            <DataTable
              emptyLabel="Run a query to view usage"
              headers={['Bucket Start', 'Subject', 'Meter', 'Aggregation', 'Unit', 'Group', 'Quantity']}
              rows={buckets.map((bucket) => [
                formatDate(bucket.bucket_start),
                <span className="mono strong">{bucket.subject}</span>,
                bucket.meter,
                <Badge variant="muted">{bucket.aggregation}</Badge>,
                bucket.unit,
                <span className="mono truncate">{JSON.stringify(bucket.group || {})}</span>,
                formatNumber(bucket.quantity),
              ])}
            />
          </CardContent>
        </Card>
      </section>

      {createOpen ? (
        <Modal title="Create Usage" onClose={() => setCreateOpen(false)}>
          <form className="modal-form usage-create-form" onSubmit={(event) => void submitCreateUsage(event)}>
            <label>
              Subject
              <input name="subject" placeholder="org_123" required />
            </label>
            <label>
              Meter
              <select aria-label="Meter" name="meter" required>
                <option value="">Select meter</option>
                {meters.map((meter) => <option key={meter.id} value={meter.name}>{meter.name}</option>)}
              </select>
            </label>
            <label>
              Quantity
              <input defaultValue="1" min="0" name="quantity" required step="0.000001" type="number" />
            </label>
            <label>
              Timestamp
              <input aria-label="Timestamp" defaultValue={toInputDateTime(new Date())} name="timestamp" type="datetime-local" />
            </label>
            <label className="wide">
              Idempotency Key
              <input name="idempotency_key" placeholder="Generated if blank" />
            </label>
            <label className="wide">
              Metadata JSON
              <textarea aria-label="Metadata JSON" defaultValue="{}" name="metadata" rows={5} />
            </label>
            <div className="modal-actions">
              <Button onClick={() => setCreateOpen(false)} type="button" variant="outline">Cancel</Button>
              <Button disabled={saving} type="submit">
                {saving ? <Loader2 className="spin" aria-hidden="true" /> : <Plus aria-hidden="true" />}
                Create
              </Button>
            </div>
          </form>
        </Modal>
      ) : null}
    </>
  )
}

function localInputToOptionalISO(value: string) {
  return value ? new Date(value).toISOString() : ''
}
