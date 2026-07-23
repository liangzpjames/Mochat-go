package dashboard

import (
	"context"
	"path/filepath"
	"testing"
)

func TestContactWelcomeWorkerSendsTextAndLink(t *testing.T) {
	store := &fakeContactWelcomeStore{
		credential: RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "contact-secret"},
	}
	client := &fakeContactWelcomeClient{}
	worker := NewContactWelcomeWorker(nil, store, client, t.TempDir(), "http://api.example.com", nil)

	err := worker.Process(context.Background(), ContactWelcomeEvent{
		CorpID:      7,
		ContactID:   101,
		EmployeeID:  3,
		ContactName: "客户A",
		WelcomeCode: "welcome-code",
		Content: ContactWelcomeContent{
			Text: "你好，##客户名称##",
			Medium: &ContactWelcomeMedium{
				MediumType: 3,
				MediumContent: map[string]any{
					"title":       "资料",
					"imageLink":   "https://example.com/doc",
					"imagePath":   "image/a.jpg",
					"description": "说明",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Process error = %v", err)
	}
	if client.sentWelcomeCode != "welcome-code" || client.sentCredential.CorpID != 7 {
		t.Fatalf("sent welcome code=%q credential=%#v", client.sentWelcomeCode, client.sentCredential)
	}
	if client.sentPayload.Text == nil || client.sentPayload.Text.Content != "你好，客户A" {
		t.Fatalf("text payload = %#v", client.sentPayload.Text)
	}
	if len(client.sentPayload.Attachments) != 1 || client.sentPayload.Attachments[0].MsgType != "link" {
		t.Fatalf("attachments = %#v", client.sentPayload.Attachments)
	}
	link := client.sentPayload.Attachments[0].Link
	if link == nil || link.Title != "资料" || link.URL != "https://example.com/doc" || link.PicURL != "http://api.example.com/static/image/a.jpg" || link.Desc != "说明" {
		t.Fatalf("link = %#v", link)
	}
}

func TestContactWelcomeWorkerUploadsImageAndMiniProgramPictures(t *testing.T) {
	root := t.TempDir()
	store := &fakeContactWelcomeStore{
		credential: RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "contact-secret"},
	}
	client := &fakeContactWelcomeClient{uploadMediaID: "media-id"}
	worker := NewContactWelcomeWorker(nil, store, client, root, "", nil)

	err := worker.Process(context.Background(), ContactWelcomeEvent{
		CorpID:      7,
		ContactID:   101,
		EmployeeID:  3,
		WelcomeCode: "welcome-code",
		Content: ContactWelcomeContent{
			Medium: &ContactWelcomeMedium{
				MediumType:    2,
				MediumContent: map[string]any{"imagePath": "image/welcome.jpg"},
			},
		},
	})
	if err != nil {
		t.Fatalf("image Process error = %v", err)
	}
	expectedImagePath := filepath.Join(root, "image", "welcome.jpg")
	if client.uploadedPaths[0] != expectedImagePath || client.sentPayload.Attachments[0].Image.MediaID != "media-id" {
		t.Fatalf("image upload paths=%#v payload=%#v", client.uploadedPaths, client.sentPayload)
	}

	client.sentPayload = ContactWelcomePayload{}
	err = worker.Process(context.Background(), ContactWelcomeEvent{
		CorpID:      7,
		ContactID:   102,
		EmployeeID:  3,
		WelcomeCode: "welcome-code-2",
		Content: ContactWelcomeContent{
			Medium: &ContactWelcomeMedium{
				MediumType: 6,
				MediumContent: map[string]any{
					"title":     "小程序",
					"imagePath": "image/mini.jpg",
					"appid":     "wx-app",
					"page":      "pages/index",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("miniprogram Process error = %v", err)
	}
	mini := client.sentPayload.Attachments[0].MiniProgram
	if mini == nil || mini.Title != "小程序" || mini.PicMediaID != "media-id" || mini.AppID != "wx-app" || mini.Page != "pages/index" {
		t.Fatalf("miniprogram = %#v", mini)
	}
}

type fakeContactWelcomeStore struct {
	credential RoomWelcomeCorpCredential
}

func (s *fakeContactWelcomeStore) RoomWelcomeCorpCredentialByID(_ context.Context, corpID int) (RoomWelcomeCorpCredential, bool, error) {
	if s.credential.CorpID == 0 {
		s.credential.CorpID = corpID
	}
	return s.credential, s.credential.WXCorpID != "", nil
}

type fakeContactWelcomeClient struct {
	uploadMediaID   string
	uploadedPaths   []string
	sentCredential  RoomWelcomeCorpCredential
	sentWelcomeCode string
	sentPayload     ContactWelcomePayload
}

func (c *fakeContactWelcomeClient) UploadTemporaryImage(_ context.Context, _ RoomWelcomeCorpCredential, filePath string) (string, error) {
	c.uploadedPaths = append(c.uploadedPaths, filePath)
	if c.uploadMediaID != "" {
		return c.uploadMediaID, nil
	}
	return "media-id", nil
}

func (c *fakeContactWelcomeClient) SendExternalContactWelcome(_ context.Context, credential RoomWelcomeCorpCredential, welcomeCode string, payload ContactWelcomePayload) error {
	c.sentCredential = credential
	c.sentWelcomeCode = welcomeCode
	c.sentPayload = payload
	return nil
}
