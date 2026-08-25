package aiinsight

import (
	"context"
	"database/sql/driver"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestDirectoryOptionsReadsAuthorityDirectoriesAndInsightCoverageWithinEmployeeScope(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	filter := DirectoryOptionFilter{
		TenantID: 7, CorpID: 8, AnalysisType: AnalysisTypeSession,
		EmployeeKeyword: `王%_\`, CustomerKeyword: `客%_\`, Limit: 25,
		Restricted: true, AllowedEmployeeIDs: []int64{1002, 1001},
	}
	ids := []driver.Value{int64(1001), int64(1002)}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM mc_work_employee e WHERE e.corp_id=? AND e.deleted_at IS NULL AND e.status=1 AND e.id IN (?,?)")).
		WithArgs(append([]driver.Value{int64(8)}, ids...)...).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	mock.ExpectQuery(`SELECT e\.id,e\.name,e\.avatar FROM mc_work_employee e WHERE e\.corp_id=\? AND e\.deleted_at IS NULL AND e\.status=1 AND e\.id IN \(\?,\?\) AND e\.name LIKE \? ESCAPE.*ORDER BY e\.name ASC,e\.id ASC LIMIT \?`).
		WithArgs(int64(8), int64(1001), int64(1002), `%王\%\_\\%`, 25).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "avatar"}).AddRow(1001, "王甲", "a1").AddRow(1002, "王乙", "a2"))
	mock.ExpectQuery(`SELECT COUNT\(DISTINCT c\.id\) FROM mc_work_contact c JOIN mc_work_contact_employee rel.*WHERE c\.corp_id=\? AND c\.deleted_at IS NULL AND rel\.employee_id IN \(\?,\?\)`).
		WithArgs(int64(8), int64(1001), int64(1002)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
	mock.ExpectQuery(`SELECT c\.id,c\.name,c\.avatar FROM mc_work_contact c JOIN mc_work_contact_employee rel.*WHERE c\.corp_id=\? AND c\.deleted_at IS NULL AND rel\.employee_id IN \(\?,\?\) AND c\.name LIKE \? ESCAPE.*GROUP BY c\.id,c\.name,c\.avatar ORDER BY c\.name ASC,c\.id ASC LIMIT \?`).
		WithArgs(int64(8), int64(1001), int64(1002), `%客\%\_\\%`, 25).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "avatar"}).AddRow(2001, "客户甲", "c1"))
	mock.ExpectQuery(`SELECT COUNT\(DISTINCT i\.employee_id\),COUNT\(DISTINCT CASE WHEN i\.target_type='1' THEN i\.target_id END\) FROM mochat_go_ai_conversation_insights i WHERE i\.tenant_id=\? AND i\.corp_id=\? AND i\.analysis_type=\? AND i\.status='succeeded' AND i\.employee_id IN \(\?,\?\)`).
		WithArgs(int64(7), int64(8), AnalysisTypeSession, int64(1001), int64(1002)).
		WillReturnRows(sqlmock.NewRows([]string{"employees", "customers"}).AddRow(1, 1))

	options, err := NewSQLRepository(db).DirectoryOptions(context.Background(), filter)
	if err != nil {
		t.Fatal(err)
	}
	if len(options.Employees) != 2 || options.Employees[0].ID != 1001 || options.Employees[1].ID != 1002 {
		t.Fatalf("employees=%#v", options.Employees)
	}
	if len(options.Customers) != 1 || options.Customers[0].ID != 2001 || options.Customers[0].Name != "客户甲" {
		t.Fatalf("customers=%#v", options.Customers)
	}
	want := InsightDirectoryCoverage{AvailableEmployeeCount: 2, AvailableCustomerCount: 3, AnalyzedEmployeeCount: 1, AnalyzedCustomerCount: 1}
	if options.Coverage != want {
		t.Fatalf("coverage=%#v want=%#v", options.Coverage, want)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDirectoryOptionsRestrictedEmptyScopeReturnsEmptyWithoutQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	options, err := NewSQLRepository(db).DirectoryOptions(context.Background(), DirectoryOptionFilter{
		TenantID: 7, CorpID: 8, AnalysisType: AnalysisTypeSession, Restricted: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(options.Employees) != 0 || len(options.Customers) != 0 || options.Coverage != (InsightDirectoryCoverage{}) {
		t.Fatalf("options=%#v", options)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestInsightWhereCustomerIDMatchesDirectConversationExactly(t *testing.T) {
	where, args := insightWhere(InsightFilter{TenantID: 7, CorpID: 8, AnalysisType: AnalysisTypeSession, CustomerID: 2001})
	actual := ""
	for _, clause := range where {
		actual += clause + " "
	}
	if !regexp.MustCompile(`i\.target_type='1'.*i\.target_id=\?`).MatchString(actual) {
		t.Fatalf("where=%s", actual)
	}
	if len(args) != 4 || args[3] != "2001" {
		t.Fatalf("args=%#v", args)
	}
}
