package store

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/dashboard"
)

func TestWriteWorkMessageExportCSVUsesBOMAndGuardsSpreadsheetFormula(t *testing.T) {
	var buffer bytes.Buffer
	err := writeWorkMessageExportCSV(&buffer, []workMessageExportRow{{
		EmployeeID: 7, EmployeeName: "员工", SenderName: "=SUM(A1)", TargetName: "客户", ToUserType: 1, ToUserID: 31,
		MessageType: 1, Content: "+危险内容", SentAt: "2026-08-20 10:00:00", Seq: 9,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(buffer.Bytes(), []byte{0xEF, 0xBB, 0xBF}) {
		t.Fatal("CSV is missing UTF-8 BOM")
	}
	if bytes.Contains(buffer.Bytes(), []byte(",=SUM(A1),")) || bytes.Contains(buffer.Bytes(), []byte(",+危险内容,")) {
		t.Fatalf("formula-like cells were not guarded: %q", buffer.String())
	}
}

func TestWriteWorkMessageExportZipSplitsByConversation(t *testing.T) {
	temp, err := os.CreateTemp(t.TempDir(), "export-*.zip")
	if err != nil {
		t.Fatal(err)
	}
	path := temp.Name()
	rows := []workMessageExportRow{{ToUserType: 1, ToUserID: 31, Seq: 1}, {ToUserType: 2, ToUserID: 41, Seq: 2}}
	count, err := writeWorkMessageExportZip(temp, workMessageExportWorkerTask{FileMode: "split"}, rows)
	if err != nil {
		t.Fatal(err)
	}
	if err := temp.Close(); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("file count=%d, want 2", count)
	}
	archive, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	if len(archive.File) != 2 {
		t.Fatalf("zip entries=%d, want 2", len(archive.File))
	}
	for _, entry := range archive.File {
		reader, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(reader)
		_ = reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.HasPrefix(content, []byte{0xEF, 0xBB, 0xBF}) {
			t.Fatalf("entry %s missing BOM", entry.Name)
		}
	}
}

func TestWorkMessageExportObjectWhereHonorsConversationScopes(t *testing.T) {
	where, args := workMessageExportObjectWhere(dashboard.WorkMessageExportCreateRequest{
		ExportType:         string(dashboard.WorkMessageExportTypeCustomer),
		ConversationScopes: []string{"customer_direct"},
	}, []int{31, 32})
	if !strings.Contains(where, "to_user_type=1") || strings.Contains(where, "to_user_type=2") {
		t.Fatalf("direct scope predicate=%q", where)
	}
	if len(args) != 2 {
		t.Fatalf("direct scope args=%d, want 2", len(args))
	}

	where, args = workMessageExportObjectWhere(dashboard.WorkMessageExportCreateRequest{
		ExportType:         string(dashboard.WorkMessageExportTypeCustomer),
		ConversationScopes: []string{"external_group"},
	}, []int{31, 32})
	if !strings.Contains(where, "to_user_type=2") || strings.Contains(where, "to_user_type=1") {
		t.Fatalf("group scope predicate=%q", where)
	}
	if len(args) != 2 {
		t.Fatalf("group scope args=%d, want 2", len(args))
	}
}
