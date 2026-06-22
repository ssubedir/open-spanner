package usage

import (
	"errors"
	"strings"
	"testing"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

func TestNormalizeSubject(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"org_123", "customer-42", "acct.abc", "tenant:west"} {
		got, err := NormalizeSubject(" " + value + " ")
		if err != nil {
			t.Fatalf("NormalizeSubject(%q) error = %v", value, err)
		}
		if got != value {
			t.Fatalf("NormalizeSubject(%q) = %q, want %q", value, got, value)
		}
	}
}

func TestNormalizeSubjectRejectsInvalidIdentifiers(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"", "org 123", "org/123", "gggg-<>ddddd", "-org", strings.Repeat("a", MaxSubjectLength+1)} {
		if _, err := NormalizeSubject(value); !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("NormalizeSubject(%q) error = %v, want invalid input", value, err)
		}
	}
}
