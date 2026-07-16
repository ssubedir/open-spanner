package usagebatch

import (
	"sort"
	"time"

	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

type Bounds struct {
	From time.Time
	To   time.Time
}

type Assignment struct {
	AssignedAt   time.Time
	Anchor       time.Time
	UnassignedAt *time.Time
}

type Counter struct {
	Subject       string
	MeterName     string
	Period        string
	PeriodStart   time.Time
	PeriodEnd     time.Time
	EventCount    int64
	QuantitySum   float64
	QuantityMin   float64
	QuantityMax   float64
	FirstQuantity float64
	FirstEventAt  time.Time
	LastQuantity  float64
	LastEventAt   time.Time
}

func SubjectBounds(events []domainusage.Event) map[string]Bounds {
	bounds := make(map[string]Bounds)
	for _, event := range events {
		at := event.EventTime()
		bound, ok := bounds[event.Subject()]
		if !ok {
			bounds[event.Subject()] = Bounds{From: at, To: at}
			continue
		}
		if at.Before(bound.From) {
			bound.From = at
		}
		if at.After(bound.To) {
			bound.To = at
		}
		bounds[event.Subject()] = bound
	}
	return bounds
}

func AggregateCounters(events []domainusage.Event, assignments map[string][]Assignment) []Counter {
	type counterKey struct {
		subject     string
		meterName   string
		period      string
		periodStart time.Time
	}
	counters := make(map[counterKey]Counter)
	for _, event := range events {
		anchor, ok := activeAnchor(assignments[event.Subject()], event.EventTime())
		if !ok {
			continue
		}
		for _, window := range counterWindows(event.EventTime(), anchor) {
			key := counterKey{subject: event.Subject(), meterName: event.MeterName(), period: window.period, periodStart: window.from}
			counter, exists := counters[key]
			if !exists {
				counters[key] = Counter{
					Subject: event.Subject(), MeterName: event.MeterName(), Period: window.period,
					PeriodStart: window.from, PeriodEnd: window.to, EventCount: 1,
					QuantitySum: event.Quantity(), QuantityMin: event.Quantity(), QuantityMax: event.Quantity(),
					FirstQuantity: event.Quantity(), FirstEventAt: event.EventTime(),
					LastQuantity: event.Quantity(), LastEventAt: event.EventTime(),
				}
				continue
			}
			counter.EventCount++
			counter.QuantitySum += event.Quantity()
			counter.QuantityMin = min(counter.QuantityMin, event.Quantity())
			counter.QuantityMax = max(counter.QuantityMax, event.Quantity())
			if event.EventTime().Before(counter.FirstEventAt) {
				counter.FirstEventAt = event.EventTime()
				counter.FirstQuantity = event.Quantity()
			}
			if !event.EventTime().Before(counter.LastEventAt) {
				counter.LastEventAt = event.EventTime()
				counter.LastQuantity = event.Quantity()
			}
			counters[key] = counter
		}
	}

	result := make([]Counter, 0, len(counters))
	for _, counter := range counters {
		result = append(result, counter)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Subject != result[j].Subject {
			return result[i].Subject < result[j].Subject
		}
		if result[i].MeterName != result[j].MeterName {
			return result[i].MeterName < result[j].MeterName
		}
		if result[i].Period != result[j].Period {
			return result[i].Period < result[j].Period
		}
		return result[i].PeriodStart.Before(result[j].PeriodStart)
	})
	return result
}

func activeAnchor(assignments []Assignment, at time.Time) (time.Time, bool) {
	// Assignment timestamps are stored as RFC3339 text in both backends, so
	// preserve the repository queries' comparison semantics here.
	atText := at.UTC().Format(time.RFC3339Nano)
	for _, assignment := range assignments {
		if assignment.AssignedAt.UTC().Format(time.RFC3339Nano) > atText {
			continue
		}
		if assignment.UnassignedAt == nil || assignment.UnassignedAt.UTC().Format(time.RFC3339Nano) > atText {
			return assignment.Anchor, true
		}
	}
	return time.Time{}, false
}

type counterWindow struct {
	period string
	from   time.Time
	to     time.Time
}

func counterWindows(at time.Time, anchor time.Time) []counterWindow {
	at = at.UTC()
	anchor = anchor.UTC()
	if anchor.IsZero() || at.Before(anchor) {
		return nil
	}
	return []counterWindow{
		counterWindowForPeriod(at, anchor, "day"),
		counterWindowForPeriod(at, anchor, "week"),
		counterWindowForPeriod(at, anchor, "month"),
		counterWindowForPeriod(at, anchor, "year"),
	}
}

func counterWindowForPeriod(at time.Time, anchor time.Time, period string) counterWindow {
	from := anchor
	to := addCounterPeriod(from, period)
	for !at.Before(to) {
		from = to
		to = addCounterPeriod(from, period)
	}
	return counterWindow{period: period, from: from, to: to}
}

func addCounterPeriod(from time.Time, period string) time.Time {
	switch period {
	case "day":
		return from.AddDate(0, 0, 1)
	case "week":
		return from.AddDate(0, 0, 7)
	case "year":
		return from.AddDate(1, 0, 0)
	default:
		return from.AddDate(0, 1, 0)
	}
}
