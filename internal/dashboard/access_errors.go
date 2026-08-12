package dashboard

import "net/http"

func writeAccessError(w http.ResponseWriter, err error) {
	switch err {
	case ErrUnauthorized:
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
	case ErrPermissionDenied:
		writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, "permission denied", nil)
	default:
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
	}
}
