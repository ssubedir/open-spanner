import { useRouter } from '@tanstack/react-router'
import { useSelector } from '@tanstack/react-store'
import { ArrowLeft, BellRing, Loader2, Play } from 'lucide-react'
import { useCallback, useEffect } from 'react'

import { appStore, appStoreActions } from '../app-store'
import { DataTable, DetailLoadingPage, DetailStatePage, Modal, PageHeader } from '../components/dashboard'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { formatNumber } from '../lib/format'
import { useInitialLoad } from '../lib/hooks'
import {
  AlertEventDetail,
  AlertEventTable,
  RuleDestinationDetail,
  RuleState,
  comparatorLabel,
  durationLabel,
  groupLabel,
  ruleForEvent,
} from './AlertPageParts'

export function AlertDetailPage({ ruleId }: { ruleId: string }) {
  const router = useRouter()
  const {
    error,
    eventLoadingMore,
    eventNextCursor,
    eventStatus,
    events,
    items,
    saving,
    selectedEvent,
    status,
  } = useSelector(appStore, (state) => state.alerts)
  const load = useCallback(() => appStoreActions.loadAlerts(), [])
  const pollEvents = useCallback(() => appStoreActions.loadAlertEvents({ quiet: true }), [])
  const rule = items.find((item) => item.id === ruleId) ?? null
  const ruleEvents = events.filter((event) => event.rule_id === ruleId)
  const selectedEventRule = selectedEvent ? ruleForEvent(items, selectedEvent) : null

  useInitialLoad(load)

  useEffect(() => {
    const poll = window.setInterval(() => {
      void pollEvents()
    }, 5000)

    return () => window.clearInterval(poll)
  }, [pollEvents])

  if (!rule && status !== 'ready' && status !== 'error') {
    return (
      <DetailLoadingPage
        eyebrow="Alerts"
        icon={<BellRing />}
        title="Loading alert"
        description="Loading alert rule before showing delivery and event details."
        action={(
          <Button onClick={() => void router.navigate({ to: '/alerts' })} type="button" variant="outline">
            <ArrowLeft aria-hidden="true" />
            Back to alerts
          </Button>
        )}
      />
    )
  }

  if (!rule) {
    const loadFailed = status === 'error'
    return (
      <DetailStatePage
        icon={<BellRing />}
        title={loadFailed ? 'Could not load alert' : 'Alert not found'}
        description={loadFailed ? 'Try again from the alerts list.' : 'This threshold rule may have been deleted or belongs to another workspace.'}
        action={(
          <Button onClick={() => void router.navigate({ to: '/alerts' })} type="button" variant="outline">
            <ArrowLeft aria-hidden="true" />
            Back to alerts
          </Button>
        )}
      />
    )
  }

  return (
    <>
      <PageHeader
        eyebrow="Alerts"
        icon={<BellRing />}
        title={rule?.name || 'Alert rule'}
        description={rule ? `${rule.meter} ${comparatorLabel(rule.comparator)} ${formatNumber(rule.threshold)} over ${durationLabel(rule.window_seconds)}` : 'Loading alert rule.'}
        action={(
          <div className="flex flex-wrap justify-end gap-2">
            <Button onClick={() => void router.navigate({ to: '/alerts' })} type="button" variant="outline">
              <ArrowLeft aria-hidden="true" />
              Back
            </Button>
            {rule ? (
              <Button disabled={saving} onClick={() => void appStoreActions.evaluateAlert(rule)} type="button">
                {saving ? <Loader2 className="spin" aria-hidden="true" /> : <Play aria-hidden="true" />}
                Evaluate
              </Button>
            ) : null}
          </div>
        )}
      />

      {error ? <div className="error-banner">{error}</div> : null}

      <div className="grid max-w-[1480px] gap-4">
        <section className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(320px,420px)]">
          <Card className="min-w-0">
            <CardHeader className="!px-4 !py-3">
              <div>
                <CardTitle>Rule</CardTitle>
                <CardDescription>Threshold definition and current state.</CardDescription>
              </div>
            </CardHeader>
            <CardContent>
              {rule ? (
                <DataTable
                  className="!min-w-0"
                  emptyLabel="No rule details"
                  headers={['Meter', 'Condition', 'Window', 'Evaluate Per', 'State']}
                  rows={[[
                    <span className="mono">{rule.meter}</span>,
                    <span>{comparatorLabel(rule.comparator)} {formatNumber(rule.threshold)}</span>,
                    durationLabel(rule.window_seconds),
                    rule.group_by ? groupLabel(rule.group_by) : 'total',
                    <RuleState rule={rule} />,
                  ]]}
                />
              ) : (
                <p className="subject-empty">Loading alert rule.</p>
              )}
            </CardContent>
          </Card>

          <Card className="min-w-0">
            <CardHeader className="!px-4 !py-3">
              <div>
                <CardTitle>Destination</CardTitle>
                <CardDescription>Delivery target used when this rule changes state.</CardDescription>
              </div>
            </CardHeader>
            <CardContent className="grid gap-3 !p-4">
              {rule ? <RuleDestinationDetail rule={rule} /> : <p className="subject-empty">Loading destination.</p>}
            </CardContent>
          </Card>
        </section>

        <Card className="min-w-0">
          <CardHeader className="!px-4 !py-3">
            <div>
              <CardTitle>Recent Events</CardTitle>
              <CardDescription>Triggered, resolved, and failed evaluations for this rule.</CardDescription>
            </div>
          </CardHeader>
          <CardContent>
            <AlertEventTable events={ruleEvents} loading={eventStatus === 'loading'} rules={items} />
            {eventNextCursor ? (
              <div className="pagination-actions">
                <Button disabled={eventLoadingMore} onClick={() => void appStoreActions.loadMoreAlertEvents()} type="button" variant="outline">
                  {eventLoadingMore ? <Loader2 className="spin" aria-hidden="true" /> : null}
                  Load more events
                </Button>
              </div>
            ) : null}
          </CardContent>
        </Card>
      </div>

      {selectedEvent ? (
        <Modal className="alert-event-modal" title="Alert Event" onClose={() => appStoreActions.setAlertSelectedEvent(null)}>
          <AlertEventDetail event={selectedEvent} rule={selectedEventRule} />
          <div className="modal-actions">
            <Button onClick={() => appStoreActions.setAlertSelectedEvent(null)} type="button" variant="outline">Close</Button>
          </div>
        </Modal>
      ) : null}
    </>
  )
}
