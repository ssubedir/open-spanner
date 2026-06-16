package meter

import (
	"errors"
	"testing"
	"time"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

func TestMeterDimensionsValidateOptionalMetadata(t *testing.T) {
	region, err := NewDimension("region", MetadataString, "Region", "Deployment region", true)
	if err != nil {
		t.Fatalf("new region dimension: %v", err)
	}
	status, err := NewDimension("status", MetadataNumber, "Status", "HTTP status", false)
	if err != nil {
		t.Fatalf("new status dimension: %v", err)
	}
	meter, err := NewWithDimensions("meter-1", "api_calls", "", "request", AggregationSum, []Dimension{region, status}, 90, time.Now())
	if err != nil {
		t.Fatalf("new meter: %v", err)
	}

	if err := meter.ValidateMetadata(map[string]any{"region": "us-east", "request_id": "req_123"}); err != nil {
		t.Fatalf("validate without optional metadata and with extra metadata: %v", err)
	}
	if err := meter.ValidateMetadata(map[string]any{"region": "us-east", "status": "200"}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("validate wrong optional metadata type error = %v, want ErrInvalidInput", err)
	}
	if err := meter.ValidateMetadata(map[string]any{"status": 200}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("validate missing required metadata error = %v, want ErrInvalidInput", err)
	}
}

func TestMeterDimensionsNormalizeDottedMetadata(t *testing.T) {
	tier, err := NewDimension("service.tier", MetadataString, "Service tier", "", true)
	if err != nil {
		t.Fatalf("new tier dimension: %v", err)
	}
	meter, err := NewWithDimensions("meter-1", "api_calls", "", "request", AggregationSum, []Dimension{tier}, 90, time.Now())
	if err != nil {
		t.Fatalf("new meter: %v", err)
	}

	normalized, err := meter.NormalizeMetadata(map[string]any{
		"service.tier": "gold",
		"region":       "us-east",
	})
	if err != nil {
		t.Fatalf("normalize flat dotted metadata: %v", err)
	}
	service, ok := normalized["service"].(map[string]any)
	if !ok || service["tier"] != "gold" {
		t.Fatalf("normalized service metadata = %#v", normalized["service"])
	}
	if _, exists := normalized["service.tier"]; exists {
		t.Fatalf("normalized metadata kept flat service.tier key: %#v", normalized)
	}
	if normalized["region"] != "us-east" {
		t.Fatalf("normalized metadata dropped extra field: %#v", normalized)
	}

	if _, err := meter.NormalizeMetadata(map[string]any{
		"service": map[string]any{"tier": "gold"},
	}); err != nil {
		t.Fatalf("normalize nested metadata: %v", err)
	}
}

func TestMeterDimensionsTreatDeprecatedAsOptional(t *testing.T) {
	region, err := NewDimension("region", MetadataString, "Region", "Deployment region", true, true)
	if err != nil {
		t.Fatalf("new deprecated region dimension: %v", err)
	}
	meter, err := NewWithDimensions("meter-1", "api_calls", "", "request", AggregationSum, []Dimension{region}, 90, time.Now())
	if err != nil {
		t.Fatalf("new meter: %v", err)
	}

	if err := meter.ValidateMetadata(map[string]any{}); err != nil {
		t.Fatalf("validate without deprecated required metadata: %v", err)
	}
	if err := meter.ValidateMetadata(map[string]any{"region": 12}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("validate wrong deprecated metadata type error = %v, want ErrInvalidInput", err)
	}
}

func TestMeterDimensionsValidateQueryableNames(t *testing.T) {
	for _, name := range []string{"region-name", "service.tier", "status_code"} {
		if _, err := NewDimension(name, MetadataString, "", "", false); err != nil {
			t.Fatalf("new dimension %q: %v", name, err)
		}
	}

	for _, name := range []string{"region name", "region/name", ".region", "region.", "region..name", "subject"} {
		if _, err := NewDimension(name, MetadataString, "", "", false); !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("new dimension %q error = %v, want ErrInvalidInput", name, err)
		}
	}
}
