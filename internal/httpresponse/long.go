package httpresponse

import (
	"net/http"
	"time"
)

// AllowLongWrite removes the server's global write deadline for an already
// authenticated, explicitly long-lived response. Callers must keep this scoped
// to known download/stream routes so normal slow-client protection remains.
func AllowLongWrite(w http.ResponseWriter) {
	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
}
