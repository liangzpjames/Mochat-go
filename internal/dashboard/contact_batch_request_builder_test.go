package dashboard

import "testing"

// TestContactBatchRequestBuilderContentTypeCompatibility (P1-9) locks the
// msgType -> add_msg_template field mapping for every supported content type:
// text, image, link and miniprogram. Unsupported combinations fail closed at
// the request builder, so the durable sender can never emit a payload the
// official contract would reject.
func TestContactBatchRequestBuilderContentTypeCompatibility(t *testing.T) {
	text := contactMessageBatchSendWeComRequest(ContactMessageBatchSendMessagePayload{Sender: "employee-1", ExternalUserID: []string{"external-1"}, Content: []ContactMessageBatchSendContent{{MsgType: "text", Content: "hello"}}})
	if text["chat_type"] != "single" || text["sender"] != "employee-1" {
		t.Fatalf("text base payload=%#v", text)
	}
	if content, ok := text["text"].(map[string]any); !ok || content["content"] != "hello" {
		t.Fatalf("text mapping=%#v", text["text"])
	}
	if _, present := text["attachments"]; present {
		t.Fatal("text-only payload must not carry an attachments array")
	}

	image := contactMessageBatchSendWeComRequest(ContactMessageBatchSendMessagePayload{Sender: "employee-1", ExternalUserID: []string{"external-1"}, Content: []ContactMessageBatchSendContent{{MsgType: "image", MediaID: "media-1"}}})
	attachments, ok := image["attachments"].([]map[string]any)
	if !ok || len(attachments) != 1 || attachments[0]["msgtype"] != "image" {
		t.Fatalf("image attachments=%#v", image["attachments"])
	}
	if inner, ok := attachments[0]["image"].(map[string]any); !ok || inner["media_id"] != "media-1" {
		t.Fatalf("image mapping=%#v", attachments[0]["image"])
	}

	link := contactMessageBatchSendWeComRequest(ContactMessageBatchSendMessagePayload{Sender: "employee-1", ExternalUserID: []string{"external-1"}, Content: []ContactMessageBatchSendContent{{MsgType: "link", Title: "标题", URL: "https://example.test", Desc: "摘要"}}})
	attachments, ok = link["attachments"].([]map[string]any)
	if !ok || len(attachments) != 1 || attachments[0]["msgtype"] != "link" {
		t.Fatalf("link attachments=%#v", link["attachments"])
	}
	if inner, ok := attachments[0]["link"].(map[string]any); !ok || inner["title"] != "标题" || inner["url"] != "https://example.test" || inner["desc"] != "摘要" {
		t.Fatalf("link mapping=%#v", attachments[0]["link"])
	}

	miniprogram := contactMessageBatchSendWeComRequest(ContactMessageBatchSendMessagePayload{Sender: "employee-1", ExternalUserID: []string{"external-1"}, Content: []ContactMessageBatchSendContent{{MsgType: "miniprogram", Title: "小程序", PicMediaID: "cover-1", AppID: "app-1", Page: "pages/index"}}})
	attachments, ok = miniprogram["attachments"].([]map[string]any)
	if !ok || len(attachments) != 1 || attachments[0]["msgtype"] != "miniprogram" {
		t.Fatalf("miniprogram attachments=%#v", miniprogram["attachments"])
	}
	if inner, ok := attachments[0]["miniprogram"].(map[string]any); !ok || inner["title"] != "小程序" || inner["pic_media_id"] != "cover-1" || inner["appid"] != "app-1" || inner["page"] != "pages/index" {
		t.Fatalf("miniprogram mapping=%#v", attachments[0]["miniprogram"])
	}

	empty := contactMessageBatchSendWeComRequest(ContactMessageBatchSendMessagePayload{Sender: "employee-1", ExternalUserID: []string{"external-1"}, Content: []ContactMessageBatchSendContent{{MsgType: "unsupported", Content: "x"}}})
	if _, present := empty["text"]; present {
		t.Fatal("unsupported msgType must not map into a text field")
	}
	if _, present := empty["attachments"]; present {
		t.Fatal("unsupported msgType must not map into attachments")
	}
}
