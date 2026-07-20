package entitlement

import (
	"testing"
	"time"

	domainmeter "github.com/ssubedir/open-spanner/internal/metering/domain/meter"
)

func TestPeriodWindowUsesAssignmentAnchor(t *testing.T) {
	anchor := time.Date(2026, time.June, 20, 15, 30, 0, 0, time.UTC)
	now := time.Date(2026, time.July, 21, 10, 0, 0, 0, time.UTC)

	from, to := periodWindow(now, anchor, PeriodMonth)

	wantFrom := time.Date(2026, time.July, 20, 15, 30, 0, 0, time.UTC)
	wantTo := time.Date(2026, time.August, 20, 15, 30, 0, 0, time.UTC)
	if !from.Equal(wantFrom) || !to.Equal(wantTo) {
		t.Fatalf("periodWindow() = %s - %s, want %s - %s", from, to, wantFrom, wantTo)
	}
}

func TestPeriodWindowBeforeAssignmentStartsAtAnchor(t *testing.T) {
	anchor := time.Date(2026, time.June, 20, 15, 30, 0, 0, time.UTC)
	now := time.Date(2026, time.June, 19, 15, 30, 0, 0, time.UTC)

	from, to := periodWindow(now, anchor, PeriodWeek)

	wantTo := time.Date(2026, time.June, 27, 15, 30, 0, 0, time.UTC)
	if !from.Equal(anchor) || !to.Equal(wantTo) {
		t.Fatalf("periodWindow() = %s - %s, want %s - %s", from, to, anchor, wantTo)
	}
}

func TestProjectedCounterValue(t *testing.T) {
	earlier := time.Date(2026, time.July, 15, 11, 0, 0, 0, time.UTC)
	later := earlier.Add(time.Hour)
	counter := EntitlementUsageCounter{
		EventCount:     2,
		QuantitySum:    8,
		QuantityMin:    3,
		QuantityMax:    5,
		FirstQuantity:  3,
		FirstEventTime: earlier,
		LastQuantity:   5,
		LastEventTime:  later,
	}

	tests := []struct {
		name        string
		aggregation domainmeter.Aggregation
		quantity    float64
		eventTime   time.Time
		duration    float64
		want        float64
	}{
		{name: "sum", aggregation: domainmeter.AggregationSum, quantity: 4, eventTime: later, duration: 10, want: 12},
		{name: "count", aggregation: domainmeter.AggregationCount, quantity: 4, eventTime: later, duration: 10, want: 3},
		{name: "average", aggregation: domainmeter.AggregationAverage, quantity: 4, eventTime: later, duration: 10, want: 4},
		{name: "minimum", aggregation: domainmeter.AggregationMinimum, quantity: 2, eventTime: later, duration: 10, want: 2},
		{name: "maximum", aggregation: domainmeter.AggregationMaximum, quantity: 7, eventTime: later, duration: 10, want: 7},
		{name: "first keeps earlier value", aggregation: domainmeter.AggregationFirst, quantity: 9, eventTime: later, duration: 10, want: 3},
		{name: "first accepts backfill", aggregation: domainmeter.AggregationFirst, quantity: 9, eventTime: earlier.Add(-time.Hour), duration: 10, want: 9},
		{name: "last accepts later value", aggregation: domainmeter.AggregationLast, quantity: 9, eventTime: later.Add(time.Hour), duration: 10, want: 9},
		{name: "last keeps later value", aggregation: domainmeter.AggregationLast, quantity: 9, eventTime: earlier, duration: 10, want: 5},
		{name: "rate", aggregation: domainmeter.AggregationRate, quantity: 4, eventTime: later, duration: 10, want: 0.3},
		{name: "rate with empty window", aggregation: domainmeter.AggregationRate, quantity: 4, eventTime: later, duration: 0, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := projectedCounterValue(counter, tt.aggregation, tt.quantity, tt.eventTime, tt.duration)
			if got != tt.want {
				t.Fatalf("projectedCounterValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProjectedCounterValueForFirstEvent(t *testing.T) {
	counter := EntitlementUsageCounter{}
	now := time.Now().UTC()
	for _, aggregation := range []domainmeter.Aggregation{
		domainmeter.AggregationMinimum,
		domainmeter.AggregationMaximum,
		domainmeter.AggregationFirst,
		domainmeter.AggregationLast,
	} {
		if got := projectedCounterValue(counter, aggregation, 6, now, 60); got != 6 {
			t.Fatalf("projectedCounterValue(empty, %s) = %v, want 6", aggregation, got)
		}
	}
}

func TestEnforcementDefaultsAndValidation(t *testing.T) {
	enforcement, err := normalizeEnforcement("")
	if err != nil || enforcement != EnforcementAdvisory {
		t.Fatalf("normalizeEnforcement(empty) = %q, %v", enforcement, err)
	}
	policy, err := normalizeFailurePolicy("")
	if err != nil || policy != FailurePolicyFailOpen {
		t.Fatalf("normalizeFailurePolicy(empty) = %q, %v", policy, err)
	}
	if _, err := normalizeEnforcement("sometimes"); err == nil {
		t.Fatal("normalizeEnforcement(invalid) returned nil error")
	}
	if _, err := normalizeFailurePolicy("maybe"); err == nil {
		t.Fatal("normalizeFailurePolicy(invalid) returned nil error")
	}
}
