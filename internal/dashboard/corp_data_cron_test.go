package dashboard

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestCorpDataCronRefreshesActiveCorps(t *testing.T) {
	now := time.Date(2026, 7, 4, 12, 30, 0, 0, time.Local)
	store := &fakeCorpDataCronStore{corpIDs: []int{7, 8}}
	cron := NewCorpDataCron(store, nil)
	cron.now = func() time.Time { return now }

	if err := cron.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(store.refreshedCorpIDs, []int{7, 8}) {
		t.Fatalf("refreshed corp ids = %#v", store.refreshedCorpIDs)
	}
	if len(store.refreshTimes) != 2 || !store.refreshTimes[0].Equal(now) || !store.refreshTimes[1].Equal(now) {
		t.Fatalf("refresh times = %#v", store.refreshTimes)
	}
}

func TestCorpDataCronContinuesAfterCorpError(t *testing.T) {
	store := &fakeCorpDataCronStore{
		corpIDs: []int{7, 8},
		failures: map[int]error{
			7: errors.New("boom"),
		},
	}
	cron := NewCorpDataCron(store, nil)

	err := cron.RunOnce(context.Background())
	if err == nil || !strings.Contains(err.Error(), "corp 7") {
		t.Fatalf("error = %v", err)
	}
	if !reflect.DeepEqual(store.refreshedCorpIDs, []int{7, 8}) {
		t.Fatalf("refreshed corp ids = %#v", store.refreshedCorpIDs)
	}
}

func TestCorpDataCronRequiresStore(t *testing.T) {
	cron := NewCorpDataCron(nil, nil)
	err := cron.RunOnce(context.Background())
	if err == nil || !strings.Contains(err.Error(), "store is not configured") {
		t.Fatalf("error = %v", err)
	}
}

type fakeCorpDataCronStore struct {
	corpIDs          []int
	failures         map[int]error
	refreshedCorpIDs []int
	refreshTimes     []time.Time
}

func (s *fakeCorpDataCronStore) ActiveCorpIDs(context.Context) ([]int, error) {
	return append([]int{}, s.corpIDs...), nil
}

func (s *fakeCorpDataCronStore) RefreshCorpDayData(_ context.Context, corpID int, now time.Time) (CorpDataCronResult, error) {
	s.refreshedCorpIDs = append(s.refreshedCorpIDs, corpID)
	s.refreshTimes = append(s.refreshTimes, now)
	if err := s.failures[corpID]; err != nil {
		return CorpDataCronResult{}, err
	}
	return CorpDataCronResult{CorpID: corpID, Date: now.Format("2006-01-02")}, nil
}
