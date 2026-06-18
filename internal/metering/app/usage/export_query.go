package usage

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

type ExportQuery struct {
	Subject    string        `json:"subject,omitempty"`
	Meter      string        `json:"meter"`
	From       string        `json:"from"`
	To         string        `json:"to"`
	BucketSize string        `json:"bucket_size"`
	GroupBy    ExportGroupBy `json:"group_by,omitempty"`
	Limit      int           `json:"limit,omitempty"`
	Filter     *ExportFilter `json:"filter,omitempty"`
}

type ExportFilter struct {
	Type  string         `json:"type"`
	Op    string         `json:"op"`
	Rules []ExportFilter `json:"rules,omitempty"`
	Field string         `json:"field,omitempty"`
	Value any            `json:"value,omitempty"`
}

type ExportGroupBy []string

func (g *ExportGroupBy) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*g = nil
		return nil
	}

	var values []string
	if err := json.Unmarshal(data, &values); err == nil {
		*g = ExportGroupBy(domainusage.SplitGroupByValues(values))
		return nil
	}

	var value string
	if err := json.Unmarshal(data, &value); err == nil {
		*g = ExportGroupBy(domainusage.SplitGroupBy(value))
		return nil
	}

	return fmt.Errorf("%w: group_by must be a string or array of strings", domain.ErrInvalidInput)
}

func ParseExportListQueryJSON(payload string) (ListQuery, error) {
	var query ExportQuery
	if err := json.Unmarshal([]byte(payload), &query); err != nil {
		return ListQuery{}, fmt.Errorf("%w: export query must be valid JSON", domain.ErrInvalidInput)
	}
	return ExportListQuery(query)
}

func ExportListQuery(query ExportQuery) (ListQuery, error) {
	from, err := requiredExportTime("from", query.From)
	if err != nil {
		return ListQuery{}, err
	}
	to, err := requiredExportTime("to", query.To)
	if err != nil {
		return ListQuery{}, err
	}
	filter, err := exportFilter(query.Filter)
	if err != nil {
		return ListQuery{}, err
	}

	return ListQuery{
		Subject:    query.Subject,
		MeterName:  query.Meter,
		From:       from,
		To:         to,
		BucketSize: domainusage.BucketSize(query.BucketSize),
		GroupBy:    query.GroupBy.Fields(),
		Limit:      query.Limit,
		Filter:     filter,
	}, nil
}

func (g ExportGroupBy) Fields() []string {
	fields := make([]string, len(g))
	copy(fields, g)
	return fields
}

func exportFilter(input *ExportFilter) (domainusage.Filter, error) {
	if input == nil {
		return domainusage.EmptyFilter(), nil
	}

	switch domainusage.FilterType(input.Type) {
	case domainusage.FilterTypeGroup:
		rules := make([]domainusage.Filter, 0, len(input.Rules))
		for _, child := range input.Rules {
			rule, err := exportFilter(&child)
			if err != nil {
				return domainusage.Filter{}, err
			}
			rules = append(rules, rule)
		}
		return domainusage.NewFilterGroup(domainusage.FilterGroupOp(input.Op), rules)
	case domainusage.FilterTypeCondition:
		return domainusage.NewFilterCondition(input.Field, domainusage.FilterConditionOp(input.Op), input.Value, input.Value != nil)
	default:
		return domainusage.Filter{}, fmt.Errorf("%w: unsupported filter type %q", domain.ErrInvalidInput, input.Type)
	}
}

func requiredExportTime(field string, value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, fmt.Errorf("%w: %s is required", domain.ErrInvalidInput, field)
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %s must be RFC3339", domain.ErrInvalidInput, field)
	}
	return parsed.UTC(), nil
}
