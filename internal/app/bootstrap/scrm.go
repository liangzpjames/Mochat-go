package bootstrap

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"time"

	appmodules "jiyi/mochat-go/internal/app/modules"
	"jiyi/mochat-go/internal/authjwt"
	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/modules/scrm"
	transporthttp "jiyi/mochat-go/internal/modules/scrm/transport/http"
)

type SCRMDependencies struct {
	DB                *sql.DB
	PrincipalResolver transporthttp.PrincipalResolver
}

func RegisterSCRM(router *appmodules.Router, enabled bool, dependencies SCRMDependencies) error {
	if !enabled {
		return nil
	}
	module, err := scrm.New(scrm.Dependencies{
		DB:                dependencies.DB,
		Clock:             scrmClock{},
		IDGenerator:       scrmIDGenerator{},
		PrincipalResolver: dependencies.PrincipalResolver,
	})
	if err != nil {
		return err
	}
	return module.RegisterRoutes(router)
}

type SCRMUserStore interface {
	UserByID(context.Context, int) (dashboard.User, bool, error)
}

type scrmPrincipalResolver struct {
	userIDs dashboard.UserIDResolver
	users   SCRMUserStore
}

func NewSCRMPrincipalResolver(userIDs dashboard.UserIDResolver, users SCRMUserStore) (transporthttp.PrincipalResolver, error) {
	if isNilSCRMDependency(userIDs) {
		return nil, errors.New("SCRM user ID resolver is required")
	}
	if isNilSCRMDependency(users) {
		return nil, errors.New("SCRM user store is required")
	}
	return scrmPrincipalResolver{userIDs: userIDs, users: users}, nil
}

func (r scrmPrincipalResolver) Resolve(request *http.Request) (transporthttp.Principal, error) {
	if request == nil {
		return transporthttp.Principal{}, transporthttp.ErrPrincipalUnauthorized
	}
	userID, err := r.userIDs.UserID(request)
	if err != nil {
		if isSCRMCredentialError(err) {
			return transporthttp.Principal{}, transporthttp.ErrPrincipalUnauthorized
		}
		return transporthttp.Principal{}, transporthttp.ErrPrincipalUnavailable
	}
	if userID <= 0 {
		return transporthttp.Principal{}, transporthttp.ErrPrincipalUnauthorized
	}
	user, found, err := r.users.UserByID(request.Context(), userID)
	if err != nil {
		return transporthttp.Principal{}, transporthttp.ErrPrincipalUnavailable
	}
	if !found || user.TenantID <= 0 {
		return transporthttp.Principal{}, transporthttp.ErrPrincipalUnauthorized
	}
	return transporthttp.Principal{UserID: int64(userID), TenantID: int64(user.TenantID)}, nil
}

func isSCRMCredentialError(err error) bool {
	for _, target := range []error{
		dashboard.ErrUnauthorized,
		authjwt.ErrUnauthorized,
		authjwt.ErrInvalidToken,
		authjwt.ErrInvalidSignature,
		authjwt.ErrTokenExpired,
		authjwt.ErrTokenNotActive,
		authjwt.ErrTokenBlacklisted,
		authjwt.ErrSessionInvalid,
	} {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}

type scrmClock struct{}

func (scrmClock) Now() time.Time {
	return time.Now().UTC()
}

type scrmIDGenerator struct{}

func (scrmIDGenerator) NewID() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate SCRM ID: %w", err)
	}
	random[6] = random[6]&0x0f | 0x40
	random[8] = random[8]&0x3f | 0x80

	var encoded [36]byte
	hex.Encode(encoded[0:8], random[0:4])
	encoded[8] = '-'
	hex.Encode(encoded[9:13], random[4:6])
	encoded[13] = '-'
	hex.Encode(encoded[14:18], random[6:8])
	encoded[18] = '-'
	hex.Encode(encoded[19:23], random[8:10])
	encoded[23] = '-'
	hex.Encode(encoded[24:36], random[10:16])
	return string(encoded[:]), nil
}

func isNilSCRMDependency(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
