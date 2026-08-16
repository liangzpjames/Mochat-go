package reporting

import (
	"testing"
	"time"
)

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

func TestConversationTrendDaysFollowsQueryTimezone(t *testing.T) {
	for _, tc := range []struct {
		name     string
		trendEnd time.Time
		timezone string
		want     []string
	}{
		{
			name:     "shanghai midnight maps to local days",
			trendEnd: time.Date(2026, 8, 15, 16, 0, 0, 0, time.UTC), // 2026-08-16T00:00+08:00
			timezone: "Asia/Shanghai",
			want:     []string{"2026-08-09", "2026-08-10", "2026-08-11", "2026-08-12", "2026-08-13", "2026-08-14", "2026-08-15"},
		},
		{
			name:     "utc midnight maps to utc days",
			trendEnd: time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC),
			timezone: "UTC",
			want:     []string{"2026-08-09", "2026-08-10", "2026-08-11", "2026-08-12", "2026-08-13", "2026-08-14", "2026-08-15"},
		},
		{
			name:     "invalid timezone falls back to utc",
			trendEnd: time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC),
			timezone: "Not/AZone",
			want:     []string{"2026-08-09", "2026-08-10", "2026-08-11", "2026-08-12", "2026-08-13", "2026-08-14", "2026-08-15"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := conversationTrendDays(tc.trendEnd, tc.timezone)
			if len(got) != len(tc.want) {
				t.Fatalf("conversationTrendDays(%v, %q) = %v, want %v", tc.trendEnd, tc.timezone, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("conversationTrendDays(%v, %q)[%d] = %q, want %q (full %v)", tc.trendEnd, tc.timezone, i, got[i], tc.want[i], got)
				}
			}
		})
	}
}

func TestFillSeriesWindowCoversEveryLocalDay(t *testing.T) {
	start := time.Date(2026, 8, 9, 16, 0, 0, 0, time.UTC) // 2026-08-10T00:00+08:00
	end := time.Date(2026, 8, 16, 16, 0, 0, 0, time.UTC) // 2026-08-17T00:00+08:00
	points := []SeriesPoint{
		{At: time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC), Value: 3},
		{At: time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC), Value: 5},
	}
	got := fillSeriesWindow(points, start, end, "Asia/Shanghai")
	if len(got) != 7 {
		t.Fatalf("fillSeriesWindow returned %d points, want 7: %+v", len(got), got)
	}
	wantDates := []string{"2026-08-10", "2026-08-11", "2026-08-12", "2026-08-13", "2026-08-14", "2026-08-15", "2026-08-16"}
	wantValues := []float64{0, 0, 3, 0, 0, 5, 0}
	for i := range wantDates {
		if got[i].At.Format("2006-01-02") != wantDates[i] {
			t.Fatalf("fillSeriesWindow[%d] date = %q, want %q", i, got[i].At.Format("2006-01-02"), wantDates[i])
		}
		if got[i].Value != wantValues[i] {
			t.Fatalf("fillSeriesWindow[%d] value = %v, want %v", i, got[i].Value, wantValues[i])
		}
	}
}
