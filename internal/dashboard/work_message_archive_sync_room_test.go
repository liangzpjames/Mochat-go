package dashboard

import (
	"encoding/json"
	"testing"
)

func TestParseWorkMessageArchiveBridgeMessagePreservesRoomIdentity(t *testing.T) {
	message, err := parseWorkMessageArchiveBridgeMessage(json.RawMessage(`{
		"seq": 88,
		"msgid": "msg-room-1",
		"from": "wmKh001",
		"roomid": "wrRoom001",
		"tolist": ["wmKh002"],
		"msgtype": "text",
		"text": {"content": "群消息"}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if message.From != "wmKh001" || message.RoomID != "wrRoom001" || len(message.ToList) != 1 || message.ToList[0] != "wmKh002" {
		t.Fatalf("message=%#v", message)
	}
}
