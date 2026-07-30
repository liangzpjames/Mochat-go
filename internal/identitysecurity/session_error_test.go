package identitysecurity

import (
	"errors"
	"testing"
)

func TestPersistentSessionErrorsAreInvalidCredentials(t *testing.T) {
	for _, err := range []error{ErrSessionNotFound, ErrSessionRevoked, ErrSessionExpired} {
		var invalid interface {
			InvalidSession() bool
		}
		if !errors.As(err, &invalid) || !invalid.InvalidSession() {
			t.Fatalf("%v is not classified as an invalid session", err)
		}
	}
}
