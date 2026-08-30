package document

import (
	"io"
	"unicode/utf8"

	pdf "github.com/ledongthuc/pdf"

	"jiyi/mochat-go/internal/modules/ai-settings/ports"
)

func ParsePDF(path string, maxBytes int64) (string, error) {
	file, reader, err := pdf.Open(path)
	if err != nil {
		return "", ports.ErrDocumentUnreadable
	}
	defer file.Close()
	plain, err := reader.GetPlainText()
	if err != nil {
		return "", ports.ErrDocumentUnreadable
	}
	content, err := io.ReadAll(io.LimitReader(plain, maxBytes+1))
	if err != nil || !utf8.Valid(content) {
		return "", ports.ErrDocumentUnreadable
	}
	if int64(len(content)) > maxBytes {
		return "", ports.ErrDocumentTextTooLarge
	}
	return string(content), nil
}
