package aisettings

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"jiyi/mochat-go/internal/modules/ai-settings/ports"
)

func TestParseDocumentSupportsUTF8TextAndMarkdown(t *testing.T) {
	for _, name := range []string{"guide.txt", "guide.md"} {
		t.Run(filepath.Ext(name), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), name)
			content := "退款需要主管审批\n产品保修期为一年"
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			parsed, err := ParseDocument(path, name)
			if err != nil {
				t.Fatal(err)
			}
			if parsed.Text != content || parsed.CharacterCount != utf8.RuneCountInString(content) || len(parsed.SHA256) != 64 {
				t.Fatalf("parsed = %#v", parsed)
			}
		})
	}
}

func TestParseDocumentSupportsDOCX(t *testing.T) {
	path := filepath.Join(t.TempDir(), "guide.docx")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	entry, err := archive.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = entry.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>退款需要主管审批</w:t></w:r></w:p><w:p><w:r><w:t>保修期一年</w:t></w:r></w:p></w:body></w:document>`))
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseDocument(path, "guide.docx")
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Text != "退款需要主管审批\n保修期一年" {
		t.Fatalf("text = %q", parsed.Text)
	}
}

func TestParseDocumentSupportsPDFTextLayer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "guide.pdf")
	if err := os.WriteFile(path, minimalPDF("Refund approval required"), 0o600); err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseDocument(path, "guide.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(parsed.Text, "Refund approval required") {
		t.Fatalf("text = %q", parsed.Text)
	}
}

func TestParseDocumentRejectsUnsupportedInvalidAndOversizedContent(t *testing.T) {
	t.Run("legacy doc", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "legacy.doc")
		_ = os.WriteFile(path, []byte("legacy"), 0o600)
		if _, err := ParseDocument(path, "legacy.doc"); err != ports.ErrUnsupportedDocumentType {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("invalid utf8", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "invalid.txt")
		_ = os.WriteFile(path, []byte{0xff, 0xfe}, 0o600)
		if _, err := ParseDocument(path, "invalid.txt"); err != ports.ErrDocumentUnreadable {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("empty pdf text", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "scan.pdf")
		_ = os.WriteFile(path, minimalPDF(""), 0o600)
		if _, err := ParseDocument(path, "scan.pdf"); err != ports.ErrDocumentUnreadable {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("text limit", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "large.txt")
		_ = os.WriteFile(path, []byte(strings.Repeat("知", MaxDocumentRunes+1)), 0o600)
		if _, err := ParseDocument(path, "large.txt"); err != ports.ErrDocumentTextTooLarge {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestChunkTextUsesBoundedOverlap(t *testing.T) {
	chunks := ChunkText(strings.Repeat("知识库内容。", 400))
	if len(chunks) < 2 {
		t.Fatalf("chunks = %d", len(chunks))
	}
	for index, chunk := range chunks {
		if chunk.Ordinal != index || chunk.CharacterCount == 0 || chunk.CharacterCount > ChunkRunes {
			t.Fatalf("chunk[%d] = %#v", index, chunk)
		}
	}
	first := []rune(chunks[0].Content)
	second := []rune(chunks[1].Content)
	if string(first[len(first)-ChunkOverlapRunes:]) != string(second[:ChunkOverlapRunes]) {
		t.Fatal("chunks do not preserve configured overlap")
	}
}

func minimalPDF(text string) []byte {
	objects := []string{
		"<</Type/Catalog/Pages 2 0 R>>",
		"<</Type/Pages/Kids[3 0 R]/Count 1>>",
		"<</Type/Page/Parent 2 0 R/MediaBox[0 0 612 792]/Contents 4 0 R/Resources<</Font<</F1 5 0 R>>>>>>",
		fmt.Sprintf("<</Length %d>>\nstream\nBT /F1 18 Tf 72 720 Td (%s) Tj ET\nendstream", len(text)+36, text),
		"<</Type/Font/Subtype/Type1/BaseFont/Helvetica>>",
	}
	var buffer bytes.Buffer
	buffer.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects)+1)
	for index, object := range objects {
		offsets[index+1] = buffer.Len()
		fmt.Fprintf(&buffer, "%d 0 obj\n%s\nendobj\n", index+1, object)
	}
	xref := buffer.Len()
	fmt.Fprintf(&buffer, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for index := 1; index < len(offsets); index++ {
		fmt.Fprintf(&buffer, "%010d 00000 n \n", offsets[index])
	}
	fmt.Fprintf(&buffer, "trailer\n<</Size %d/Root 1 0 R>>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return buffer.Bytes()
}
