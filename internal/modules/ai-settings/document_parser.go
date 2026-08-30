package aisettings

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	documentadapter "jiyi/mochat-go/internal/modules/ai-settings/adapters/document"
	"jiyi/mochat-go/internal/modules/ai-settings/ports"
)

const (
	MaxUploadBytes    int64 = 20 << 20
	MaxDocumentRunes        = 500_000
	ChunkRunes              = 1_200
	ChunkOverlapRunes       = 100
	maxDOCXXMLBytes   int64 = 8 << 20
)

type ParsedDocument = ports.ParsedDocument

func ParseDocument(path, filename string) (ParsedDocument, error) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return ParsedDocument{}, ports.ErrDocumentUnreadable
	}
	if info.Size() > MaxUploadBytes {
		return ParsedDocument{}, ports.ErrDocumentTooLarge
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return ParsedDocument{}, ports.ErrDocumentUnreadable
	}
	extension := strings.ToLower(filepath.Ext(filepath.Base(filename)))
	var text, mimeType string
	switch extension {
	case ".txt":
		text, mimeType, err = parseUTF8(raw), "text/plain; charset=utf-8", nil
	case ".md":
		text, mimeType, err = parseUTF8(raw), "text/markdown; charset=utf-8", nil
	case ".docx":
		text, err = parseDOCX(path)
		mimeType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".pdf":
		text, err = parsePDF(path)
		mimeType = "application/pdf"
	default:
		return ParsedDocument{}, ports.ErrUnsupportedDocumentType
	}
	if err != nil {
		return ParsedDocument{}, err
	}
	text = normalizeExtractedText(text)
	if text == "" {
		return ParsedDocument{}, ports.ErrDocumentUnreadable
	}
	characterCount := utf8.RuneCountInString(text)
	if characterCount > MaxDocumentRunes {
		return ParsedDocument{}, ports.ErrDocumentTextTooLarge
	}
	digest := sha256.Sum256(raw)
	return ParsedDocument{
		Text:           text,
		SHA256:         hex.EncodeToString(digest[:]),
		CharacterCount: characterCount,
		Extension:      strings.TrimPrefix(extension, "."),
		MIMEType:       mimeType,
	}, nil
}

func parseUTF8(raw []byte) string {
	raw = bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf})
	if !utf8.Valid(raw) {
		return ""
	}
	return string(raw)
}

func parseDOCX(path string) (string, error) {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return "", ports.ErrDocumentUnreadable
	}
	defer archive.Close()
	for _, file := range archive.File {
		if file.Name != "word/document.xml" {
			continue
		}
		if file.UncompressedSize64 > uint64(maxDOCXXMLBytes) {
			return "", ports.ErrDocumentTextTooLarge
		}
		reader, err := file.Open()
		if err != nil {
			return "", ports.ErrDocumentUnreadable
		}
		text, parseErr := extractDOCXText(io.LimitReader(reader, maxDOCXXMLBytes+1))
		closeErr := reader.Close()
		if parseErr != nil || closeErr != nil {
			return "", ports.ErrDocumentUnreadable
		}
		return text, nil
	}
	return "", ports.ErrDocumentUnreadable
}

func extractDOCXText(reader io.Reader) (string, error) {
	decoder := xml.NewDecoder(reader)
	var builder strings.Builder
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
		switch value := token.(type) {
		case xml.StartElement:
			switch value.Name.Local {
			case "t":
				var content string
				if err := decoder.DecodeElement(&content, &value); err != nil {
					return "", err
				}
				builder.WriteString(content)
			case "tab":
				builder.WriteByte('\t')
			case "br", "cr":
				builder.WriteByte('\n')
			}
		case xml.EndElement:
			if value.Name.Local == "p" && builder.Len() > 0 {
				builder.WriteByte('\n')
			}
		}
	}
	return builder.String(), nil
}

func parsePDF(path string) (string, error) {
	return documentadapter.ParsePDF(path, int64(MaxDocumentRunes*4))
}

func normalizeExtractedText(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	value = strings.ReplaceAll(value, "\u00a0", " ")
	lines := strings.Split(value, "\n")
	result := make([]string, 0, len(lines))
	blank := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if len(result) > 0 && !blank {
				result = append(result, "")
				blank = true
			}
			continue
		}
		result = append(result, line)
		blank = false
	}
	return strings.TrimSpace(strings.Join(result, "\n"))
}

func ChunkText(text string) []ports.KnowledgeChunk {
	runes := []rune(text)
	if len(runes) == 0 {
		return []ports.KnowledgeChunk{}
	}
	chunks := make([]ports.KnowledgeChunk, 0, (len(runes)+ChunkRunes-1)/ChunkRunes)
	for start := 0; start < len(runes); {
		end := start + ChunkRunes
		if end > len(runes) {
			end = len(runes)
		}
		content := strings.TrimSpace(string(runes[start:end]))
		if content != "" {
			chunks = append(chunks, ports.KnowledgeChunk{Ordinal: len(chunks), Content: content, CharacterCount: utf8.RuneCountInString(content)})
		}
		if end == len(runes) {
			break
		}
		start = end - ChunkOverlapRunes
	}
	return chunks
}
