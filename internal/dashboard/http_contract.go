package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

type User struct {
	ID                              int
	Phone                           string
	Name                            string
	Gender                          int
	Department                      string
	Position                        string
	LoginTime                       string
	Status                          int
	TenantID                        int
	TenantStatus                    int
	TenantPackageExpired            bool
	TenantPackageExpiresAt          string
	TenantSubscriptionManaged       bool
	TenantSubscriptionStatus        string
	TenantSubscriptionAccessAllowed bool
	TenantSubscriptionGraceEndsAt   string
	IsSuperAdmin                    int
}

type Employee struct {
	ID               int
	Name             string
	Mobile           string
	Position         string
	Gender           int
	Email            string
	Avatar           string
	ThumbAvatar      string
	Telephone        string
	Alias            string
	Status           int
	QRCode           string
	ExternalPosition string
	Address          string
}

type Corp struct {
	ID   int
	Name string
}

type LoginCache interface {
	UserCorpCache(ctx context.Context, userID int) (string, error)
}

type UserIDResolver interface {
	UserID(r *http.Request) (int, error)
}

var ErrUnauthorized = errors.New("unauthorized")

type HeaderUserIDResolver struct {
	HeaderName string
}

func (r HeaderUserIDResolver) UserID(req *http.Request) (int, error) {
	header := r.HeaderName
	if header == "" {
		header = "X-Mochat-Go-User-ID"
	}
	raw := req.Header.Get(header)
	userID, err := strconv.Atoi(raw)
	if err != nil || userID <= 0 {
		return 0, ErrUnauthorized
	}
	return userID, nil
}

type envelope struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

func writeEnvelope(w http.ResponseWriter, httpStatus int, code int, msg string, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(httpStatus)
	_ = json.NewEncoder(w).Encode(envelope{Code: code, Msg: msg, Data: data})
}

func writeMachineEnvelope(w http.ResponseWriter, httpStatus int, code, msg string, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(httpStatus)
	_ = json.NewEncoder(w).Encode(struct {
		Code      int    `json:"code"`
		ErrorCode string `json:"errorCode"`
		Msg       string `json:"msg"`
		Data      any    `json:"data"`
	}{Code: httpStatus, ErrorCode: code, Msg: msg, Data: data})
}
