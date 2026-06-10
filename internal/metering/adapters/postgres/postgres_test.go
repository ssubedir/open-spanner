package postgres

import "testing"

func TestRebind(t *testing.T) {
	query := "SELECT * FROM usage_events WHERE subject = ? AND meter_name = ? LIMIT ?"
	got := rebind(query)
	want := "SELECT * FROM usage_events WHERE subject = $1 AND meter_name = $2 LIMIT $3"
	if got != want {
		t.Fatalf("rebind() = %q, want %q", got, want)
	}
}

func TestPostgresJSONPath(t *testing.T) {
	got := postgresJSONPath("region.zone")
	want := "'{region,zone}'"
	if got != want {
		t.Fatalf("postgresJSONPath() = %q, want %q", got, want)
	}
}
