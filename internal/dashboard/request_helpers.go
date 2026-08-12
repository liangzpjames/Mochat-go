package dashboard

import (
	"net/http"
	"strconv"
)

func positiveQueryInt(r *http.Request, key string, fallback int) int {
	value, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func positiveQueryIntRequired(r *http.Request, key string) (int, error) {
	value, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil || value <= 0 {
		if err == nil {
			err = strconv.ErrSyntax
		}
		return 0, err
	}
	return value, nil
}

func containsInt(values []int, needle int) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

type fieldError string

func (e fieldError) Error() string { return string(e) }
