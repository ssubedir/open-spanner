package postgres

import (
	"testing"

	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

func TestMetadataContainsJSON(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value any
		want  string
	}{
		{
			name:  "flat string",
			key:   "region",
			value: "us-east-1",
			want:  `{"region":"us-east-1"}`,
		},
		{
			name:  "nested string",
			key:   "resource.plan",
			value: "enterprise",
			want:  `{"resource":{"plan":"enterprise"}}`,
		},
		{
			name:  "number",
			key:   "attempt",
			value: float64(3),
			want:  `{"attempt":3}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := metadataContainsJSON(tc.key, tc.value)
			if err != nil {
				t.Fatalf("metadataContainsJSON: %v", err)
			}
			if got != tc.want {
				t.Fatalf("metadataContainsJSON() = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestMetadataContainsJSONRejectsUnsafeKeys(t *testing.T) {
	if _, err := metadataContainsJSON("region-name", "us-east-1"); err == nil {
		t.Fatal("metadataContainsJSON error = nil, want rejection")
	}
}

func TestMetadataEqualityFilterUsesJSONBContainment(t *testing.T) {
	filter, err := domainusage.NewFilterCondition("metadata.region", domainusage.FilterOpEqual, "us-east-1", true)
	if err != nil {
		t.Fatalf("new filter: %v", err)
	}

	args := []any{}
	got, err := conditionWhereSQL(filter, &args)
	if err != nil {
		t.Fatalf("conditionWhereSQL: %v", err)
	}

	if got != "metadata @> $1::jsonb" {
		t.Fatalf("conditionWhereSQL() = %q, want JSONB containment", got)
	}
	if len(args) != 1 || args[0] != `{"region":"us-east-1"}` {
		t.Fatalf("args = %#v, want metadata containment payload", args)
	}
}

func TestMetadataInFilterUsesJSONBContainment(t *testing.T) {
	filter, err := domainusage.NewFilterCondition("metadata.plan", domainusage.FilterOpIn, []any{"free", "pro"}, true)
	if err != nil {
		t.Fatalf("new filter: %v", err)
	}

	args := []any{}
	got, err := conditionWhereSQL(filter, &args)
	if err != nil {
		t.Fatalf("conditionWhereSQL: %v", err)
	}

	if got != "(metadata @> $1::jsonb OR metadata @> $2::jsonb)" {
		t.Fatalf("conditionWhereSQL() = %q, want JSONB containment disjunction", got)
	}
	if len(args) != 2 || args[0] != `{"plan":"free"}` || args[1] != `{"plan":"pro"}` {
		t.Fatalf("args = %#v, want metadata containment payloads", args)
	}
}
