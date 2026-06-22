package sqlite

import (
	"net/url"
	"testing"
)

func TestSQLiteDSNAppliesConnectionPragmas(t *testing.T) {
	dsn := sqliteDSN("open-spanner.db")
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}

	values := parsed.Query()
	pragmas := values["_pragma"]
	for _, expected := range []string{
		"busy_timeout=10000",
		"foreign_keys=ON",
		"journal_mode=WAL",
		"synchronous=NORMAL",
	} {
		if !contains(pragmas, expected) {
			t.Fatalf("dsn pragmas = %v, want %q", pragmas, expected)
		}
	}
	if got := values.Get("_txlock"); got != "immediate" {
		t.Fatalf("_txlock = %q, want immediate", got)
	}
}

func TestSQLiteDSNPreservesExistingQuery(t *testing.T) {
	dsn := sqliteDSN("file:open-spanner.db?cache=shared")
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}

	values := parsed.Query()
	if got := values.Get("cache"); got != "shared" {
		t.Fatalf("cache = %q, want shared", got)
	}
	if got := values.Get("_txlock"); got != "immediate" {
		t.Fatalf("_txlock = %q, want immediate", got)
	}
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
