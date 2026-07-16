package usagebatch

import (
	"testing"
	"time"

	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

func TestAggregateCountersGroupsEventsAndPreservesFirstLast(t *testing.T) {
	anchor := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	events := []domainusage.Event{
		newEvent(t, "one", 2, time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)),
		newEvent(t, "two", 5, time.Date(2026, 1, 2, 9, 0, 0, 0, time.UTC)),
		newEvent(t, "three", 3, time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)),
	}
	counters := AggregateCounters(events, map[string][]Assignment{
		"subject": {{AssignedAt: anchor, Anchor: anchor}},
	})
	if len(counters) != 4 {
		t.Fatalf("counter count = %d, want 4", len(counters))
	}
	for _, counter := range counters {
		if counter.EventCount != 3 || counter.QuantitySum != 10 || counter.QuantityMin != 2 || counter.QuantityMax != 5 {
			t.Fatalf("%s counter totals = %#v", counter.Period, counter)
		}
		if counter.FirstQuantity != 5 || !counter.FirstEventAt.Equal(events[1].EventTime()) {
			t.Fatalf("%s first event = %v at %v", counter.Period, counter.FirstQuantity, counter.FirstEventAt)
		}
		if counter.LastQuantity != 3 || !counter.LastEventAt.Equal(events[2].EventTime()) {
			t.Fatalf("%s last event = %v at %v", counter.Period, counter.LastQuantity, counter.LastEventAt)
		}
	}
}

func TestAggregateCountersResolvesAssignmentBoundaries(t *testing.T) {
	firstAnchor := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	boundary := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)
	secondAnchor := boundary
	events := []domainusage.Event{
		newEvent(t, "before", 2, boundary.Add(-time.Second)),
		newEvent(t, "after", 3, boundary),
	}
	counters := AggregateCounters(events, map[string][]Assignment{
		"subject": {
			{AssignedAt: boundary, Anchor: secondAnchor},
			{AssignedAt: firstAnchor, Anchor: firstAnchor, UnassignedAt: &boundary},
		},
	})
	if len(counters) != 8 {
		t.Fatalf("counter count = %d, want 8 across two assignments", len(counters))
	}
	for _, counter := range counters {
		if counter.EventCount != 1 {
			t.Fatalf("boundary counter combined assignments: %#v", counter)
		}
	}
}

func newEvent(t *testing.T, id string, quantity float64, at time.Time) domainusage.Event {
	t.Helper()
	event, err := domainusage.NewEvent(id, id, "subject", "meter", quantity, at, at, nil)
	if err != nil {
		t.Fatalf("new event: %v", err)
	}
	return event
}
