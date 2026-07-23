package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
)

type ContactSOPLogSource struct {
	ID             int
	CorpID         int
	SettingRaw     string
	EmployeeIDsRaw string
	ContactIDsRaw  string
}

type RoomSOPLogSource struct {
	ID         int
	CorpID     int
	SettingRaw string
	RoomIDsRaw string
}

type ContactSOPLogTarget struct {
	EmployeeWXUserID      string
	ContactWXExternalUser string
	AnchorTime            time.Time
}

type RoomSOPLogTarget struct {
	RoomID                int
	EmployeeWXUserID      string
	ContactWXExternalUser string
	AnchorTime            time.Time
}

type ContactSOPLogCreate struct {
	CorpID                int
	ContactSOPID          int
	EmployeeWXUserID      string
	ContactWXExternalUser string
	TaskRaw               string
	CreatedAt             time.Time
}

type RoomSOPLogCreate struct {
	CorpID                int
	RoomSOPID             int
	RoomID                int
	EmployeeWXUserID      string
	ContactWXExternalUser string
	TaskRaw               string
	CreatedAt             time.Time
}

type SOPLogCronResult struct {
	ContactSourcesScanned int
	RoomSourcesScanned    int
	ContactLogsInserted   int
	RoomLogsInserted      int
	ItemsSkipped          int
	ItemsFailed           int
}

type SOPLogCronStore interface {
	ActiveContactSOPLogSources(ctx context.Context) ([]ContactSOPLogSource, error)
	ActiveRoomSOPLogSources(ctx context.Context) ([]RoomSOPLogSource, error)
	ContactSOPLogTargets(ctx context.Context, corpID int, employeeIDs []int, contactIDs []int) ([]ContactSOPLogTarget, error)
	RoomSOPLogTargets(ctx context.Context, corpID int, roomIDs []int, targetAnchor string) ([]RoomSOPLogTarget, error)
	RoomIDByWXChatID(ctx context.Context, corpID int, wxChatID string) (int, bool, error)
	InsertContactSOPLog(ctx context.Context, log ContactSOPLogCreate) (bool, error)
	InsertRoomSOPLog(ctx context.Context, log RoomSOPLogCreate) (bool, error)
}

const SOPLogTargetAnchorRoomJoin = "room_join"

type SOPLogCron struct {
	store  SOPLogCronStore
	logger *log.Logger
	now    func() time.Time
}

func NewSOPLogCron(store SOPLogCronStore, logger *log.Logger) *SOPLogCron {
	if logger == nil {
		logger = log.Default()
	}
	return &SOPLogCron{store: store, logger: logger, now: time.Now}
}

func (c *SOPLogCron) RunOnce(ctx context.Context) error {
	if c.store == nil {
		return fmt.Errorf("SOP log cron dependencies are not configured")
	}
	result := SOPLogCronResult{}
	now := c.now()
	var firstErr error
	contactResult, err := c.runContact(ctx, now)
	mergeSOPLogCronResult(&result, contactResult)
	if err != nil {
		firstErr = err
	}
	roomResult, err := c.runRoom(ctx, now)
	mergeSOPLogCronResult(&result, roomResult)
	if err != nil && firstErr == nil {
		firstErr = err
	}
	c.logger.Printf("SOP log cron finished: contact_sources=%d room_sources=%d contact_logs_inserted=%d room_logs_inserted=%d skipped=%d failed=%d", result.ContactSourcesScanned, result.RoomSourcesScanned, result.ContactLogsInserted, result.RoomLogsInserted, result.ItemsSkipped, result.ItemsFailed)
	return firstErr
}

func (c *SOPLogCron) RunContactTarget(ctx context.Context, corpID int, employeeID int, contactID int) (SOPLogCronResult, error) {
	if c.store == nil {
		return SOPLogCronResult{}, fmt.Errorf("SOP log cron dependencies are not configured")
	}
	if corpID <= 0 || employeeID <= 0 || contactID <= 0 {
		return SOPLogCronResult{}, nil
	}
	sources, err := c.store.ActiveContactSOPLogSources(ctx)
	if err != nil {
		return SOPLogCronResult{}, err
	}
	now := c.now()
	targets, err := c.store.ContactSOPLogTargets(ctx, corpID, []int{employeeID}, []int{contactID})
	if err != nil {
		return SOPLogCronResult{}, err
	}
	if len(targets) == 0 {
		return SOPLogCronResult{ItemsSkipped: 1}, nil
	}
	result := SOPLogCronResult{}
	var firstErr error
	for _, source := range sources {
		if source.CorpID != corpID || source.ID <= 0 {
			continue
		}
		employeeIDs, err := sopLogIDs(source.EmployeeIDsRaw)
		if err != nil {
			result.ItemsFailed++
			if firstErr == nil {
				firstErr = fmt.Errorf("contact sop %d employee ids: %w", source.ID, err)
			}
			continue
		}
		contactIDs, err := sopLogIDs(source.ContactIDsRaw)
		if err != nil {
			result.ItemsFailed++
			if firstErr == nil {
				firstErr = fmt.Errorf("contact sop %d contact ids: %w", source.ID, err)
			}
			continue
		}
		if !sopLogContainsID(employeeIDs, employeeID) || !sopLogContainsID(contactIDs, contactID) {
			continue
		}
		result.ContactSourcesScanned++
		tasks, err := sopLogTasks(source.SettingRaw, now)
		if err != nil {
			result.ItemsFailed++
			if firstErr == nil {
				firstErr = fmt.Errorf("contact sop %d tasks: %w", source.ID, err)
			}
			continue
		}
		if len(tasks) == 0 {
			result.ItemsSkipped++
			continue
		}
		for _, task := range tasks {
			for _, target := range targets {
				createdAt, due := task.createdAtForTarget(target.AnchorTime, now)
				if !due {
					continue
				}
				inserted, err := c.store.InsertContactSOPLog(ctx, ContactSOPLogCreate{
					CorpID:                source.CorpID,
					ContactSOPID:          source.ID,
					EmployeeWXUserID:      target.EmployeeWXUserID,
					ContactWXExternalUser: target.ContactWXExternalUser,
					TaskRaw:               task.Raw,
					CreatedAt:             createdAt,
				})
				if err != nil {
					result.ItemsFailed++
					if firstErr == nil {
						firstErr = fmt.Errorf("contact sop %d insert: %w", source.ID, err)
					}
					continue
				}
				if inserted {
					result.ContactLogsInserted++
				}
			}
		}
	}
	return result, firstErr
}

func (c *SOPLogCron) RunRoomJoinTargetByWXChatID(ctx context.Context, corpID int, wxChatID string) (SOPLogCronResult, error) {
	if c.store == nil {
		return SOPLogCronResult{}, fmt.Errorf("SOP log cron dependencies are not configured")
	}
	wxChatID = strings.TrimSpace(wxChatID)
	if corpID <= 0 || wxChatID == "" {
		return SOPLogCronResult{}, nil
	}
	roomID, found, err := c.store.RoomIDByWXChatID(ctx, corpID, wxChatID)
	if err != nil {
		return SOPLogCronResult{}, err
	}
	if !found || roomID <= 0 {
		return SOPLogCronResult{ItemsSkipped: 1}, nil
	}
	sources, err := c.store.ActiveRoomSOPLogSources(ctx)
	if err != nil {
		return SOPLogCronResult{}, err
	}
	now := c.now()
	result := SOPLogCronResult{}
	var firstErr error
	for _, source := range sources {
		if source.CorpID != corpID || source.ID <= 0 {
			continue
		}
		roomIDs, err := sopLogIDs(source.RoomIDsRaw)
		if err != nil {
			result.ItemsFailed++
			if firstErr == nil {
				firstErr = fmt.Errorf("room sop %d room ids: %w", source.ID, err)
			}
			continue
		}
		if !sopLogContainsID(roomIDs, roomID) {
			continue
		}
		result.RoomSourcesScanned++
		tasks, err := sopLogTasks(source.SettingRaw, now)
		if err != nil {
			result.ItemsFailed++
			if firstErr == nil {
				firstErr = fmt.Errorf("room sop %d tasks: %w", source.ID, err)
			}
			continue
		}
		if len(tasks) == 0 {
			result.ItemsSkipped++
			continue
		}
		targets, err := c.store.RoomSOPLogTargets(ctx, source.CorpID, []int{roomID}, SOPLogTargetAnchorRoomJoin)
		if err != nil {
			result.ItemsFailed++
			if firstErr == nil {
				firstErr = fmt.Errorf("room sop %d targets: %w", source.ID, err)
			}
			continue
		}
		if len(targets) == 0 {
			result.ItemsSkipped++
			continue
		}
		for _, task := range tasks {
			if !task.RequiresTargetAnchor || task.TargetAnchor != SOPLogTargetAnchorRoomJoin {
				continue
			}
			for _, target := range targets {
				createdAt, due := task.createdAtForTarget(target.AnchorTime, now)
				if !due {
					continue
				}
				inserted, err := c.store.InsertRoomSOPLog(ctx, RoomSOPLogCreate{
					CorpID:                source.CorpID,
					RoomSOPID:             source.ID,
					RoomID:                target.RoomID,
					EmployeeWXUserID:      target.EmployeeWXUserID,
					ContactWXExternalUser: target.ContactWXExternalUser,
					TaskRaw:               task.Raw,
					CreatedAt:             createdAt,
				})
				if err != nil {
					result.ItemsFailed++
					if firstErr == nil {
						firstErr = fmt.Errorf("room sop %d insert: %w", source.ID, err)
					}
					continue
				}
				if inserted {
					result.RoomLogsInserted++
				}
			}
		}
	}
	return result, firstErr
}

func (c *SOPLogCron) runContact(ctx context.Context, now time.Time) (SOPLogCronResult, error) {
	sources, err := c.store.ActiveContactSOPLogSources(ctx)
	if err != nil {
		return SOPLogCronResult{}, err
	}
	result := SOPLogCronResult{}
	var firstErr error
	for _, source := range sources {
		if source.ID <= 0 || source.CorpID <= 0 {
			result.ItemsSkipped++
			continue
		}
		result.ContactSourcesScanned++
		tasks, err := sopLogTasks(source.SettingRaw, now)
		if err != nil {
			result.ItemsFailed++
			if firstErr == nil {
				firstErr = fmt.Errorf("contact sop %d tasks: %w", source.ID, err)
			}
			continue
		}
		employeeIDs, err := sopLogIDs(source.EmployeeIDsRaw)
		if err != nil {
			result.ItemsFailed++
			if firstErr == nil {
				firstErr = fmt.Errorf("contact sop %d employee ids: %w", source.ID, err)
			}
			continue
		}
		contactIDs, err := sopLogIDs(source.ContactIDsRaw)
		if err != nil {
			result.ItemsFailed++
			if firstErr == nil {
				firstErr = fmt.Errorf("contact sop %d contact ids: %w", source.ID, err)
			}
			continue
		}
		if len(tasks) == 0 || len(employeeIDs) == 0 || len(contactIDs) == 0 {
			result.ItemsSkipped++
			continue
		}
		targets, err := c.store.ContactSOPLogTargets(ctx, source.CorpID, employeeIDs, contactIDs)
		if err != nil {
			result.ItemsFailed++
			if firstErr == nil {
				firstErr = fmt.Errorf("contact sop %d targets: %w", source.ID, err)
			}
			continue
		}
		if len(targets) == 0 {
			result.ItemsSkipped++
			continue
		}
		for _, task := range tasks {
			for _, target := range targets {
				createdAt, due := task.createdAtForTarget(target.AnchorTime, now)
				if !due {
					continue
				}
				inserted, err := c.store.InsertContactSOPLog(ctx, ContactSOPLogCreate{
					CorpID:                source.CorpID,
					ContactSOPID:          source.ID,
					EmployeeWXUserID:      target.EmployeeWXUserID,
					ContactWXExternalUser: target.ContactWXExternalUser,
					TaskRaw:               task.Raw,
					CreatedAt:             createdAt,
				})
				if err != nil {
					result.ItemsFailed++
					if firstErr == nil {
						firstErr = fmt.Errorf("contact sop %d insert: %w", source.ID, err)
					}
					continue
				}
				if inserted {
					result.ContactLogsInserted++
				}
			}
		}
	}
	return result, firstErr
}

func (c *SOPLogCron) runRoom(ctx context.Context, now time.Time) (SOPLogCronResult, error) {
	sources, err := c.store.ActiveRoomSOPLogSources(ctx)
	if err != nil {
		return SOPLogCronResult{}, err
	}
	result := SOPLogCronResult{}
	var firstErr error
	for _, source := range sources {
		if source.ID <= 0 || source.CorpID <= 0 {
			result.ItemsSkipped++
			continue
		}
		result.RoomSourcesScanned++
		tasks, err := sopLogTasks(source.SettingRaw, now)
		if err != nil {
			result.ItemsFailed++
			if firstErr == nil {
				firstErr = fmt.Errorf("room sop %d tasks: %w", source.ID, err)
			}
			continue
		}
		roomIDs, err := sopLogIDs(source.RoomIDsRaw)
		if err != nil {
			result.ItemsFailed++
			if firstErr == nil {
				firstErr = fmt.Errorf("room sop %d room ids: %w", source.ID, err)
			}
			continue
		}
		if len(tasks) == 0 || len(roomIDs) == 0 {
			result.ItemsSkipped++
			continue
		}
		for _, task := range tasks {
			targets, err := c.store.RoomSOPLogTargets(ctx, source.CorpID, roomIDs, task.TargetAnchor)
			if err != nil {
				result.ItemsFailed++
				if firstErr == nil {
					firstErr = fmt.Errorf("room sop %d targets: %w", source.ID, err)
				}
				continue
			}
			if len(targets) == 0 {
				result.ItemsSkipped++
				continue
			}
			for _, target := range targets {
				createdAt, due := task.createdAtForTarget(target.AnchorTime, now)
				if !due {
					continue
				}
				inserted, err := c.store.InsertRoomSOPLog(ctx, RoomSOPLogCreate{
					CorpID:                source.CorpID,
					RoomSOPID:             source.ID,
					RoomID:                target.RoomID,
					EmployeeWXUserID:      target.EmployeeWXUserID,
					ContactWXExternalUser: target.ContactWXExternalUser,
					TaskRaw:               task.Raw,
					CreatedAt:             createdAt,
				})
				if err != nil {
					result.ItemsFailed++
					if firstErr == nil {
						firstErr = fmt.Errorf("room sop %d insert: %w", source.ID, err)
					}
					continue
				}
				if inserted {
					result.RoomLogsInserted++
				}
			}
		}
	}
	return result, firstErr
}

type sopLogTask struct {
	Raw                  string
	CreatedAt            time.Time
	TargetAnchorDelay    time.Duration
	TargetAnchor         string
	RequiresTargetAnchor bool
}

type sopLogRecurrence struct {
	Period       string
	Weekdays     map[time.Weekday]struct{}
	MonthDays    map[int]struct{}
	Dates        map[string]struct{}
	MonthDates   map[string]struct{}
	HasWeekdays  bool
	HasMonthDays bool
	HasDates     bool
}

func (t sopLogTask) createdAtForTarget(anchor time.Time, now time.Time) (time.Time, bool) {
	if !t.RequiresTargetAnchor {
		return t.CreatedAt, true
	}
	if anchor.IsZero() {
		return time.Time{}, false
	}
	scheduled := anchor.Add(t.TargetAnchorDelay)
	return scheduled, !scheduled.After(now)
}

func sopLogTasks(raw string, now time.Time) ([]sopLogTask, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" || raw == "{}" {
		return []sopLogTask{}, nil
	}
	var decoded any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		encoded, _ := json.Marshal(map[string]any{"content": []any{map[string]any{"type": "text", "value": raw}}})
		return []sopLogTask{{Raw: string(encoded), CreatedAt: now}}, nil
	}
	values := sopLogTaskValues(decoded)
	tasks := make([]sopLogTask, 0, len(values))
	for _, value := range values {
		createdAt, due, targetDelay, targetAnchor, requiresTargetAnchor, occurrenceKey := sopLogTaskSchedule(value, now)
		if !due && !requiresTargetAnchor {
			continue
		}
		encodedValue := value
		if occurrenceKey != "" {
			encodedValue = sopLogTaskWithOccurrence(value, occurrenceKey)
		}
		encoded, err := json.Marshal(encodedValue)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, sopLogTask{
			Raw:                  string(encoded),
			CreatedAt:            createdAt,
			TargetAnchorDelay:    targetDelay,
			TargetAnchor:         targetAnchor,
			RequiresTargetAnchor: requiresTargetAnchor,
		})
	}
	return tasks, nil
}

func sopLogTaskValues(decoded any) []any {
	switch value := decoded.(type) {
	case []any:
		if len(value) == 0 {
			return []any{}
		}
		if sopLogLooksLikeContent(value) {
			return []any{map[string]any{"content": value}}
		}
		return value
	case map[string]any:
		return []any{value}
	case string:
		if strings.TrimSpace(value) == "" {
			return []any{}
		}
		return []any{map[string]any{"content": []any{map[string]any{"type": "text", "value": value}}}}
	default:
		return []any{}
	}
}

func sopLogLooksLikeContent(values []any) bool {
	for _, value := range values {
		item, ok := value.(map[string]any)
		if !ok {
			return false
		}
		if _, hasContent := item["content"]; hasContent {
			return false
		}
		_, hasType := item["type"]
		_, hasValue := item["value"]
		if !hasType && !hasValue {
			return false
		}
	}
	return true
}

func sopLogTaskSchedule(value any, now time.Time) (time.Time, bool, time.Duration, string, bool, string) {
	item, ok := value.(map[string]any)
	if !ok {
		return now, true, 0, "", false, ""
	}
	if scheduled, parsed, occurrenceKey := parseSOPLogRecurringSchedule(item, now); parsed {
		return scheduled, !scheduled.After(now), 0, "", false, occurrenceKey
	}
	for _, key := range []string{"sendTime", "send_time", "remindTime", "remind_time", "tipTime", "tip_time", "triggerTime", "trigger_time", "executeTime", "execute_time", "time", "date"} {
		if raw, ok := item[key]; ok {
			if scheduled, parsed := parseSOPLogSchedule(raw, now); parsed {
				return scheduled, !scheduled.After(now), 0, "", false, ""
			}
		}
	}
	if scheduled, parsed := parseSOPLogRelativeSchedule(item, now); parsed {
		return scheduled, !scheduled.After(now), 0, "", false, ""
	}
	if delay, parsed := parseSOPLogRelativeDelay(item); parsed {
		return time.Time{}, false, delay, parseSOPLogTargetAnchor(item), true, ""
	}
	if hasSOPLogRelativeDelayKey(item) {
		return time.Time{}, false, 0, "", false, ""
	}
	return now, true, 0, "", false, ""
}

func sopLogTaskWithOccurrence(value any, occurrenceKey string) any {
	item, ok := value.(map[string]any)
	if !ok || occurrenceKey == "" {
		return value
	}
	clone := make(map[string]any, len(item)+1)
	for key, field := range item {
		clone[key] = field
	}
	clone["_mochatGoOccurrence"] = occurrenceKey
	return clone
}

func parseSOPLogSchedule(raw any, base time.Time) (time.Time, bool) {
	location := base.Location()
	switch value := raw.(type) {
	case string:
		value = strings.TrimSpace(value)
		if value == "" {
			return time.Time{}, false
		}
		layouts := []string{"2006-01-02 15:04:05", "2006-01-02 15:04", time.RFC3339, "2006-01-02"}
		for _, layout := range layouts {
			if parsed, err := time.ParseInLocation(layout, value, location); err == nil {
				return parsed, true
			}
		}
		if parsed, err := time.ParseInLocation("15:04:05", value, location); err == nil {
			return time.Date(base.Year(), base.Month(), base.Day(), parsed.Hour(), parsed.Minute(), parsed.Second(), 0, location), true
		}
		if parsed, err := time.ParseInLocation("15:04", value, location); err == nil {
			return time.Date(base.Year(), base.Month(), base.Day(), parsed.Hour(), parsed.Minute(), 0, 0, location), true
		}
	case float64:
		if value > 0 {
			return time.Unix(int64(value), 0).In(location), true
		}
	case int:
		if value > 0 {
			return time.Unix(int64(value), 0).In(location), true
		}
	case int64:
		if value > 0 {
			return time.Unix(value, 0).In(location), true
		}
	case json.Number:
		if intValue, err := value.Int64(); err == nil && intValue > 0 {
			return time.Unix(intValue, 0).In(location), true
		}
	}
	return time.Time{}, false
}

func parseSOPLogRecurringSchedule(item map[string]any, now time.Time) (time.Time, bool, string) {
	recurrence, found := parseSOPLogRecurrence(item, now)
	if !found {
		return time.Time{}, false, ""
	}
	if !recurrence.actionable() {
		return now.Add(24 * time.Hour), true, ""
	}
	if !recurrence.matches(now) {
		return now.Add(24 * time.Hour), true, ""
	}
	scheduled := now
	for _, key := range []string{"sendTime", "send_time", "remindTime", "remind_time", "tipTime", "tip_time", "triggerTime", "trigger_time", "executeTime", "execute_time", "time"} {
		if raw, ok := item[key]; ok {
			if parsed, ok := parseSOPLogSchedule(raw, now); ok {
				scheduled = parsed
				break
			}
		}
	}
	return scheduled, true, recurrence.occurrenceKey(scheduled)
}

func parseSOPLogRecurrence(item map[string]any, now time.Time) (sopLogRecurrence, bool) {
	recurrence := sopLogRecurrence{}
	if period, ok := sopLogRecurrencePeriod(item); ok {
		recurrence.Period = period
	}
	if weekdays, ok := sopLogRecurrenceWeekdays(item); ok {
		recurrence.Weekdays = weekdays
		recurrence.HasWeekdays = true
	}
	if monthDays, ok := sopLogRecurrenceMonthDays(item); ok {
		recurrence.MonthDays = monthDays
		recurrence.HasMonthDays = true
	}
	dates, monthDates, hasDates := sopLogRecurrenceDates(item, now)
	if hasDates {
		recurrence.Dates = dates
		recurrence.MonthDates = monthDates
		recurrence.HasDates = true
	}
	if recurrence.Period == "" && !recurrence.HasWeekdays && !recurrence.HasMonthDays && !recurrence.HasDates {
		return sopLogRecurrence{}, false
	}
	return recurrence, true
}

func sopLogRecurrencePeriod(item map[string]any) (string, bool) {
	for _, key := range []string{"cycle", "period", "repeat", "frequency", "freq"} {
		raw, ok := item[key]
		if !ok {
			continue
		}
		switch value := raw.(type) {
		case bool:
			if value {
				return "daily", true
			}
		case string:
			normalized := strings.ToLower(strings.TrimSpace(value))
			normalized = strings.ReplaceAll(normalized, "_", "")
			normalized = strings.ReplaceAll(normalized, "-", "")
			normalized = strings.ReplaceAll(normalized, " ", "")
			switch normalized {
			case "daily", "day", "everyday", "每天", "每日":
				return "daily", true
			case "weekly", "week", "everyweek", "每周", "每星期":
				return "weekly", true
			case "monthly", "month", "everymonth", "每月":
				return "monthly", true
			}
		}
	}
	return "", false
}

func sopLogRecurrenceWeekdays(item map[string]any) (map[time.Weekday]struct{}, bool) {
	values, ok := sopLogValuesForKeys(item, "weekdays", "weekDays", "week_days", "weekday", "weekDay", "week_day", "daysOfWeek", "days_of_week")
	if !ok {
		return nil, false
	}
	weekdays := map[time.Weekday]struct{}{}
	for _, value := range values {
		if weekday, parsed := parseSOPLogWeekday(value); parsed {
			weekdays[weekday] = struct{}{}
		}
	}
	if len(weekdays) == 0 {
		return nil, false
	}
	return weekdays, true
}

func sopLogRecurrenceMonthDays(item map[string]any) (map[int]struct{}, bool) {
	values, ok := sopLogValuesForKeys(item, "monthDays", "month_days", "daysOfMonth", "days_of_month", "monthDay", "month_day", "dayOfMonth", "day_of_month")
	if !ok {
		return nil, false
	}
	days := map[int]struct{}{}
	for _, value := range values {
		if day, parsed := parseSOPLogInt(value); parsed && day >= 1 && day <= 31 {
			days[day] = struct{}{}
		}
	}
	if len(days) == 0 {
		return nil, false
	}
	return days, true
}

func sopLogRecurrenceDates(item map[string]any, now time.Time) (map[string]struct{}, map[string]struct{}, bool) {
	values, ok := sopLogValuesForKeys(item, "dates", "dateList", "date_list", "repeatDates", "repeat_dates")
	if !ok {
		return nil, nil, false
	}
	dates := map[string]struct{}{}
	monthDates := map[string]struct{}{}
	for _, value := range values {
		text := strings.TrimSpace(fmt.Sprint(value))
		if text == "" {
			continue
		}
		if parsed, ok := parseSOPLogSchedule(text, now); ok {
			dates[parsed.Format("2006-01-02")] = struct{}{}
			continue
		}
		if parsed, err := time.ParseInLocation("01-02", text, now.Location()); err == nil {
			monthDates[parsed.Format("01-02")] = struct{}{}
		}
	}
	if len(dates) == 0 && len(monthDates) == 0 {
		return nil, nil, false
	}
	return dates, monthDates, true
}

func sopLogValuesForKeys(item map[string]any, keys ...string) ([]any, bool) {
	for _, key := range keys {
		raw, ok := item[key]
		if !ok {
			continue
		}
		values := flattenSOPLogValues(raw)
		if len(values) > 0 {
			return values, true
		}
	}
	return nil, false
}

func flattenSOPLogValues(raw any) []any {
	switch value := raw.(type) {
	case []any:
		values := make([]any, 0, len(value))
		for _, item := range value {
			values = append(values, flattenSOPLogValues(item)...)
		}
		return values
	case string:
		parts := strings.FieldsFunc(value, func(r rune) bool {
			return r == ',' || r == '，' || r == ';' || r == '；' || r == '|' || r == '、' || r == ' '
		})
		if len(parts) <= 1 {
			return []any{value}
		}
		values := make([]any, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				values = append(values, part)
			}
		}
		return values
	default:
		return []any{value}
	}
}

func parseSOPLogWeekday(raw any) (time.Weekday, bool) {
	if number, ok := parseSOPLogInt(raw); ok {
		switch {
		case number == 0 || number == 7:
			return time.Sunday, true
		case number >= 1 && number <= 6:
			return time.Weekday(number), true
		default:
			return time.Sunday, false
		}
	}
	normalized := strings.ToLower(strings.TrimSpace(fmt.Sprint(raw)))
	normalized = strings.ReplaceAll(normalized, "星期", "周")
	normalized = strings.ReplaceAll(normalized, "礼拜", "周")
	normalized = strings.TrimPrefix(normalized, "周")
	switch normalized {
	case "sun", "sunday", "日", "天", "7", "0":
		return time.Sunday, true
	case "mon", "monday", "一", "1":
		return time.Monday, true
	case "tue", "tues", "tuesday", "二", "2":
		return time.Tuesday, true
	case "wed", "wednesday", "三", "3":
		return time.Wednesday, true
	case "thu", "thur", "thurs", "thursday", "四", "4":
		return time.Thursday, true
	case "fri", "friday", "五", "5":
		return time.Friday, true
	case "sat", "saturday", "六", "6":
		return time.Saturday, true
	default:
		return time.Sunday, false
	}
}

func parseSOPLogInt(raw any) (int, bool) {
	switch value := raw.(type) {
	case int:
		return value, true
	case int64:
		return int(value), true
	case float64:
		return int(value), value == float64(int(value))
	case json.Number:
		intValue, err := value.Int64()
		if err != nil {
			return 0, false
		}
		return int(intValue), true
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func (r sopLogRecurrence) matches(now time.Time) bool {
	if r.HasWeekdays {
		if _, ok := r.Weekdays[now.Weekday()]; !ok {
			return false
		}
	}
	if r.HasMonthDays {
		if _, ok := r.MonthDays[now.Day()]; !ok {
			return false
		}
	}
	if r.HasDates {
		dateKey := now.Format("2006-01-02")
		monthDateKey := now.Format("01-02")
		_, dateOK := r.Dates[dateKey]
		_, monthDateOK := r.MonthDates[monthDateKey]
		if !dateOK && !monthDateOK {
			return false
		}
	}
	return true
}

func (r sopLogRecurrence) actionable() bool {
	switch r.Period {
	case "weekly":
		return r.HasWeekdays || r.HasDates
	case "monthly":
		return r.HasMonthDays || r.HasDates
	default:
		return true
	}
}

func (r sopLogRecurrence) occurrenceKey(scheduled time.Time) string {
	period := r.Period
	if period == "" {
		switch {
		case r.HasDates:
			period = "date"
		case r.HasMonthDays:
			period = "monthly"
		case r.HasWeekdays:
			period = "weekly"
		default:
			period = "recurring"
		}
	}
	return period + ":" + scheduled.Format("2006-01-02")
}

func parseSOPLogRelativeSchedule(item map[string]any, base time.Time) (time.Time, bool) {
	anchor, hasAnchor := parseSOPLogRelativeAnchor(item, base)
	if !hasAnchor {
		return time.Time{}, false
	}
	delay, hasDelay := parseSOPLogRelativeDelay(item)
	if !hasDelay {
		return time.Time{}, false
	}
	return anchor.Add(delay), true
}

func parseSOPLogRelativeAnchor(item map[string]any, base time.Time) (time.Time, bool) {
	for _, key := range []string{"baseTime", "base_time", "startTime", "start_time", "anchorTime", "anchor_time", "createdAt", "created_at"} {
		if raw, ok := item[key]; ok {
			if scheduled, parsed := parseSOPLogSchedule(raw, base); parsed {
				return scheduled, true
			}
		}
	}
	return time.Time{}, false
}

func parseSOPLogRelativeDelay(item map[string]any) (time.Duration, bool) {
	var total time.Duration
	found := false
	for _, key := range []string{"delay", "delayTime", "delay_time", "relativeTime", "relative_time", "after", "afterTime", "after_time"} {
		if raw, ok := item[key]; ok {
			if value, parsed := parseSOPLogDuration(raw, 0); parsed {
				total += value
				found = true
			}
		}
	}
	for _, candidate := range []struct {
		keys []string
		unit time.Duration
	}{
		{keys: []string{"delaySeconds", "delay_seconds", "afterSeconds", "after_seconds", "seconds", "second", "sec"}, unit: time.Second},
		{keys: []string{"delayMinutes", "delay_minutes", "afterMinutes", "after_minutes", "minutes", "minute"}, unit: time.Minute},
		{keys: []string{"delayHours", "delay_hours", "afterHours", "after_hours", "hours", "hour"}, unit: time.Hour},
		{keys: []string{"delayDays", "delay_days", "afterDays", "after_days", "days", "day"}, unit: 24 * time.Hour},
	} {
		for _, key := range candidate.keys {
			if raw, ok := item[key]; ok {
				if value, parsed := parseSOPLogDuration(raw, candidate.unit); parsed {
					total += value
					found = true
				}
			}
		}
	}
	return total, found
}

func hasSOPLogRelativeDelayKey(item map[string]any) bool {
	for _, key := range []string{
		"delay", "delayTime", "delay_time", "relativeTime", "relative_time", "after", "afterTime", "after_time",
		"delaySeconds", "delay_seconds", "afterSeconds", "after_seconds", "seconds", "second", "sec",
		"delayMinutes", "delay_minutes", "afterMinutes", "after_minutes", "minutes", "minute",
		"delayHours", "delay_hours", "afterHours", "after_hours", "hours", "hour",
		"delayDays", "delay_days", "afterDays", "after_days", "days", "day",
	} {
		if _, ok := item[key]; ok {
			return true
		}
	}
	return false
}

func parseSOPLogTargetAnchor(item map[string]any) string {
	values, ok := sopLogValuesForKeys(item,
		"targetAnchor", "target_anchor", "anchor", "anchorType", "anchor_type",
		"event", "eventType", "event_type", "trigger", "triggerType", "trigger_type",
		"base", "baseType", "base_type",
	)
	if !ok {
		return ""
	}
	for _, raw := range values {
		normalized := strings.ToLower(strings.TrimSpace(fmt.Sprint(raw)))
		normalized = strings.ReplaceAll(normalized, "_", "")
		normalized = strings.ReplaceAll(normalized, "-", "")
		normalized = strings.ReplaceAll(normalized, " ", "")
		normalized = strings.ReplaceAll(normalized, "客户", "")
		normalized = strings.ReplaceAll(normalized, "用户", "")
		normalized = strings.ReplaceAll(normalized, "成员", "")
		normalized = strings.ReplaceAll(normalized, "时间", "")
		switch normalized {
		case "roomjoin", "joinroom", "contactjoinroom", "customerjoinroom", "memberjoinroom", "chatjoin", "joinchat", "join", "joined", "入群", "进群", "加入群", "加入客户群", "群入群":
			return SOPLogTargetAnchorRoomJoin
		}
	}
	return ""
}

func parseSOPLogDuration(raw any, defaultUnit time.Duration) (time.Duration, bool) {
	switch value := raw.(type) {
	case string:
		return parseSOPLogDurationString(value, defaultUnit)
	case float64:
		return sopLogDurationFromFloat(value, defaultUnit)
	case int:
		return sopLogDurationFromFloat(float64(value), defaultUnit)
	case int64:
		return sopLogDurationFromFloat(float64(value), defaultUnit)
	case json.Number:
		floatValue, err := value.Float64()
		if err != nil {
			return 0, false
		}
		return sopLogDurationFromFloat(floatValue, defaultUnit)
	default:
		return 0, false
	}
}

func parseSOPLogDurationString(raw string, defaultUnit time.Duration) (time.Duration, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, false
	}
	if duration, err := time.ParseDuration(value); err == nil && duration >= 0 {
		return duration, true
	}
	normalized := strings.ToLower(strings.ReplaceAll(value, " ", ""))
	for _, candidate := range []struct {
		suffix string
		unit   time.Duration
	}{
		{suffix: "milliseconds", unit: time.Millisecond},
		{suffix: "millisecond", unit: time.Millisecond},
		{suffix: "ms", unit: time.Millisecond},
		{suffix: "seconds", unit: time.Second},
		{suffix: "second", unit: time.Second},
		{suffix: "secs", unit: time.Second},
		{suffix: "sec", unit: time.Second},
		{suffix: "秒", unit: time.Second},
		{suffix: "s", unit: time.Second},
		{suffix: "minutes", unit: time.Minute},
		{suffix: "minute", unit: time.Minute},
		{suffix: "mins", unit: time.Minute},
		{suffix: "min", unit: time.Minute},
		{suffix: "分钟", unit: time.Minute},
		{suffix: "分", unit: time.Minute},
		{suffix: "m", unit: time.Minute},
		{suffix: "hours", unit: time.Hour},
		{suffix: "hour", unit: time.Hour},
		{suffix: "hrs", unit: time.Hour},
		{suffix: "hr", unit: time.Hour},
		{suffix: "小时", unit: time.Hour},
		{suffix: "时", unit: time.Hour},
		{suffix: "h", unit: time.Hour},
		{suffix: "days", unit: 24 * time.Hour},
		{suffix: "day", unit: 24 * time.Hour},
		{suffix: "天", unit: 24 * time.Hour},
		{suffix: "d", unit: 24 * time.Hour},
	} {
		if !strings.HasSuffix(normalized, candidate.suffix) {
			continue
		}
		number := strings.TrimSpace(strings.TrimSuffix(normalized, candidate.suffix))
		if duration, parsed := sopLogDurationNumber(number, candidate.unit); parsed {
			return duration, true
		}
	}
	return sopLogDurationNumber(value, defaultUnit)
}

func sopLogDurationNumber(raw string, unit time.Duration) (time.Duration, bool) {
	if unit <= 0 {
		return 0, false
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0, false
	}
	return sopLogDurationFromFloat(value, unit)
}

func sopLogDurationFromFloat(value float64, unit time.Duration) (time.Duration, bool) {
	if value < 0 || unit <= 0 {
		return 0, false
	}
	return time.Duration(value * float64(unit)), true
}

func sopLogIDs(raw string) ([]int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" || raw == "{}" {
		return []int{}, nil
	}
	if ids, ok := sopCSVInts(raw); ok {
		return uniquePositiveInts(ids), nil
	}
	var decoded any
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil {
		return nil, err
	}
	ids := make([]int, 0)
	collectSOPLogIDs(decoded, &ids)
	return uniquePositiveInts(ids), nil
}

func collectSOPLogIDs(value any, ids *[]int) {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			collectSOPLogIDs(item, ids)
		}
	case map[string]any:
		for _, key := range []string{"id", "employeeId", "employee_id", "contactId", "contact_id", "roomId", "room_id"} {
			if value, ok := typed[key]; ok {
				collectSOPLogIDs(value, ids)
				return
			}
		}
	case json.Number:
		if intValue, err := typed.Int64(); err == nil && intValue > 0 {
			*ids = append(*ids, int(intValue))
		}
	case float64:
		if typed > 0 {
			*ids = append(*ids, int(typed))
		}
	case string:
		for _, part := range strings.Split(typed, ",") {
			value, err := strconv.Atoi(strings.TrimSpace(part))
			if err == nil && value > 0 {
				*ids = append(*ids, value)
			}
		}
	}
}

func uniquePositiveInts(values []int) []int {
	seen := map[int]struct{}{}
	result := make([]int, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func sopLogContainsID(values []int, target int) bool {
	if target <= 0 {
		return false
	}
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func mergeSOPLogCronResult(target *SOPLogCronResult, source SOPLogCronResult) {
	target.ContactSourcesScanned += source.ContactSourcesScanned
	target.RoomSourcesScanned += source.RoomSourcesScanned
	target.ContactLogsInserted += source.ContactLogsInserted
	target.RoomLogsInserted += source.RoomLogsInserted
	target.ItemsSkipped += source.ItemsSkipped
	target.ItemsFailed += source.ItemsFailed
}
