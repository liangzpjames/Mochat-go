package mysqlconn

import (
	"testing"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
)

func TestConfigWithSessionTimeZoneUsesDSNLocation(t *testing.T) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	config := mysqldriver.NewConfig()
	config.Loc = location

	normalized, err := configWithSessionTimeZone(config, time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if got := normalized.Params[sessionTimeZoneParameter]; got != "'+08:00'" {
		t.Fatalf("time_zone = %q", got)
	}
	if config.Params != nil {
		t.Fatalf("input config was mutated: %+v", config.Params)
	}
}

func TestConfigWithSessionTimeZonePreservesExplicitValue(t *testing.T) {
	config := mysqldriver.NewConfig()
	config.Loc = time.FixedZone("custom", 8*60*60)
	config.Params = map[string]string{"TIME_ZONE": "'+00:00'", "sql_mode": "'STRICT_ALL_TABLES'"}

	normalized, err := configWithSessionTimeZone(config, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(normalized.Params) != 2 || normalized.Params["TIME_ZONE"] != "'+00:00'" {
		t.Fatalf("params = %+v", normalized.Params)
	}
}

func TestSessionTimeZoneLiteralRejectsUnsupportedOffset(t *testing.T) {
	if _, err := sessionTimeZoneLiteral(time.FixedZone("invalid", 15*60*60), time.Now()); err == nil {
		t.Fatal("expected unsupported offset error")
	}
}
