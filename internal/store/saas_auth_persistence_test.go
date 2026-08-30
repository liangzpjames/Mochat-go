package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"jiyi/mochat-go/internal/saasauth"
)

func TestSaaSMFAStatusTransitionActivatesEnrollment(t *testing.T) {
	tests := []struct {
		name        string
		current     int
		challenge   string
		wantNext    int
		wantAllowed bool
	}{
		{name: "enrollment activates pending credential", current: saasauth.SaaSMFAStatusPending, challenge: saasauth.SaaSMFAChallengeEnrollment, wantNext: saasauth.SaaSMFAStatusActive, wantAllowed: true},
		{name: "login challenge keeps active credential", current: saasauth.SaaSMFAStatusActive, challenge: saasauth.SaaSMFAChallengeLogin, wantNext: saasauth.SaaSMFAStatusActive, wantAllowed: true},
		{name: "enrollment cannot replace active credential", current: saasauth.SaaSMFAStatusActive, challenge: saasauth.SaaSMFAChallengeEnrollment, wantNext: 0, wantAllowed: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			next, allowed := saasMFAStatusTransition(test.current, test.challenge)
			if next != test.wantNext || allowed != test.wantAllowed {
				t.Fatalf("transition(%d, %q) = (%d, %t), want (%d, %t)", test.current, test.challenge, next, allowed, test.wantNext, test.wantAllowed)
			}
		})
	}
}

func TestSaaSMFAFailureUpdateChecksLimitBeforeIncrement(t *testing.T) {
	source, err := os.ReadFile("saas_auth_persistence.go")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(source))
	statusAssignment := strings.Index(sql, "set status = if(attempts + 1 >= max_attempts")
	attemptsAssignment := strings.Index(sql, "attempts = attempts + 1")
	if statusAssignment < 0 || attemptsAssignment < 0 || statusAssignment > attemptsAssignment {
		t.Fatal("RecordMFAFailure must evaluate the max-attempt threshold before incrementing attempts")
	}
}

func TestCompleteSaaSMFAChallengePreservesChallengeReadFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	want := errors.New("database connection interrupted")
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT user_id, auth_version, challenge_type").WillReturnError(want)
	mock.ExpectRollback()

	_, got := NewSaaSIdentityStore(db).CompleteMFAChallenge(context.Background(), [32]byte{1}, 7, 3, saasauth.SaaSMFAChallengeLogin, 123)
	if !errors.Is(got, want) {
		t.Fatalf("CompleteMFAChallenge() error=%v, want infrastructure error", got)
	}
	if errors.Is(got, saasauth.ErrMFAChallengeInvalid) {
		t.Fatalf("CompleteMFAChallenge() collapsed infrastructure error into business rejection: %v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCompleteSaaSMFAChallengeMapsMissingChallengeToBusinessRejection(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT user_id, auth_version, challenge_type").WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	_, got := NewSaaSIdentityStore(db).CompleteMFAChallenge(context.Background(), [32]byte{1}, 7, 3, saasauth.SaaSMFAChallengeLogin, 123)
	if !errors.Is(got, saasauth.ErrMFAChallengeInvalid) {
		t.Fatalf("CompleteMFAChallenge() error=%v, want business rejection", got)
	}
}
