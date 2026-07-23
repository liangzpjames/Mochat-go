package dashboard

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func parseRequestParams(r *http.Request) (map[string]any, error) {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return nil, err
		}
		return valuesToParams(r.Form), nil
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return map[string]any{}, nil
	}

	var params map[string]any
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	if err := decoder.Decode(&params); err == nil {
		return params, nil
	}

	values, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, err
	}
	return valuesToParams(values), nil
}

func valuesToParams(values url.Values) map[string]any {
	params := make(map[string]any, len(values))
	for key, value := range values {
		normalizedKey := strings.TrimSuffix(key, "[]")
		if len(value) == 1 {
			params[normalizedKey] = value[0]
			continue
		}
		params[normalizedKey] = value
	}
	return params
}

func stringParam(params map[string]any, key string) string {
	value, ok := params[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatInt(int64(typed), 10)
	case []string:
		if len(typed) == 0 {
			return ""
		}
		return strings.TrimSpace(typed[0])
	case []any:
		if len(typed) == 0 {
			return ""
		}
		return strings.TrimSpace(fmt.Sprint(typed[0]))
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func intParam(params map[string]any, key string) (int, bool, error) {
	value, ok := params[key]
	if !ok || value == nil {
		return 0, false, nil
	}
	switch typed := value.(type) {
	case json.Number:
		integer, err := typed.Int64()
		if err != nil {
			return 0, true, err
		}
		return int(integer), true, nil
	case float64:
		return int(typed), true, nil
	case int:
		return typed, true, nil
	case string:
		if strings.TrimSpace(typed) == "" {
			return 0, false, nil
		}
		integer, err := strconv.Atoi(strings.TrimSpace(typed))
		return integer, true, err
	case []string:
		if len(typed) == 0 || strings.TrimSpace(typed[0]) == "" {
			return 0, false, nil
		}
		integer, err := strconv.Atoi(strings.TrimSpace(typed[0]))
		return integer, true, err
	case []any:
		if len(typed) == 0 {
			return 0, false, nil
		}
		return intParam(map[string]any{key: typed[0]}, key)
	default:
		integer, err := strconv.Atoi(strings.TrimSpace(fmt.Sprint(value)))
		return integer, true, err
	}
}

func intSliceParam(params map[string]any, key string) ([]int, error) {
	value, ok := params[key]
	if !ok || value == nil {
		return []int{}, nil
	}

	var values []any
	switch typed := value.(type) {
	case []any:
		values = typed
	case []string:
		values = make([]any, 0, len(typed))
		for _, raw := range typed {
			values = append(values, raw)
		}
	case string:
		if strings.TrimSpace(typed) == "" {
			return []int{}, nil
		}
		for _, raw := range strings.Split(typed, ",") {
			values = append(values, strings.TrimSpace(raw))
		}
	default:
		values = []any{typed}
	}

	seen := map[int]struct{}{}
	result := make([]int, 0, len(values))
	for _, raw := range values {
		integer, ok, err := intParam(map[string]any{key: raw}, key)
		if err != nil {
			return nil, err
		}
		if !ok || integer <= 0 {
			continue
		}
		if _, exists := seen[integer]; exists {
			continue
		}
		seen[integer] = struct{}{}
		result = append(result, integer)
	}
	return result, nil
}

func stringSliceParam(params map[string]any, key string) []string {
	value, ok := params[key]
	if !ok || value == nil {
		return []string{}
	}

	var values []any
	switch typed := value.(type) {
	case []any:
		values = typed
	case []string:
		values = make([]any, 0, len(typed))
		for _, raw := range typed {
			values = append(values, raw)
		}
	case string:
		for _, raw := range strings.Split(typed, ",") {
			values = append(values, strings.TrimSpace(raw))
		}
	default:
		values = []any{typed}
	}

	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(fmt.Sprint(raw))
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
