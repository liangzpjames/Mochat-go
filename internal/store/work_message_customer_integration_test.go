package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"jiyi/mochat-go/internal/dashboard"
)

func TestCustomerDirectoryMariaDBIntegration(t *testing.T) {
	if strings.TrimSpace(os.Getenv("MOCHAT_GO_MYSQL_INTEGRATION_DSN")) == "" {
		t.Skip("SKIP: MOCHAT_GO_MYSQL_INTEGRATION_DSN is not set; isolated MariaDB DSN is required")
	}
	db := newCurrentStoreIntegrationDB(t)
	createCustomerDirectoryFixture(t, db)
	store := NewMySQLStore(db)
	ctx := context.Background()

	page, err := store.WorkMessageCustomerDirectory(ctx, dashboard.WorkMessageCustomerDirectoryFilter{TenantID: 11, CorpID: 27, UserID: 77, Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	if page.Counts != (dashboard.WorkMessageCustomerCounts{All: 3, Focused: 1, Active: 1, Lost: 1}) || page.Total != 3 || len(page.Customers) != 3 {
		t.Fatalf("page=%#v", page)
	}
	byID := map[int]dashboard.WorkMessageCustomerDirectoryItem{}
	for _, item := range page.Customers {
		byID[item.ID] = item
	}
	if customer := byID[101]; customer.DirectConversationCount != 2 || customer.GroupConversationCount != 1 || customer.FocusedConversationCount != 1 || customer.ActiveRelationCount != 1 {
		t.Fatalf("deduplicated customer=%#v", customer)
	}
	if customer := byID[102]; customer.ActiveRelationCount != 0 || customer.LostRelationCount != 1 {
		t.Fatalf("lost customer=%#v", customer)
	}
	if customer := byID[103]; customer.ProfileStatus != "missing" {
		t.Fatalf("missing profile customer=%#v", customer)
	}
	for _, leakedID := range []int{104, 105} {
		if _, exists := byID[leakedID]; exists {
			t.Fatalf("cross-corp or deleted room membership leaked customer %d into directory: %#v", leakedID, page)
		}
	}
	for mode, wantID := range map[dashboard.WorkMessageCustomerMode]int{
		dashboard.WorkMessageCustomerModeFocused: 101,
		dashboard.WorkMessageCustomerModeActive:  101,
		dashboard.WorkMessageCustomerModeLost:    102,
	} {
		filtered, err := store.WorkMessageCustomerDirectory(ctx, dashboard.WorkMessageCustomerDirectoryFilter{TenantID: 11, CorpID: 27, UserID: 77, Page: 1, Mode: mode})
		if err != nil {
			t.Fatal(err)
		}
		if filtered.Total != 1 || len(filtered.Customers) != 1 || filtered.Customers[0].ID != wantID {
			t.Fatalf("mode=%s page=%#v", mode, filtered)
		}
	}

	limited, err := store.WorkMessageCustomerDirectory(ctx, dashboard.WorkMessageCustomerDirectoryFilter{
		TenantID: 11, CorpID: 27, UserID: 77, Page: 1, RestrictEmployeeIDs: true, EmployeeIDs: []int{9},
	})
	if err != nil {
		t.Fatal(err)
	}
	if limited.Total != 1 || len(limited.Customers) != 1 || limited.Customers[0].ID != 101 || limited.Counts != (dashboard.WorkMessageCustomerCounts{All: 1, Focused: 1, Active: 1}) {
		t.Fatalf("employee 9 scope leaked employee 10 customer: %#v", limited)
	}
}

func TestCustomerConversationMariaDBIntegration(t *testing.T) {
	if strings.TrimSpace(os.Getenv("MOCHAT_GO_MYSQL_INTEGRATION_DSN")) == "" {
		t.Skip("SKIP: MOCHAT_GO_MYSQL_INTEGRATION_DSN is not set; isolated MariaDB DSN is required")
	}
	db := newCurrentStoreIntegrationDB(t)
	createCustomerDirectoryFixture(t, db)
	base := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)
	statements := []string{
		`INSERT INTO mc_work_contact (id,corp_id,name) VALUES (104,28,'跨企业客户')`,
		`INSERT INTO mc_work_room (id,corp_id,name,notice) VALUES (504,27,'已退群',''),(505,27,'无关联群',''),(506,27,'员工范围群','')`,
		`INSERT INTO mc_work_contact_room (room_id,contact_id,employee_id,deleted_at) VALUES (504,101,9,UTC_TIMESTAMP()),(501,104,9,NULL),(506,102,10,NULL)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	for index, row := range []struct{ employeeID, targetType, targetID int }{
		{9, 1, 101}, // same direct conversation, a later message
		{10, 2, 501},
		{9, 2, 504},
		{9, 2, 505}, // no customer membership: must not be returned
		{9, 2, 506}, // customer 102 belongs only to employee 10: must not authorize employee 9
	} {
		if _, err := db.Exec(`INSERT INTO mc_work_message_1 (corp_id,msgid,seq,work_employee_id,to_user_type,to_user_id,content,content_text,msg_data_time) VALUES (27,?,?,?,?,?,'{}','conversation fixture',?)`,
			"customer-conversation-"+fmt.Sprint(index+1), 100+index, row.employeeID, row.targetType, row.targetID, base.Add(time.Duration(index)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	store := NewMySQLStore(db)
	ctx := context.Background()

	direct, err := store.WorkMessageCustomerConversations(ctx, dashboard.WorkMessageCustomerConversationFilter{TenantID: 11, CorpID: 27, UserID: 77, CustomerID: 101, Mode: dashboard.WorkMessageCustomerConversationModeDirect, Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	if direct.Total != 2 || len(direct.List) != 2 {
		t.Fatalf("direct=%#v", direct)
	}
	for _, conversation := range direct.List {
		if conversation.ConversationID != "9:1:101" && conversation.ConversationID != "10:1:101" {
			t.Fatalf("unexpected direct stable id: %#v", conversation)
		}
	}

	groups, err := store.WorkMessageCustomerConversations(ctx, dashboard.WorkMessageCustomerConversationFilter{TenantID: 11, CorpID: 27, UserID: 77, CustomerID: 101, Mode: dashboard.WorkMessageCustomerConversationModeGroup, Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	if groups.Total != len(groups.List) || groups.Total != 3 {
		t.Fatalf("group count/page must have the same base: %#v", groups)
	}
	seen := map[string]dashboard.WorkMessageCustomerConversation{}
	for _, conversation := range groups.List {
		seen[conversation.ConversationID] = conversation
	}
	for _, id := range []string{"9:2:501", "10:2:501", "9:2:504"} {
		if _, ok := seen[id]; !ok {
			t.Fatalf("missing group stable id %s: %#v", id, groups)
		}
	}
	if seen["9:2:504"].MembershipStatus != "left" {
		t.Fatalf("historical membership must be marked left: %#v", seen["9:2:504"])
	}
	if _, leaked := seen["9:2:505"]; leaked {
		t.Fatalf("room without customer membership leaked: %#v", groups)
	}

	crossCorp, err := store.WorkMessageCustomerConversations(ctx, dashboard.WorkMessageCustomerConversationFilter{TenantID: 11, CorpID: 27, UserID: 77, CustomerID: 104, Mode: dashboard.WorkMessageCustomerConversationModeGroup, Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	if crossCorp.Total != 0 || len(crossCorp.List) != 0 {
		t.Fatalf("cross-corp contact membership leaked group conversations: %#v", crossCorp)
	}

	_, err = store.WorkMessageCustomerConversations(ctx, dashboard.WorkMessageCustomerConversationFilter{TenantID: 11, CorpID: 27, UserID: 77, CustomerID: 102, Mode: dashboard.WorkMessageCustomerConversationModeGroup, Page: 1, RestrictEmployeeIDs: true, EmployeeIDs: []int{9}})
	if err != dashboard.ErrWorkMessageConversationNotFound {
		t.Fatalf("unrelated group member must not authorize employee 9: %v", err)
	}
	_, err = store.WorkMessageCustomerConversations(ctx, dashboard.WorkMessageCustomerConversationFilter{TenantID: 11, CorpID: 27, UserID: 77, CustomerID: 103, Mode: dashboard.WorkMessageCustomerConversationModeDirect, Page: 1, RestrictEmployeeIDs: true, EmployeeIDs: []int{9}})
	if err != dashboard.ErrWorkMessageConversationNotFound {
		t.Fatalf("missing customer profile without allowed relationship must be hidden: %v", err)
	}
}

func TestCustomerDetailMariaDBIntegration(t *testing.T) {
	if strings.TrimSpace(os.Getenv("MOCHAT_GO_MYSQL_INTEGRATION_DSN")) == "" {
		t.Skip("SKIP: MOCHAT_GO_MYSQL_INTEGRATION_DSN is not set; isolated MariaDB DSN is required")
	}
	db := newCurrentStoreIntegrationDB(t)
	createCustomerDirectoryFixture(t, db)
	base := time.Date(2026, 8, 19, 11, 0, 0, 0, time.UTC)
	for index := 1; index <= 52; index++ {
		if _, err := db.Exec(`INSERT INTO mc_work_message_1 (corp_id,msgid,seq,work_employee_id,to_user_type,to_user_id,sender_type,content,content_text,msg_data_time) VALUES (27,?,?,?,?,?,?,'{}',?,?)`,
			"customer-detail-"+fmt.Sprint(index), index, 9, 1, 101, index%2, fmt.Sprintf("detail-%02d", index), base.Add(time.Duration(index)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	store := NewMySQLStore(db)
	filter := dashboard.WorkMessageCustomerDetailFilter{TenantID: 11, CorpID: 27, UserID: 77, CustomerID: 101, EmployeeID: 9, ToUserType: 1, ToUserID: 101, PageSize: 50}
	first, err := store.WorkMessageCustomerDetail(context.Background(), filter)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Messages) != 50 || !first.HasMore || first.NextBefore == "" {
		t.Fatalf("first=%#v", first)
	}
	secondFilter := filter
	secondFilter.Before = first.NextBefore
	second, err := store.WorkMessageCustomerDetail(context.Background(), secondFilter)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]struct{}{}
	lastSentAt := ""
	chronological := append(append([]dashboard.WorkMessageStaffMessage{}, second.Messages...), first.Messages...)
	for _, message := range chronological {
		if _, duplicate := seen[message.ID]; duplicate {
			t.Fatalf("cursor pages overlap at %s", message.ID)
		}
		seen[message.ID] = struct{}{}
		if lastSentAt != "" && message.SentAt < lastSentAt {
			t.Fatalf("messages must stay globally ascending: %s before %s", message.SentAt, lastSentAt)
		}
		lastSentAt = message.SentAt
	}
	keywordFilter := filter
	keywordFilter.Keyword = "detail-01"
	keyword, err := store.WorkMessageCustomerDetail(context.Background(), keywordFilter)
	if err != nil {
		t.Fatal(err)
	}
	if keyword.Stats != first.Stats {
		t.Fatalf("stats must ignore keyword: keyword=%#v all=%#v", keyword.Stats, first.Stats)
	}
	_, err = store.WorkMessageCustomerDetail(context.Background(), dashboard.WorkMessageCustomerDetailFilter{TenantID: 11, CorpID: 27, UserID: 77, CustomerID: 101, EmployeeID: 9, ToUserType: 2, ToUserID: 999, PageSize: 50})
	if !errors.Is(err, dashboard.ErrWorkMessageConversationNotFound) {
		t.Fatalf("unrelated group err=%v", err)
	}
	if _, err := db.Exec(`INSERT INTO mc_work_message_1 (corp_id,msgid,seq,work_employee_id,to_user_type,to_user_id,sender_type,content,content_text,msg_data_time) VALUES (27,'customer-detail-group',999,9,2,501,1,'{}','group inbound',?)`, base.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	group, err := store.WorkMessageCustomerDetail(context.Background(), dashboard.WorkMessageCustomerDetailFilter{TenantID: 11, CorpID: 27, UserID: 77, CustomerID: 101, EmployeeID: 9, ToUserType: 2, ToUserID: 501, PageSize: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(group.Messages) == 0 || group.Messages[len(group.Messages)-1].Direction != "inbound" || group.Messages[len(group.Messages)-1].SenderName != "群成员" {
		t.Fatalf("group inbound identity must remain the staff detail result: %#v", group.Messages)
	}
}

func createCustomerDirectoryFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	statements := []string{
		`INSERT INTO mc_tenant (id,name,status) VALUES (11,'Work message tenant',1)`,
		`INSERT INTO mc_corp (id,tenant_id,chat_status) VALUES (27,11,1)`,
		`INSERT INTO mc_work_employee (id,corp_id,name) VALUES (9,27,'员工甲'),(10,27,'员工乙')`,
		`INSERT INTO mc_work_contact (id,corp_id,name,wx_external_userid) VALUES (101,27,'客户甲','wx-101'),(102,27,'客户乙','wx-102')`,
		`INSERT INTO mc_work_room (id,corp_id,name,notice) VALUES (501,27,'客户甲所在群',''),(502,28,'跨企业群',''),(503,27,'已删除群','')`,
		`UPDATE mc_work_room SET deleted_at=UTC_TIMESTAMP() WHERE id=503`,
		`INSERT INTO mc_work_contact_room (room_id,contact_id,employee_id) VALUES (501,101,9),(502,104,9),(503,105,9)`,
		`INSERT INTO mc_work_contact_employee (contact_id,employee_id,corp_id,status,add_way) VALUES (101,9,27,1,0),(102,10,27,2,0)`,
		`INSERT INTO mochat_go_work_message_focus (tenant_id,corp_id,user_id,work_employee_id,to_user_type,to_user_id) VALUES (11,27,77,9,1,101)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	base := time.Date(2026, 8, 19, 9, 0, 0, 0, time.UTC)
	for index, row := range []struct{ employeeID, targetType, targetID int }{
		{9, 1, 101}, {10, 1, 101}, {9, 2, 501}, {10, 1, 102}, {10, 1, 103}, {9, 2, 502}, {9, 2, 503},
	} {
		if _, err := db.Exec(`INSERT INTO mc_work_message_1 (corp_id,msgid,seq,work_employee_id,to_user_type,to_user_id,content,content_text,msg_data_time) VALUES (27,?,?,?,?,?,'{}','fixture',?)`,
			"customer-directory-"+fmt.Sprint(index+1), index+1, row.employeeID, row.targetType, row.targetID, base.Add(time.Duration(index)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
}
