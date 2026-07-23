package dashboard

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMediumMediaCronUploadsExpiredMedia(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "image"), 0o755); err != nil {
		t.Fatal(err)
	}
	localPath := filepath.Join(root, "image", "hello.txt")
	if err := os.WriteFile(localPath, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2_000_000_000, 0)
	store := &fakeMediumMediaCronStore{
		corpIDs: []int{7},
		itemsByCorp: map[int][]MediumMediaUpdateItem{
			7: {
				{
					ID:             21,
					Type:           2,
					MediaID:        "old-media-id",
					LastUploadTime: 1,
					Content:        map[string]any{"imagePath": "image/hello.txt"},
				},
			},
		},
		credential: MediumCorpCredential{CorpID: 7, WXCorpID: "wx-corp", EmployeeSecret: "employee-secret"},
		corpFound:  true,
	}
	client := &fakeMediumMediaClient{mediaID: "cron-media-id"}
	cron := NewMediumMediaCron(store, client, root, nil)
	cron.now = func() time.Time { return now }

	if err := cron.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if client.calls != 1 || client.lastMediaType != "image" || client.lastFilePath != localPath {
		t.Fatalf("client calls=%d type=%q path=%q", client.calls, client.lastMediaType, client.lastFilePath)
	}
	if store.updatedMediumID != 21 || store.updatedMediaID != "cron-media-id" || store.updatedLastUploadTime != now.Unix() {
		t.Fatalf("update = id %d media %q time %d", store.updatedMediumID, store.updatedMediaID, store.updatedLastUploadTime)
	}
	expectedOlderThan := now.Unix() - mediumTemporaryMediaFreshLimit
	if store.olderThan != expectedOlderThan {
		t.Fatalf("olderThan = %d want %d", store.olderThan, expectedOlderThan)
	}
}

func TestMediumMediaCronSkipsMissingFile(t *testing.T) {
	store := &fakeMediumMediaCronStore{
		corpIDs: []int{7},
		itemsByCorp: map[int][]MediumMediaUpdateItem{
			7: {
				{
					ID:             21,
					Type:           2,
					MediaID:        "old-media-id",
					LastUploadTime: 1,
					Content:        map[string]any{"imagePath": "image/missing.txt"},
				},
			},
		},
		credential: MediumCorpCredential{CorpID: 7, WXCorpID: "wx-corp", EmployeeSecret: "employee-secret"},
		corpFound:  true,
	}
	client := &fakeMediumMediaClient{mediaID: "cron-media-id"}
	cron := NewMediumMediaCron(store, client, t.TempDir(), nil)
	cron.now = func() time.Time { return time.Unix(2_000_000_000, 0) }

	if err := cron.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if client.calls != 0 {
		t.Fatalf("client calls = %d", client.calls)
	}
	if store.updatedMediumID != 0 {
		t.Fatalf("updated medium id = %d", store.updatedMediumID)
	}
}

func TestMediumMediaCronRequiresDependencies(t *testing.T) {
	cron := NewMediumMediaCron(nil, nil, "", nil)
	err := cron.RunOnce(context.Background())
	if err == nil || !strings.Contains(err.Error(), "dependencies are not configured") {
		t.Fatalf("error = %v", err)
	}
}

type fakeMediumMediaCronStore struct {
	corpIDs               []int
	itemsByCorp           map[int][]MediumMediaUpdateItem
	credential            MediumCorpCredential
	corpFound             bool
	olderThan             int64
	updatedMediumID       int
	updatedMediaID        string
	updatedLastUploadTime int64
}

func (s *fakeMediumMediaCronStore) ActiveCorpIDs(context.Context) ([]int, error) {
	return append([]int{}, s.corpIDs...), nil
}

func (s *fakeMediumMediaCronStore) MediumMediaForUpdateByCorp(_ context.Context, corpID int, olderThan int64) ([]MediumMediaUpdateItem, error) {
	s.olderThan = olderThan
	return append([]MediumMediaUpdateItem{}, s.itemsByCorp[corpID]...), nil
}

func (s *fakeMediumMediaCronStore) MediumCorpCredentialByID(context.Context, int) (MediumCorpCredential, bool, error) {
	return s.credential, s.corpFound, nil
}

func (s *fakeMediumMediaCronStore) UpdateMediumMediaID(_ context.Context, mediumID int, mediaID string, lastUploadTime int64) (bool, error) {
	s.updatedMediumID = mediumID
	s.updatedMediaID = mediaID
	s.updatedLastUploadTime = lastUploadTime
	return true, nil
}
