package usage

import "time"

type MeterStats struct {
	meterName   string
	usageEvents int
	lastEventAt time.Time
}

func NewMeterStats(meterName string, usageEvents int, lastEventAt time.Time) MeterStats {
	return MeterStats{
		meterName:   meterName,
		usageEvents: usageEvents,
		lastEventAt: lastEventAt.UTC(),
	}
}

func (s MeterStats) MeterName() string {
	return s.meterName
}

func (s MeterStats) UsageEvents() int {
	return s.usageEvents
}

func (s MeterStats) LastEventAt() time.Time {
	return s.lastEventAt
}
