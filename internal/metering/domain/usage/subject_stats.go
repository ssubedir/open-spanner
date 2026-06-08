package usage

import "time"

type SubjectStats struct {
	subject     string
	usageEvents int
	meters      int
	lastEventAt time.Time
}

func NewSubjectStats(subject string, usageEvents int, meters int, lastEventAt time.Time) SubjectStats {
	return SubjectStats{
		subject:     subject,
		usageEvents: usageEvents,
		meters:      meters,
		lastEventAt: lastEventAt.UTC(),
	}
}

func (s SubjectStats) Subject() string {
	return s.subject
}

func (s SubjectStats) UsageEvents() int {
	return s.usageEvents
}

func (s SubjectStats) Meters() int {
	return s.meters
}

func (s SubjectStats) LastEventAt() time.Time {
	return s.lastEventAt
}
