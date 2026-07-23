package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMessageRemindEventSupportsPHPStyleObjectPayload(t *testing.T) {
	var event MessageRemindEvent
	if err := json.Unmarshal([]byte(`{"corpId":"7","toParty":[1,2],"msgType":"text","content":"部门提醒","extra":{"safe":1},"source":"php"}`), &event); err != nil {
		t.Fatal(err)
	}
	toType, recipients, err := event.recipientTarget()
	if err != nil {
		t.Fatal(err)
	}
	if event.CorpID != 7 || event.MsgType != "text" || event.Content != "部门提醒" || toType != "party" || len(recipients) != 2 || recipients[0] != "1" || recipients[1] != "2" {
		t.Fatalf("event = %+v toType=%s recipients=%v", event, toType, recipients)
	}
}

func TestMessageRemindEventSupportsLegacyArrayPayload(t *testing.T) {
	var employee MessageRemindEvent
	if err := json.Unmarshal([]byte(`[7,["go-a","go-b"],"text","员工提醒",{"safe":1}]`), &employee); err != nil {
		t.Fatal(err)
	}
	toType, recipients, err := employee.recipientTarget()
	if err != nil {
		t.Fatal(err)
	}
	if employee.CorpID != 7 || toType != "user" || len(recipients) != 2 || recipients[0] != "go-a" || recipients[1] != "go-b" {
		t.Fatalf("employee = %+v toType=%s recipients=%v", employee, toType, recipients)
	}

	var tag MessageRemindEvent
	if err := json.Unmarshal([]byte(`[7,"tag",[3,4],"text","标签提醒",{}]`), &tag); err != nil {
		t.Fatal(err)
	}
	toType, recipients, err = tag.recipientTarget()
	if err != nil {
		t.Fatal(err)
	}
	if toType != "tag" || len(recipients) != 2 || recipients[0] != "3" || recipients[1] != "4" {
		t.Fatalf("tag = %+v toType=%s recipients=%v", tag, toType, recipients)
	}
}

func TestMessageRemindWorkerSendsTextMessage(t *testing.T) {
	store := &fakeMessageRemindWorkerStore{
		agent:          messageRemindAgentFixture(),
		agentFound:     true,
		medium:         MediumCorpCredential{CorpID: 7, WXCorpID: "ww-go", EmployeeSecret: "employee-secret"},
		mediumFound:    true,
		tenantByCorpID: map[int]int{7: 11},
	}
	client := &fakeMessageRemindWorkerClient{}
	worker := NewMessageRemindWorker(nil, store, client, t.TempDir(), log.New(io.Discard, "", 0))

	err := worker.Process(context.Background(), MessageRemindEvent{
		CorpID:  7,
		ToUser:  MessageRemindRecipients{" go-a ", "go-b"},
		MsgType: "text",
		Content: "员工提醒",
		Extra:   map[string]any{"safe": json.Number("1"), "duplicate_check_interval": "600"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if client.sent.MsgType != "text" || client.sent.ToUser != "go-a|go-b" || client.sent.Content != "员工提醒" {
		t.Fatalf("sent = %+v", client.sent)
	}
	if client.sent.Extra["safe"] != json.Number("1") {
		t.Fatalf("extra = %+v", client.sent.Extra)
	}
}

func TestMessageRemindWorkerUploadsPathContent(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "medium"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "medium", "image.png"), []byte("fake-image"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := &fakeMessageRemindWorkerStore{
		agent:       messageRemindAgentFixture(),
		agentFound:  true,
		medium:      MediumCorpCredential{CorpID: 7, WXCorpID: "ww-go", EmployeeSecret: "employee-secret"},
		mediumFound: true,
	}
	client := &fakeMessageRemindWorkerClient{uploadMediaID: "media-go"}
	worker := NewMessageRemindWorker(nil, store, client, root, log.New(io.Discard, "", 0))

	err := worker.Process(context.Background(), MessageRemindEvent{
		CorpID:  7,
		ToParty: MessageRemindRecipients{"1", "2"},
		MsgType: "image",
		Content: map[string]any{"path": "medium/image.png"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if client.uploadMediaType != "image" || client.uploadFilePath != filepath.Join(root, "medium", "image.png") {
		t.Fatalf("upload mediaType=%q filePath=%q", client.uploadMediaType, client.uploadFilePath)
	}
	content, ok := client.sent.Content.(map[string]any)
	if !ok {
		t.Fatalf("content type = %T", client.sent.Content)
	}
	if content["media_id"] != "media-go" {
		t.Fatalf("content = %+v", content)
	}
	if _, ok := content["path"]; ok {
		t.Fatalf("path should be removed after upload: %+v", content)
	}
	if client.sent.ToParty != "1|2" {
		t.Fatalf("toParty = %q", client.sent.ToParty)
	}
}

func TestMessageRemindWorkerAcksSuccessfulDelivery(t *testing.T) {
	queue := &fakeMessageRemindQueue{}
	store := &fakeMessageRemindWorkerStore{agent: messageRemindAgentFixture(), agentFound: true, tenantByCorpID: map[int]int{7: 11}}
	worker := NewMessageRemindWorker(queue, store, &fakeMessageRemindWorkerClient{}, t.TempDir(), log.New(io.Discard, "", 0))

	worker.handleDelivery(context.Background(), MessageRemindDelivery{
		Event:    MessageRemindEvent{CorpID: 7, ToUser: MessageRemindRecipients{"go-user"}, MsgType: "text", Content: "提醒"},
		Raw:      "raw",
		Attempts: 0,
	})
	if !queue.acked || queue.retried {
		t.Fatalf("queue acked=%v retried=%v", queue.acked, queue.retried)
	}
}

func TestMessageRemindWorkerRetriesFailedDelivery(t *testing.T) {
	queue := &fakeMessageRemindQueue{}
	store := &fakeMessageRemindWorkerStore{agentErr: fmt.Errorf("boom")}
	worker := NewMessageRemindWorker(queue, store, &fakeMessageRemindWorkerClient{}, t.TempDir(), log.New(io.Discard, "", 0))

	worker.handleDelivery(context.Background(), MessageRemindDelivery{
		Event:    MessageRemindEvent{CorpID: 7, ToUser: MessageRemindRecipients{"go-user"}, MsgType: "text", Content: "提醒"},
		Raw:      "raw",
		Attempts: 0,
	})
	if queue.acked || !queue.retried || queue.retryReason == "" {
		t.Fatalf("queue acked=%v retried=%v reason=%q", queue.acked, queue.retried, queue.retryReason)
	}
}

func TestWorkAgentMessageRequestMatchesWeComPayload(t *testing.T) {
	request, err := workAgentMessageRequest(messageRemindAgentFixture(), WorkAgentMessagePayload{
		ToTag:   "3|4",
		MsgType: "text",
		Content: "标签提醒",
		Extra: map[string]any{
			"safe":                     json.Number("1"),
			"enable_id_trans":          "1",
			"enable_duplicate_check":   1,
			"duplicate_check_interval": "600",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if request["totag"] != "3|4" || request["msgtype"] != "text" || request["agentid"] != "100001" {
		t.Fatalf("request = %+v", request)
	}
	if request["safe"] != 1 || request["enable_id_trans"] != 1 || request["enable_duplicate_check"] != 1 || request["duplicate_check_interval"] != 600 {
		t.Fatalf("extra fields = %+v", request)
	}
	text, ok := request["text"].(map[string]any)
	if !ok || text["content"] != "标签提醒" {
		t.Fatalf("text = %+v", request["text"])
	}
}

func messageRemindAgentFixture() RoomTagPullAgentCredential {
	return RoomTagPullAgentCredential{CorpID: 7, WXCorpID: "ww-go", WXAgentID: "100001", WXSecret: "agent-secret"}
}

type fakeMessageRemindWorkerStore struct {
	agent          RoomTagPullAgentCredential
	agentFound     bool
	agentErr       error
	medium         MediumCorpCredential
	mediumFound    bool
	mediumErr      error
	tenantByCorpID map[int]int
}

func (s *fakeMessageRemindWorkerStore) TenantIDByCorpID(_ context.Context, corpID int) (int, error) {
	if s.tenantByCorpID == nil {
		return 0, nil
	}
	return s.tenantByCorpID[corpID], nil
}

func (s *fakeMessageRemindWorkerStore) RoomTagPullRemindAgentByCorpID(_ context.Context, _ int) (RoomTagPullAgentCredential, bool, error) {
	return s.agent, s.agentFound, s.agentErr
}

func (s *fakeMessageRemindWorkerStore) MediumCorpCredentialByID(_ context.Context, _ int) (MediumCorpCredential, bool, error) {
	return s.medium, s.mediumFound, s.mediumErr
}

type fakeMessageRemindWorkerClient struct {
	sent            WorkAgentMessagePayload
	uploadMediaID   string
	uploadMediaType string
	uploadFilePath  string
	sendErr         error
	uploadErr       error
}

func (c *fakeMessageRemindWorkerClient) SendAgentMessage(_ context.Context, _ RoomTagPullAgentCredential, payload WorkAgentMessagePayload) error {
	c.sent = payload
	return c.sendErr
}

func (c *fakeMessageRemindWorkerClient) UploadTemporaryMedia(_ context.Context, _ MediumCorpCredential, mediaType string, filePath string) (string, error) {
	c.uploadMediaType = mediaType
	c.uploadFilePath = filePath
	if c.uploadErr != nil {
		return "", c.uploadErr
	}
	if c.uploadMediaID == "" {
		return "media-id", nil
	}
	return c.uploadMediaID, nil
}

type fakeMessageRemindQueue struct {
	acked       bool
	retried     bool
	retryReason string
	deadLetter  bool
}

func (q *fakeMessageRemindQueue) DequeueMessageRemind(_ context.Context, _ time.Duration) (MessageRemindDelivery, bool, error) {
	return MessageRemindDelivery{}, false, nil
}

func (q *fakeMessageRemindQueue) AckMessageRemind(_ context.Context, _ MessageRemindDelivery) error {
	q.acked = true
	return nil
}

func (q *fakeMessageRemindQueue) RetryMessageRemind(_ context.Context, _ MessageRemindDelivery, reason string, _ int) (bool, error) {
	q.retried = true
	q.retryReason = reason
	return q.deadLetter, nil
}

func (q *fakeMessageRemindQueue) RecoverMessageRemindProcessing(_ context.Context, _ time.Duration, _ int) (int, error) {
	return 0, nil
}
