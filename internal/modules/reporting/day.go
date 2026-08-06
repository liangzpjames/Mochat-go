package reporting

import "time"

// parseReportDay tolerates both plain "2006-01-02" dates and RFC3339 values
// (the MySQL driver returns temporal columns as time.Time when the DSN uses
// parseTime=true, which database/sql formats as RFC3339 when scanned into a
// string destination).
func parseReportDay(day string) time.Time {
	if t, err := time.Parse("2006-01-02", day); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339Nano, day); err == nil {
		return t
	}
	return time.Time{}
}
