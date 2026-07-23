package session

import (
	"net/http"
	"testing"
)

func TestResolveLoginCorpInfoUsesWeChatHeaders(t *testing.T) {
	headers := http.Header{}
	headers.Set("mochat-source-type", "wechat-app")
	headers.Set("mochat-corp-id", "12")

	info := ResolveLoginCorpInfo(headers, 7, "99-100", func(userID int, corpID int) int {
		if userID != 7 || corpID != 12 {
			t.Fatalf("resolver got userID=%d corpID=%d", userID, corpID)
		}
		return 34
	}, nil)

	if info.RequestSource != RequestSourceWeChatApp {
		t.Fatalf("RequestSource = %d", info.RequestSource)
	}
	if len(info.CorpIDs) != 1 || info.CorpIDs[0] != 12 {
		t.Fatalf("CorpIDs = %#v", info.CorpIDs)
	}
	if info.WorkEmployeeID != 34 {
		t.Fatalf("WorkEmployeeID = %d", info.WorkEmployeeID)
	}
}

func TestResolveLoginCorpInfoUsesDashboardCache(t *testing.T) {
	info := ResolveLoginCorpInfo(http.Header{}, 7, "12-34", nil, func(userID int) (int, int, bool) {
		t.Fatalf("firstEmployeeByUser should not be called")
		return 0, 0, false
	})

	if info.RequestSource != RequestSourceDashboard {
		t.Fatalf("RequestSource = %d", info.RequestSource)
	}
	if len(info.CorpIDs) != 1 || info.CorpIDs[0] != 12 {
		t.Fatalf("CorpIDs = %#v", info.CorpIDs)
	}
	if info.WorkEmployeeID != 34 {
		t.Fatalf("WorkEmployeeID = %d", info.WorkEmployeeID)
	}
}

func TestResolveLoginCorpInfoFallsBackToFirstEmployee(t *testing.T) {
	info := ResolveLoginCorpInfo(http.Header{}, 7, "", nil, func(userID int) (int, int, bool) {
		if userID != 7 {
			t.Fatalf("userID = %d", userID)
		}
		return 56, 78, true
	})

	if info.RequestSource != RequestSourceDashboard {
		t.Fatalf("RequestSource = %d", info.RequestSource)
	}
	if len(info.CorpIDs) != 1 || info.CorpIDs[0] != 56 {
		t.Fatalf("CorpIDs = %#v", info.CorpIDs)
	}
	if info.WorkEmployeeID != 78 {
		t.Fatalf("WorkEmployeeID = %d", info.WorkEmployeeID)
	}
}

func TestResolveLoginCorpInfoKeepsPHPCastBehaviorForBadCache(t *testing.T) {
	info := ResolveLoginCorpInfo(http.Header{}, 7, "bad-cache", nil, nil)

	if info.RequestSource != RequestSourceDashboard {
		t.Fatalf("RequestSource = %d", info.RequestSource)
	}
	if len(info.CorpIDs) != 1 || info.CorpIDs[0] != 0 {
		t.Fatalf("CorpIDs = %#v", info.CorpIDs)
	}
	if info.WorkEmployeeID != 0 {
		t.Fatalf("WorkEmployeeID = %d", info.WorkEmployeeID)
	}
}
