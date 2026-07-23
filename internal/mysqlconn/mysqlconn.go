package mysqlconn

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
)

const sessionTimeZoneParameter = "time_zone"

func Open(dsn string) (*sql.DB, error) {
	config, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		return nil, err
	}
	return OpenConfig(config)
}

func OpenConfig(config *mysqldriver.Config) (*sql.DB, error) {
	normalized, err := configWithSessionTimeZone(config, time.Now())
	if err != nil {
		return nil, err
	}
	connector, err := mysqldriver.NewConnector(normalized)
	if err != nil {
		return nil, err
	}
	return sql.OpenDB(connector), nil
}

func configWithSessionTimeZone(config *mysqldriver.Config, now time.Time) (*mysqldriver.Config, error) {
	if config == nil {
		return nil, errors.New("mysql config is required")
	}
	normalized := config.Clone()
	for key := range normalized.Params {
		if strings.EqualFold(key, sessionTimeZoneParameter) {
			return normalized, nil
		}
	}
	literal, err := sessionTimeZoneLiteral(normalized.Loc, now)
	if err != nil {
		return nil, err
	}
	if normalized.Params == nil {
		normalized.Params = make(map[string]string)
	}
	normalized.Params[sessionTimeZoneParameter] = literal
	return normalized, nil
}

func sessionTimeZoneLiteral(location *time.Location, now time.Time) (string, error) {
	if location == nil {
		location = time.UTC
	}
	_, offsetSeconds := now.In(location).Zone()
	if offsetSeconds%60 != 0 {
		return "", errors.New("mysql session time zone offset must use whole minutes")
	}
	sign := '+'
	if offsetSeconds < 0 {
		sign = '-'
		offsetSeconds = -offsetSeconds
	}
	if offsetSeconds > 14*60*60 {
		return "", errors.New("mysql session time zone offset exceeds 14 hours")
	}
	hours := offsetSeconds / 3600
	minutes := (offsetSeconds % 3600) / 60
	return fmt.Sprintf("'%c%02d:%02d'", sign, hours, minutes), nil
}
