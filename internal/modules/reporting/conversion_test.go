package reporting

import "testing"

func TestConversionResultUsesNullForZeroRate(t *testing.T) {
	r := ConversionResult(ConversionCounts{Lead: 0, Contact: 0})
	if Ratio(*r.Summary["contact"], *r.Summary["lead"]) != nil {
		t.Fatal("expected null")
	}
}
