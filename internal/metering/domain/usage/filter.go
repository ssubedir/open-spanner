package usage

import (
	"fmt"
	"strings"
	"time"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

type FilterType string
type FilterGroupOp string
type FilterConditionOp string

const (
	FilterTypeGroup     FilterType = "group"
	FilterTypeCondition FilterType = "condition"

	FilterGroupAnd FilterGroupOp = "and"
	FilterGroupOr  FilterGroupOp = "or"

	FilterOpEqual              FilterConditionOp = "eq"
	FilterOpNotEqual           FilterConditionOp = "neq"
	FilterOpGreaterThan        FilterConditionOp = "gt"
	FilterOpGreaterThanOrEqual FilterConditionOp = "gte"
	FilterOpLessThan           FilterConditionOp = "lt"
	FilterOpLessThanOrEqual    FilterConditionOp = "lte"
	FilterOpIn                 FilterConditionOp = "in"
	FilterOpContains           FilterConditionOp = "contains"
	FilterOpExists             FilterConditionOp = "exists"
)

type Filter struct {
	typ      FilterType
	groupOp  FilterGroupOp
	rules    []Filter
	field    string
	condOp   FilterConditionOp
	value    any
	hasValue bool
}

func EmptyFilter() Filter {
	return Filter{}
}

func NewFilterGroup(op FilterGroupOp, rules []Filter) (Filter, error) {
	switch op {
	case FilterGroupAnd, FilterGroupOr:
	default:
		return Filter{}, fmt.Errorf("%w: unsupported filter group operator %q", domain.ErrInvalidInput, op)
	}
	if len(rules) == 0 {
		return Filter{}, fmt.Errorf("%w: filter group requires at least one rule", domain.ErrInvalidInput)
	}

	copied := make([]Filter, len(rules))
	copy(copied, rules)
	return Filter{typ: FilterTypeGroup, groupOp: op, rules: copied}, nil
}

func NewFilterCondition(field string, op FilterConditionOp, value any, hasValue bool) (Filter, error) {
	field = strings.TrimSpace(field)
	if err := validateFilterField(field); err != nil {
		return Filter{}, err
	}
	switch op {
	case FilterOpEqual, FilterOpNotEqual, FilterOpGreaterThan, FilterOpGreaterThanOrEqual, FilterOpLessThan, FilterOpLessThanOrEqual, FilterOpIn, FilterOpContains, FilterOpExists:
	default:
		return Filter{}, fmt.Errorf("%w: unsupported filter operator %q", domain.ErrInvalidInput, op)
	}
	if op == FilterOpExists {
		return Filter{typ: FilterTypeCondition, field: field, condOp: op}, nil
	}
	if !hasValue {
		return Filter{}, fmt.Errorf("%w: filter value is required", domain.ErrInvalidInput)
	}
	if op == FilterOpIn {
		values, ok := value.([]any)
		if !ok || len(values) == 0 {
			return Filter{}, fmt.Errorf("%w: in filter requires a non-empty value array", domain.ErrInvalidInput)
		}
	}

	return Filter{typ: FilterTypeCondition, field: field, condOp: op, value: value, hasValue: true}, nil
}

func validateFilterField(field string) error {
	switch field {
	case "subject", "meter", "quantity", "timestamp", "event_time", "received_at", "idempotency_key":
		return nil
	}
	if strings.HasPrefix(field, "metadata.") && len(strings.TrimPrefix(field, "metadata.")) > 0 {
		return nil
	}
	return fmt.Errorf("%w: unsupported filter field %q", domain.ErrInvalidInput, field)
}

func (f Filter) IsZero() bool {
	return f.typ == ""
}

func (f Filter) Type() FilterType {
	return f.typ
}

func (f Filter) GroupOp() FilterGroupOp {
	return f.groupOp
}

func (f Filter) Rules() []Filter {
	rules := make([]Filter, len(f.rules))
	copy(rules, f.rules)
	return rules
}

func (f Filter) Field() string {
	return f.field
}

func (f Filter) ConditionOp() FilterConditionOp {
	return f.condOp
}

func (f Filter) Value() any {
	return f.value
}

func (f Filter) Matches(event Event) bool {
	if f.IsZero() {
		return true
	}
	switch f.Type() {
	case FilterTypeGroup:
		if f.GroupOp() == FilterGroupOr {
			for _, rule := range f.rules {
				if rule.Matches(event) {
					return true
				}
			}
			return false
		}
		for _, rule := range f.rules {
			if !rule.Matches(event) {
				return false
			}
		}
		return true
	case FilterTypeCondition:
		actual, exists := filterValue(event, f.field)
		return compareFilterValue(actual, exists, f.condOp, f.value)
	default:
		return true
	}
}

func filterValue(event Event, field string) (any, bool) {
	switch field {
	case "subject":
		return event.Subject(), true
	case "meter":
		return event.MeterName(), true
	case "quantity":
		return event.Quantity(), true
	case "timestamp", "event_time":
		return event.EventTime(), true
	case "received_at":
		return event.ReceivedAt(), true
	case "idempotency_key":
		return event.IdempotencyKey(), event.IdempotencyKey() != ""
	default:
		key := strings.TrimPrefix(field, "metadata.")
		return metadataPathValue(event.Metadata(), key)
	}
}

func metadataPathValue(metadata map[string]any, key string) (any, bool) {
	if value, exists := metadata[key]; exists {
		return value, true
	}

	parts := strings.Split(key, ".")
	if len(parts) == 1 {
		return nil, false
	}

	var current any = metadata
	for _, part := range parts {
		node, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		value, exists := node[part]
		if !exists {
			return nil, false
		}
		current = value
	}
	return current, true
}

func compareFilterValue(actual any, exists bool, op FilterConditionOp, expected any) bool {
	if op == FilterOpExists {
		return exists
	}
	if !exists {
		return false
	}
	if op == FilterOpIn {
		values, ok := expected.([]any)
		if !ok {
			return false
		}
		for _, value := range values {
			if compareFilterValue(actual, true, FilterOpEqual, value) {
				return true
			}
		}
		return false
	}

	order, comparable := compareOrder(actual, expected)
	switch op {
	case FilterOpEqual:
		return comparable && order == 0
	case FilterOpNotEqual:
		return !comparable || order != 0
	case FilterOpGreaterThan:
		return comparable && order > 0
	case FilterOpGreaterThanOrEqual:
		return comparable && order >= 0
	case FilterOpLessThan:
		return comparable && order < 0
	case FilterOpLessThanOrEqual:
		return comparable && order <= 0
	case FilterOpContains:
		return strings.Contains(metadataValueString(actual), metadataValueString(expected))
	default:
		return false
	}
}

func compareOrder(actual any, expected any) (int, bool) {
	if actualTime, ok := actual.(time.Time); ok {
		expectedTime, err := time.Parse(time.RFC3339Nano, metadataValueString(expected))
		if err != nil {
			return 0, false
		}
		if actualTime.Equal(expectedTime) {
			return 0, true
		}
		if actualTime.After(expectedTime) {
			return 1, true
		}
		return -1, true
	}

	actualNumber, actualOK := numberValue(actual)
	expectedNumber, expectedOK := numberValue(expected)
	if actualOK && expectedOK {
		if actualNumber == expectedNumber {
			return 0, true
		}
		if actualNumber > expectedNumber {
			return 1, true
		}
		return -1, true
	}

	actualText := metadataValueString(actual)
	expectedText := metadataValueString(expected)
	if actualText == expectedText {
		return 0, true
	}
	if actualText > expectedText {
		return 1, true
	}
	return -1, true
}

func numberValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int8:
		return float64(typed), true
	case int16:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint8:
		return float64(typed), true
	case uint16:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	default:
		return 0, false
	}
}
