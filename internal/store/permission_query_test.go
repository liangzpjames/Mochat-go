package store

import (
	"strings"
	"testing"
)

func TestPageMenusQueryIncludesPageAndActionNodes(t *testing.T) {
	normalized := strings.Join(strings.Fields(pageMenusQuery), " ")
	if !strings.Contains(normalized, "is_page_menu IN (1, 2)") {
		t.Fatalf("pageMenusQuery excludes action nodes: %s", normalized)
	}
}
