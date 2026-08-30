package store

import (
	"fmt"
	"time"
)

type corpTime struct {
	Time  time.Time
	Valid bool
}

func (value *corpTime) Scan(source any) error {
	if source == nil {
		value.Time = time.Time{}
		value.Valid = false
		return nil
	}
	if parsed, ok := source.(time.Time); ok {
		value.Time = parsed
		value.Valid = true
		return nil
	}
	var raw string
	switch typed := source.(type) {
	case []byte:
		raw = string(typed)
	case string:
		raw = typed
	default:
		return fmt.Errorf("unsupported corp data update time type %T", source)
	}
	parsed, err := time.ParseInLocation("2006-01-02 15:04:05", raw, time.FixedZone("UTC+08:00", 8*60*60))
	if err != nil {
		return fmt.Errorf("parse corp data update time: %w", err)
	}
	value.Time = parsed
	value.Valid = true
	return nil
}
