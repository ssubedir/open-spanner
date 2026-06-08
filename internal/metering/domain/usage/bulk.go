package usage

type BulkSaveResult struct {
	accepted   []Event
	duplicates []Event
}

func NewBulkSaveResult(accepted []Event, duplicates []Event) BulkSaveResult {
	return BulkSaveResult{
		accepted:   copyEventSlice(accepted),
		duplicates: copyEventSlice(duplicates),
	}
}

func (r BulkSaveResult) Accepted() []Event {
	return copyEventSlice(r.accepted)
}

func (r BulkSaveResult) Duplicates() []Event {
	return copyEventSlice(r.duplicates)
}

func (r BulkSaveResult) Events() []Event {
	events := make([]Event, 0, len(r.accepted)+len(r.duplicates))
	events = append(events, r.accepted...)
	events = append(events, r.duplicates...)
	return copyEventSlice(events)
}

func copyEventSlice(events []Event) []Event {
	copied := make([]Event, len(events))
	copy(copied, events)
	return copied
}
