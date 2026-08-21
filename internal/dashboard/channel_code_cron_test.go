package dashboard

import (
	"context"
	"testing"
	"time"
)

func TestChannelCodeCronUpdatesExistingContactWay(t *testing.T) {
	store := &fakeChannelCodeCronStore{
		items: []ChannelCodeCronItem{{
			ID:            900003,
			CorpID:        7,
			AutoAddFriend: 1,
			WXConfigID:    "config-old",
			DrainageEmployee: map[string]any{
				"employees": []any{
					map[string]any{
						"week": 6,
						"timeSlot": []any{
							map[string]any{"startTime": "00:00", "endTime": "00:00", "employeeId": []any{31}},
						},
					},
				},
			},
		}},
		wxUserIDs:  []string{"go-user"},
		credential: RoomWelcomeCorpCredential{WXCorpID: "ww-go", ContactSecret: "contact-secret"},
	}
	client := &fakeChannelCodeCronClient{}
	cron := NewChannelCodeCron(store, client, nil)
	cron.now = func() time.Time { return time.Date(2026, 7, 4, 10, 0, 0, 0, time.Local) }

	if err := cron.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if client.updateConfigID != "config-old" || client.updateState != "channelCode-900003" || !client.updateSkipVerify || len(client.updateUsers) != 1 || client.updateUsers[0] != "go-user" {
		t.Fatalf("update call = config %q state %q skip %v users %#v", client.updateConfigID, client.updateState, client.updateSkipVerify, client.updateUsers)
	}
	if store.qrCodeID != 0 || store.deletedID != 0 {
		t.Fatalf("store qr=%d deleted=%d", store.qrCodeID, store.deletedID)
	}
}

func TestChannelCodeCronCreatesMissingContactWay(t *testing.T) {
	store := &fakeChannelCodeCronStore{
		items: []ChannelCodeCronItem{{
			ID:            900004,
			CorpID:        7,
			AutoAddFriend: 2,
			DrainageEmployee: map[string]any{
				"employees": []any{
					map[string]any{
						"week": 6,
						"timeSlot": []any{
							map[string]any{"startTime": "00:00", "endTime": "00:00", "employeeId": []any{31}},
						},
					},
				},
			},
		}},
		wxUserIDs:  []string{"go-user"},
		credential: RoomWelcomeCorpCredential{WXCorpID: "ww-go", ContactSecret: "contact-secret"},
	}
	client := &fakeChannelCodeCronClient{createQRCode: "https://example.com/qrcode.png", createConfigID: "config-new"}
	cron := NewChannelCodeCron(store, client, nil)
	cron.now = func() time.Time { return time.Date(2026, 7, 4, 10, 0, 0, 0, time.Local) }

	if err := cron.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if client.createState != "channelCode-900004" || client.createSkipVerify || len(client.createUsers) != 1 || client.createUsers[0] != "go-user" {
		t.Fatalf("create call = state %q skip %v users %#v", client.createState, client.createSkipVerify, client.createUsers)
	}
	if store.qrCodeID != 900004 || store.qrCodeURL != "https://example.com/qrcode.png" || store.configID != "config-new" {
		t.Fatalf("qr update = id %d url %q config %q", store.qrCodeID, store.qrCodeURL, store.configID)
	}
}

func TestChannelCodeCronRequiresDependencies(t *testing.T) {
	cron := NewChannelCodeCron(nil, nil, nil)
	err := cron.RunOnce(context.Background())
	if err == nil || err.Error() != "channelCode cron dependencies are not configured" {
		t.Fatalf("err = %v", err)
	}
}

func TestChannelCodeCronExpiresFixedValidityAfterProviderSync(t *testing.T) {
	store := &fakeChannelCodeCronStore{
		items: []ChannelCodeCronItem{{
			ID: 900005, CorpID: 7, WXConfigID: "config-expired",
			ValidUntil: "2026-07-03 23:00:00", LifecycleState: "active",
		}},
		credential:       RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "secret"},
		providerConfigID: "config-expired",
	}
	client := &fakeChannelCodeCronClient{}
	cron := NewChannelCodeCron(store, client, nil)
	cron.now = func() time.Time { return time.Date(2026, 7, 4, 10, 0, 0, 0, time.Local) }

	if err := cron.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if client.deletedConfigID != "config-expired" || store.lifecycleState != "expired" {
		t.Fatalf("expiry sync = client %q state %q", client.deletedConfigID, store.lifecycleState)
	}
}

type fakeChannelCodeCronStore struct {
	items            []ChannelCodeCronItem
	wxUserIDs        []string
	credential       RoomWelcomeCorpCredential
	qrCodeID         int
	qrCodeURL        string
	configID         string
	deletedID        int
	providerConfigID string
	lifecycleState   string
}

func (s *fakeChannelCodeCronStore) ChannelCodesForCron(context.Context) ([]ChannelCodeCronItem, error) {
	return s.items, nil
}

func (s *fakeChannelCodeCronStore) ChannelCodeEmployeeWXUserIDs(context.Context, []int) ([]string, error) {
	return s.wxUserIDs, nil
}

func (s *fakeChannelCodeCronStore) ChannelCodeContactCountsByEmployee(context.Context, []int) (map[int]int, error) {
	return map[int]int{}, nil
}

func (s *fakeChannelCodeCronStore) RoomWelcomeCorpCredentialByID(context.Context, int) (RoomWelcomeCorpCredential, bool, error) {
	return s.credential, true, nil
}

func (s *fakeChannelCodeCronStore) UpdateChannelCodeQRCode(_ context.Context, channelCodeID int, qrCodeURL string, wxConfigID string) error {
	s.qrCodeID = channelCodeID
	s.qrCodeURL = qrCodeURL
	s.configID = wxConfigID
	return nil
}

func (s *fakeChannelCodeCronStore) DeleteChannelCode(_ context.Context, channelCodeID int) error {
	s.deletedID = channelCodeID
	return nil
}

func (s *fakeChannelCodeCronStore) ChannelCodeProviderConfig(_ context.Context, _ int, _ int) (RoomWelcomeCorpCredential, string, bool, error) {
	return s.credential, s.providerConfigID, s.providerConfigID != "", nil
}

func (s *fakeChannelCodeCronStore) SetChannelCodeLifecycle(_ context.Context, _ int, _ int, state string, _ string, _ string) error {
	s.lifecycleState = state
	return nil
}

type fakeChannelCodeCronClient struct {
	createQRCode     string
	createConfigID   string
	createUsers      []string
	createSkipVerify bool
	createState      string
	updateConfigID   string
	updateUsers      []string
	updateSkipVerify bool
	updateState      string
	deletedConfigID  string
}

func (c *fakeChannelCodeCronClient) CreateContactWay(_ context.Context, _ RoomWelcomeCorpCredential, userIDs []string, skipVerify bool, state string) (string, string, error) {
	c.createUsers = append([]string{}, userIDs...)
	c.createSkipVerify = skipVerify
	c.createState = state
	return c.createQRCode, c.createConfigID, nil
}

func (c *fakeChannelCodeCronClient) UpdateContactWay(_ context.Context, _ RoomWelcomeCorpCredential, configID string, userIDs []string, skipVerify bool, state string) error {
	c.updateConfigID = configID
	c.updateUsers = append([]string{}, userIDs...)
	c.updateSkipVerify = skipVerify
	c.updateState = state
	return nil
}

func (c *fakeChannelCodeCronClient) DeleteContactWay(_ context.Context, _ RoomWelcomeCorpCredential, configID string) error {
	c.deletedConfigID = configID
	return nil
}
