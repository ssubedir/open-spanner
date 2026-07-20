package main

import (
	"testing"
	"time"
)

func TestSplitPreservesTotal(t *testing.T) {
	got := split(27, 5)
	total := 0
	for _, value := range got {
		total += value
	}
	if total != 27 || len(got) != 5 || got[0]-got[4] > 1 {
		t.Fatalf("split = %v", got)
	}
}

func TestPercentileUsesNearestRank(t *testing.T) {
	values := []time.Duration{5 * time.Millisecond, time.Millisecond, 3 * time.Millisecond, 2 * time.Millisecond, 4 * time.Millisecond}
	if got := percentile(values, .95); got != 5*time.Millisecond {
		t.Fatalf("p95 = %s", got)
	}
	if got := percentile(values, .50); got != 3*time.Millisecond {
		t.Fatalf("p50 = %s", got)
	}
}
