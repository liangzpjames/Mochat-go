package dashboard

import (
	"strings"
	"testing"
	"time"
)

func TestValidateWorkMessageExportRequestRejectsUnsafeRangesAndCounts(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		request WorkMessageExportCreateRequest
		want    string
	}{
		{
			name: "too many objects",
			request: WorkMessageExportCreateRequest{
				ExportType: "customer", ObjectIDs: make([]int, workMessageExportMaxObjects+1),
				StartAt: now.Add(-time.Hour), EndAt: now,
			},
			want: "最多选择 100 个对象",
		},
		{
			name: "range longer than two years",
			request: WorkMessageExportCreateRequest{
				ExportType: "customer", ObjectIDs: []int{7},
				StartAt: now.AddDate(-2, 0, -1), EndAt: now,
			},
			want: "日期跨度不能超过 2 年",
		},
		{
			name: "unsupported internal scope",
			request: WorkMessageExportCreateRequest{
				ExportType: "customer", ObjectIDs: []int{7},
				ConversationScopes: []string{"internal"}, StartAt: now.Add(-time.Hour), EndAt: now,
			},
			want: "当前仅支持客户单聊和外部群聊",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateWorkMessageExportRequest(test.request, now)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error=%v, want text %q", err, test.want)
			}
		})
	}
}

func TestNormalizeWorkMessageExportRequestAtClampsCurrentShanghaiDay(t *testing.T) {
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, shanghai)
	request := WorkMessageExportCreateRequest{
		ExportType: "customer", ObjectIDs: []int{7},
		StartAt: time.Date(2026, 8, 15, 0, 0, 0, 0, shanghai),
		EndAt:   time.Date(2026, 8, 21, 23, 59, 59, 0, shanghai),
	}

	normalized := normalizeWorkMessageExportRequestAt(request, now)
	if !normalized.EndAt.Equal(now) {
		t.Fatalf("endAt=%s, want current time %s", normalized.EndAt, now)
	}
	if err := validateWorkMessageExportRequest(normalized, now); err != nil {
		t.Fatalf("current-day range rejected after normalization: %v", err)
	}
}

func TestNormalizeWorkMessageExportRequestAtKeepsFutureDayRejected(t *testing.T) {
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, shanghai)
	request := WorkMessageExportCreateRequest{
		ExportType: "customer", ObjectIDs: []int{7},
		StartAt: time.Date(2026, 8, 15, 0, 0, 0, 0, shanghai),
		EndAt:   time.Date(2026, 8, 22, 23, 59, 59, 0, shanghai),
	}

	normalized := normalizeWorkMessageExportRequestAt(request, now)
	if err := validateWorkMessageExportRequest(normalized, now); err == nil || !strings.Contains(err.Error(), "结束时间不能晚于当前时间") {
		t.Fatalf("future-day range error=%v", err)
	}
}

func TestWorkMessageExportPageSizeIsFixed(t *testing.T) {
	if err := validateWorkMessageExportPageSize(20); err != nil {
		t.Fatalf("fixed page size rejected: %v", err)
	}
	for _, pageSize := range []int{0, 10, 50, 100} {
		if err := validateWorkMessageExportPageSize(pageSize); err == nil {
			t.Fatalf("page size %d accepted", pageSize)
		}
	}
}

func TestWorkMessageExportCSVCellGuardsFormulaPrefixes(t *testing.T) {
	for _, raw := range []string{"=SUM(A1)", "+1", "-1", "@mention"} {
		if got := safeWorkMessageExportCSVCell(raw); !strings.HasPrefix(got, "'") {
			t.Fatalf("cell %q was not guarded: %q", raw, got)
		}
	}
	if got := safeWorkMessageExportCSVCell("ordinary text"); got != "ordinary text" {
		t.Fatalf("ordinary cell changed: %q", got)
	}
}
