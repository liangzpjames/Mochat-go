package http

import (
	"testing"
	"time"

	"jiyi/mochat-go/internal/modules/scrm/ports"
)

func TestContactViewsNormalizeNilCollections(t *testing.T) {
	summary := ports.ContactSummary{ID: "c1", Name: "张三", UpdatedAt: time.Unix(0, 0).UTC()}
	view := contactSummaryView(summary)
	if view.TagNames == nil || len(view.TagNames) != 0 {
		t.Fatalf("tagNames should be empty slice, got %#v", view.TagNames)
	}
	detail := contactDetailView(ports.ContactDetail{ContactSummary: summary})
	if detail.Tags == nil || detail.WeComFriends == nil || detail.Opportunities == nil || detail.FollowUps == nil {
		t.Fatalf("detail collections should be empty slices, got tags=%#v wecom=%#v opps=%#v followUps=%#v", detail.Tags, detail.WeComFriends, detail.Opportunities, detail.FollowUps)
	}
}
