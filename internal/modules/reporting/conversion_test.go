package reporting

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestConversionResultUsesNullForZeroRate(t *testing.T) {
	r := ConversionResult(ConversionCounts{Lead: 0, Contact: 0})
	if Ratio(*r.Summary["contact"], *r.Summary["lead"]) != nil {
		t.Fatal("expected null")
	}
}

func TestConversionReportTreatsMissingOptionalOrderTableAsEmpty(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	for _, query := range []string{
		"SELECT COUNT(DISTINCT l.id) FROM mochat_go_scrm_leads l",
		"SELECT COUNT(DISTINCT c.id) FROM mochat_go_scrm_contacts c",
		"SELECT COUNT(DISTINCT o.id) FROM mochat_go_scrm_opportunities o",
		"SELECT COUNT(DISTINCT o.id) FROM mochat_go_scrm_opportunities o",
	} {
		mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs(int64(7), int64(8), start, end).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name=?")).
		WithArgs("mochat_go_scrm_orders").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	result, err := NewSQLRepository(db, ConversionReport).queryConversion(context.Background(), ReportQuery{
		TenantID: 7, CorpID: 8, StartAt: start, EndAt: end, Timezone: "Asia/Shanghai", Page: 1, PageSize: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Summary["order"] == nil || *result.Summary["order"] != 0 {
		t.Fatalf("order summary=%v, want zero", result.Summary["order"])
	}
	if len(result.Limitations) == 0 || result.Limitations[len(result.Limitations)-1].Code != "table_unavailable" {
		t.Fatalf("limitations=%#v", result.Limitations)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
