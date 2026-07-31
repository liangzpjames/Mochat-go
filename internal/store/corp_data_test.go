package store

import (
	"reflect"
	"strings"
	"testing"
)

func TestCorpDataTrendQueryScopesCorpAndInclusiveDateRange(t *testing.T) {
	query, args := corpDataTrendQuery(7, "2026-07-01", "2026-07-31")
	normalized := strings.Join(strings.Fields(query), " ")

	if !strings.Contains(normalized, "WHERE corp_id = ?") {
		t.Fatalf("query is not corp scoped: %s", normalized)
	}
	if !strings.Contains(normalized, "DATE(date) BETWEEN ? AND ?") {
		t.Fatalf("query is not date scoped: %s", normalized)
	}
	if !strings.Contains(normalized, "ORDER BY date ASC LIMIT 31") {
		t.Fatalf("query does not preserve deterministic ordering/window: %s", normalized)
	}
	if !reflect.DeepEqual(args, []any{7, "2026-07-01", "2026-07-31"}) {
		t.Fatalf("args = %#v", args)
	}
}
