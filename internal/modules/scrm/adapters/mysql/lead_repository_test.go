package mysql

import (
	"encoding/base64"
	"errors"
	"testing"

	"jiyi/mochat-go/internal/modules/scrm/ports"
)

func TestDecodeLeadCursorIdentifiesMalformedClientCursors(t *testing.T) {
	for _, cursor := range []string{
		"%%%",
		base64.RawURLEncoding.EncodeToString([]byte("{")),
		base64.RawURLEncoding.EncodeToString([]byte(`{"created_at":"0001-01-01T00:00:00Z","id":""}`)),
	} {
		if _, err := decodeLeadCursor(cursor); !errors.Is(err, ports.ErrInvalidCursor) {
			t.Fatalf("decodeLeadCursor(%q) error = %v, want ErrInvalidCursor", cursor, err)
		}
	}
}
