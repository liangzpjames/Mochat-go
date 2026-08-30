package httpresponse

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type deadlineWriter struct {
	*httptest.ResponseRecorder
	deadlines []time.Time
}

func (w *deadlineWriter) SetWriteDeadline(deadline time.Time) error {
	w.deadlines = append(w.deadlines, deadline)
	return nil
}

func TestAllowLongWriteClearsServerWriteDeadlineWithoutWaitingThirtySeconds(t *testing.T) {
	writer := &deadlineWriter{ResponseRecorder: httptest.NewRecorder()}
	AllowLongWrite(writer)
	if len(writer.deadlines) != 1 || !writer.deadlines[0].IsZero() {
		t.Fatalf("write deadlines = %+v", writer.deadlines)
	}
	writer.WriteHeader(http.StatusOK)
}
