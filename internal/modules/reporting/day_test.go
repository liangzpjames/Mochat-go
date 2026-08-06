package reporting

import "testing"

func TestParseReportDay(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  string
	}{
		{input: "2026-08-07", want: "2026-08-07"},
		{input: "2026-08-07T00:00:00+08:00", want: "2026-08-07"},
		{input: "2026-08-07T16:00:00Z", want: "2026-08-07"},
		{input: "", want: "0001-01-01"},
	} {
		got := parseReportDay(tc.input).Format("2006-01-02")
		if got != tc.want {
			t.Fatalf("parseReportDay(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
