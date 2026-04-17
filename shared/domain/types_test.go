package domain

import (
	"testing"

	"github.com/AbdeljalilB92/lldl/shared/errors"
)

func TestQualityFromString_ValidInputs(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  Quality
	}{
		{"numeric high", "720", QualityHigh},
		{"numeric medium", "540", QualityMedium},
		{"numeric low", "360", QualityLow},
		{"friendly high", "high", QualityHigh},
		{"friendly medium", "medium", QualityMedium},
		{"friendly low", "low", QualityLow},
		{"p-suffix high", "720p", QualityHigh},
		{"p-suffix medium", "540p", QualityMedium},
		{"p-suffix low", "360p", QualityLow},
		{"menu index high", "1", QualityHigh},
		{"menu index medium", "2", QualityMedium},
		{"menu index low", "3", QualityLow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := QualityFromString(tt.input)
			if err != nil {
				t.Fatalf("QualityFromString(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("QualityFromString(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestQualityFromString_InvalidInput(t *testing.T) {
	_, err := QualityFromString("1080")
	if err == nil {
		t.Fatalf("QualityFromString(\"1080\") expected error, got nil")
	}
	// Verify the concrete error type so callers can type-switch on ValidationError.
	ve, ok := err.(*errors.ValidationError)
	if !ok {
		t.Fatalf("expected *errors.ValidationError, got %T", err)
	}
	if ve.Field != "quality" {
		t.Errorf("ValidationError.Field = %q, want %q", ve.Field, "quality")
	}
}

func TestQuality_String(t *testing.T) {
	tests := []struct {
		q    Quality
		want string
	}{
		{QualityHigh, "720"},
		{QualityMedium, "540"},
		{QualityLow, "360"},
		{Quality(0), "0"},
	}
	for _, tt := range tests {
		got := tt.q.String()
		if got != tt.want {
			t.Errorf("Quality(%d).String() = %q, want %q", tt.q, got, tt.want)
		}
	}
}
