package aiinsight

import (
	"bytes"
	"context"
	"encoding/csv"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type task5ProjectionExportRepo struct {
	workspaceTestRepo
	items  []ConversationInsight
	filter InsightFilter
}

func (r *task5ProjectionExportRepo) InsightPage(_ context.Context, filter InsightFilter) (InsightPage, error) {
	r.filter = filter
	return InsightPage{Page: 1, PageSize: filter.PageSize, Total: len(r.items), Items: r.items}, nil
}

func TestProjectionExportPageNormalizationKeepsBoundedExportLimit(t *testing.T) {
	page, size := normalizeInsightPage(InsightFilter{Page: 9, PageSize: 10000, Export: true})
	if page != 1 || size != 10000 {
		t.Fatalf("export page=%d size=%d, want page=1 size=10000", page, size)
	}

	page, size = normalizeInsightPage(InsightFilter{Page: 2, PageSize: 10000})
	if page != 2 || size != 20 {
		t.Fatalf("ordinary page=%d size=%d, want existing cap fallback page=2 size=20", page, size)
	}
}

func TestProjectionExportWritesMoreThanTwentyTruthfulFormulaSafeRows(t *testing.T) {
	items := make([]ConversationInsight, 25)
	for index := range items {
		result := `{"customer":{"emotion":{"label":"negative","reason":"=真实依据"},"keywords":["+采购"]},"employeeQa":{"score":0,"strengths":["-优点"],"issues":["@问题"],"suggestions":["=建议"]}}`
		if index == 1 {
			result = `{"customer":{"emotion":{"label":"unknown","reason":""},"keywords":[]},"employeeQa":{"strengths":[],"issues":[],"suggestions":[]}}`
		}
		items[index] = ConversationInsight{
			EmployeeName: "=员工", TargetName: "+对象", Summary: "-评分摘要", Status: AnalysisStatusSucceeded,
			ErrorSummary: "@失败", SourceMessageCount: index + 1, Provider: "=provider", Model: "+model", PromptVersion: "-prompt",
			ResultJSON: []byte(result),
		}
	}

	for _, test := range []struct {
		view       string
		field      string
		want       string
		formulaCol string
		formula    string
	}{
		{view: "emotion", field: "客户情绪", want: "negative", formulaCol: "情绪依据", formula: "'=真实依据"},
		{view: "employee-score", field: "员工评分", want: "0", formulaCol: "优点", formula: "'-优点"},
		{view: "communication-keyword", field: "沟通关键词", want: "'+采购", formulaCol: "摘要", formula: "'-评分摘要"},
	} {
		t.Run(test.view, func(t *testing.T) {
			repo := &task5ProjectionExportRepo{items: items}
			handler := NewWorkspaceHandler(workspaceTestResolver{principal: WorkspacePrincipal{UserID: 7, TenantID: 11, CorpID: 22}}, nil, repo, nil)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/dashboard/ai-insight/"+test.view+"/export", nil))
			if recorder.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
			if !repo.filter.Export || repo.filter.PageSize != 10000 || repo.filter.View != test.view {
				t.Fatalf("export filter=%#v", repo.filter)
			}
			body := bytes.TrimPrefix(recorder.Body.Bytes(), []byte{0xEF, 0xBB, 0xBF})
			records, err := csv.NewReader(bytes.NewReader(body)).ReadAll()
			if err != nil {
				t.Fatal(err)
			}
			if len(records) != 26 {
				t.Fatalf("CSV records=%d, want header + 25 data rows", len(records))
			}
			header := make(map[string]int, len(records[0]))
			for index, name := range records[0] {
				header[name] = index
			}
			row := records[1]
			if row[header[test.field]] != test.want || row[header[test.formulaCol]] != test.formula {
				t.Fatalf("view=%s row=%#v", test.view, row)
			}
			for _, name := range []string{"员工", "对象", "失败原因", "Provider", "Model", "Prompt"} {
				if !strings.HasPrefix(row[header[name]], "'") {
					t.Errorf("column %s was not formula-safe: %q", name, row[header[name]])
				}
			}
			if test.view == "employee-score" && records[2][header["员工评分"]] != "" {
				t.Fatalf("missing score exported as %q, want empty", records[2][header["员工评分"]])
			}
		})
	}
}
