package dashboard

import (
	"context"
	"encoding/json"
	"testing"
)

type recordingAuthorizer struct {
	permissionKey  string
	corpID         int
	workEmployeeID int
	access         AccessContext
	accessSet      bool
	err            error
}

func (a *recordingAuthorizer) Resolve(_ context.Context, _ int, permissionKey string, corpID int, workEmployeeID int) (AccessContext, error) {
	a.permissionKey = permissionKey
	a.corpID = corpID
	a.workEmployeeID = workEmployeeID
	if a.accessSet {
		return a.access, a.err
	}
	return AccessContext{}, a.err
}

type staticAdminCache string

func (c staticAdminCache) UserCorpCache(context.Context, int) (string, error) {
	return string(c), nil
}

type staticCache string

func (c staticCache) UserCorpCache(context.Context, int) (string, error) {
	return string(c), nil
}

func decodeBody(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	return out
}
