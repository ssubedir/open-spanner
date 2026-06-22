package usage

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

const MaxSubjectLength = 128

var subjectIdentifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]*$`)

func NormalizeSubject(value string) (string, error) {
	subject, err := NormalizeOptionalSubject(value)
	if err != nil {
		return "", err
	}
	if subject == "" {
		return "", fmt.Errorf("%w: subject is required", domain.ErrInvalidInput)
	}
	return subject, nil
}

func NormalizeOptionalSubject(value string) (string, error) {
	subject := strings.TrimSpace(value)
	if subject == "" {
		return "", nil
	}
	if len(subject) > MaxSubjectLength {
		return "", fmt.Errorf("%w: subject cannot exceed %d characters", domain.ErrInvalidInput, MaxSubjectLength)
	}
	if !subjectIdentifierPattern.MatchString(subject) {
		return "", fmt.Errorf("%w: subject %q must use letters, numbers, underscores, hyphens, dots, or colons", domain.ErrInvalidInput, subject)
	}
	return subject, nil
}
