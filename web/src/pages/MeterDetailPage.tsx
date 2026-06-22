import { useRouter } from '@tanstack/react-router'
import { useSelector } from '@tanstack/react-store'
import { ArrowLeft, BarChart3, Boxes, CalendarClock, Clock } from 'lucide-react'
import type { ReactNode } from 'react'
import { useCallback } from 'react'

import { appStore, appStoreActions } from '../app-store'
import { DetailLoadingPage, DetailStatePage, PageHeader } from '../components/dashboard'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { formatDate, formatNumber } from '../lib/format'
import { useInitialLoad } from '../lib/hooks'
import { defaultFilterQuery, queryWithMeter } from '../lib/usage-query'
import { DimensionChips, DimensionTable } from './MeterPageParts'

export function MeterDetailPage({ routeMeter }: { routeMeter: string }) {
  const router = useRouter()
  const { error, items, stats, status } = useSelector(appStore, (state) => state.meters)
  const load = useCallback(() => appStoreActions.loadMeters(), [])
  const meter = items.find((item) => item.name === routeMeter || item.id === routeMeter) ?? null
  const stat = meter ? stats[meter.name] : null

  useInitialLoad(load)

  function openUsage() {
    if (!meter) {
      return
    }
    appStoreActions.setUsageFilterQuery(queryWithMeter(defaultFilterQuery(), meter.name))
    void router.navigate({ to: '/usage' })
  }

  if (!meter && status !== 'ready' && status !== 'error') {
    return (
      <DetailLoadingPage
        eyebrow="Meters"
        icon={<Boxes />}
        title="Loading meter"
        description="Loading meter definition before showing schema and activity."
        action={(
          <Button onClick={() => void router.navigate({ to: '/meters' })} type="button" variant="outline">
            <ArrowLeft aria-hidden="true" />
            Back to meters
          </Button>
        )}
      />
    )
  }

  if (!meter) {
    const loadFailed = status === 'error'
    return (
      <DetailStatePage
        icon={<Boxes />}
        title={loadFailed ? 'Could not load meter' : 'Meter not found'}
        description={loadFailed ? 'Try again from the meters list.' : 'This meter may have been deleted or belongs to another workspace.'}
        action={(
          <Button onClick={() => void router.navigate({ to: '/meters' })} type="button" variant="outline">
            <ArrowLeft aria-hidden="true" />
            Back to meters
          </Button>
        )}
      />
    )
  }

  return (
    <>
      <PageHeader
        eyebrow="Meters"
        icon={<Boxes />}
        title={meter?.name ?? 'Meter details'}
        description={meter?.description || 'Inspect this meter definition, schema, and activity.'}
        action={(
          <div className="flex flex-wrap justify-end gap-2">
            <Button onClick={() => void router.navigate({ to: '/meters' })} type="button" variant="outline">
              <ArrowLeft aria-hidden="true" />
              Back
            </Button>
            {meter ? (
              <Button onClick={openUsage} type="button" variant="outline">
                <BarChart3 aria-hidden="true" />
                Analyze usage
              </Button>
            ) : null}
          </div>
        )}
      />

      {error ? <div className="error-banner">{error}</div> : null}

      <div className="grid max-w-[1480px] gap-4">
        <section className="grid gap-4 lg:grid-cols-4">
          <MeterFact icon={<Boxes />} label="Aggregation" value={meter?.aggregation ?? '-'} helper={meter?.unit ? `Unit: ${meter.unit}` : 'Loading'} />
          <MeterFact icon={<BarChart3 />} label="Usage Events" value={formatNumber(stat?.usage_events ?? 0)} helper={stat?.last_event_at ? `Last: ${formatDate(stat.last_event_at)}` : 'No events yet'} />
          <MeterFact icon={<Clock />} label="Retention" value={`${meter?.event_retention_days ?? stat?.retention_days ?? 0} days`} helper="Event retention window" />
          <MeterFact icon={<CalendarClock />} label="Created" value={meter ? formatDate(meter.created_at) : '-'} helper="Definition created" />
        </section>

        <section className="grid items-start gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(320px,420px)]">
          <Card className="min-w-0">
            <CardHeader className="!px-4 !py-3">
              <div>
                <CardTitle>Dimensions</CardTitle>
                <CardDescription>Metadata schema accepted for this meter.</CardDescription>
              </div>
            </CardHeader>
            <CardContent>
              {meter ? <DimensionTable meter={meter} /> : <p className="subject-empty">Loading meter schema.</p>}
            </CardContent>
          </Card>

          <Card className="min-w-0">
            <CardHeader className="!px-4 !py-3">
              <div>
                <CardTitle>Summary</CardTitle>
                <CardDescription>Definition settings and schema chips.</CardDescription>
              </div>
            </CardHeader>
            <CardContent className="grid gap-4 !p-4">
              {meter ? (
                <>
                  <div className="grid gap-2">
                    <span className="text-xs font-bold uppercase text-muted">Schema</span>
                    <DimensionChips meter={meter} />
                  </div>
                  <div className="grid gap-2">
                    <span className="text-xs font-bold uppercase text-muted">Definition</span>
                    <div className="flex flex-wrap gap-2">
                      <Badge variant="muted">{meter.aggregation}</Badge>
                      <Badge variant="muted">{meter.unit}</Badge>
                      <Badge variant="muted">{meter.event_retention_days} days retention</Badge>
                    </div>
                  </div>
                </>
              ) : (
                <p className="subject-empty">Loading meter summary.</p>
              )}
            </CardContent>
          </Card>
        </section>
      </div>
    </>
  )
}

function MeterFact({ helper, icon, label, value }: { helper: string; icon: ReactNode; label: string; value: string }) {
  return (
    <Card className="min-w-0">
      <CardContent className="flex items-center gap-3 !p-4">
        <span className="metric-icon">{icon}</span>
        <span className="grid min-w-0 gap-1">
          <span className="text-xs font-bold uppercase text-muted">{label}</span>
          <strong className="truncate text-2xl">{value}</strong>
          <small className="truncate text-xs text-muted">{helper}</small>
        </span>
      </CardContent>
    </Card>
  )
}
