package dashboard

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeWorkMessageExportDownloadStore struct {
	*fakeAutoTagStore
	artifact WorkMessageExportArtifact
	events   *[]string
}

func (s *fakeWorkMessageExportDownloadStore) WorkMessageExportCandidates(context.Context, WorkMessageExportCandidateFilter) (WorkMessageExportCandidatesPage, error) {
	return WorkMessageExportCandidatesPage{}, nil
}
func (s *fakeWorkMessageExportDownloadStore) WorkMessageExportTasks(context.Context, WorkMessageExportTaskQuery) (WorkMessageExportTaskPage, error) {
	return WorkMessageExportTaskPage{}, nil
}
func (s *fakeWorkMessageExportDownloadStore) CreateWorkMessageExportTask(context.Context, WorkMessageExportTaskInput) (WorkMessageExportCreateResult, error) {
	return WorkMessageExportCreateResult{}, nil
}
func (s *fakeWorkMessageExportDownloadStore) WorkMessageExportArtifact(context.Context, int, int, int, int64) (WorkMessageExportArtifact, error) {
	*s.events = append(*s.events, "artifact")
	return s.artifact, nil
}

type orderedDeadlineRecorder struct {
	*httptest.ResponseRecorder
	events *[]string
}

func (w *orderedDeadlineRecorder) SetWriteDeadline(deadline time.Time) error {
	if deadline.IsZero() {
		*w.events = append(*w.events, "deadline")
	}
	return nil
}

func (w *orderedDeadlineRecorder) Write(body []byte) (int, error) {
	*w.events = append(*w.events, "body")
	return w.ResponseRecorder.Write(body)
}

func (w *orderedDeadlineRecorder) WriteString(body string) (int, error) {
	*w.events = append(*w.events, "body")
	return w.ResponseRecorder.WriteString(body)
}

func (w *orderedDeadlineRecorder) ReadFrom(reader io.Reader) (int64, error) {
	*w.events = append(*w.events, "body")
	body, err := io.ReadAll(reader)
	if err != nil {
		return 0, err
	}
	written, err := w.ResponseRecorder.Write(body)
	return int64(written), err
}

func TestWorkMessageExportDownloadClearsDeadlineAfterArtifactBeforeBody(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "export.zip")
	if err := os.WriteFile(path, []byte("zip-body"), 0o600); err != nil {
		t.Fatal(err)
	}
	events := []string{}
	store := &fakeWorkMessageExportDownloadStore{
		fakeAutoTagStore: &fakeAutoTagStore{user: User{ID: 1, TenantID: 10, IsSuperAdmin: 1}},
		artifact:         WorkMessageExportArtifact{Path: path, Filename: "export.zip", ContentType: "application/zip"},
		events:           &events,
	}
	handler := NewAutoTagHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, nil)
	req := withAutoTagTestPrincipal(httptest.NewRequest(http.MethodGet, "/dashboard/workMessage/exportDownload?taskId=9", nil), 1, 10, 7)
	recorder := &orderedDeadlineRecorder{ResponseRecorder: httptest.NewRecorder(), events: &events}
	handler.WorkMessageExportDownload(recorder, req)
	if strings.Join(events, ",") != "artifact,deadline,body" || recorder.Code != http.StatusOK {
		t.Fatalf("events=%v status=%d body=%q", events, recorder.Code, recorder.Body.String())
	}

	unauthorizedEvents := []string{}
	unauthorized := &orderedDeadlineRecorder{ResponseRecorder: httptest.NewRecorder(), events: &unauthorizedEvents}
	handler.WorkMessageExportDownload(unauthorized, httptest.NewRequest(http.MethodGet, "/dashboard/workMessage/exportDownload?taskId=9", nil))
	if strings.Contains(strings.Join(unauthorizedEvents, ","), "deadline") {
		t.Fatalf("unauthorized response cleared deadline: %v", unauthorizedEvents)
	}
}

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
