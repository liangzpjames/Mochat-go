package archivesim

import (
	"strings"
	"testing"
	"time"
)

func TestSimulationBlueprintCoversConversationDirectionsAndDisplayTypes(t *testing.T) {
	messages := buildMessages("acceptance", time.Unix(1000, 0), "employee-a", "employee-b", "contact", "room")
	if len(messages) != 13 {
		t.Fatalf("messages=%d", len(messages))
	}
	for _, required := range []string{"text", "image", "file", "voice", "video", "location", "card", "link", "emotion"} {
		found := false
		for _, message := range messages {
			if message.MsgType == required {
				found = true
			}
			if !strings.HasPrefix(message.MsgID, "MOCHAT-SIM:acceptance:") {
				t.Fatalf("unsafe msgid=%s", message.MsgID)
			}
		}
		if !found {
			t.Fatalf("missing type %s", required)
		}
	}
	if messages[1].From != "contact" || messages[9].ToList[0] != "employee-b" || messages[10].RoomID != "room" || messages[12].Action != "revoke" {
		t.Fatalf("conversation matrix incomplete: %+v", messages)
	}
}

func TestSimulationRejectsUnsafeBatchKeys(t *testing.T) {
	for _, input := range []string{"", "../real", "batch with spaces", strings.Repeat("x", 81)} {
		if _, err := validate(3, input); err == nil {
			t.Fatalf("accepted %q", input)
		}
	}
	if _, err := validate(3, "acceptance_20260813"); err != nil {
		t.Fatal(err)
	}
}

func TestSimulationEntityLabelsAreDedicatedAndTraceable(t *testing.T) {
	labels := simulationEntityLabels("ai_insight_accept_20260824")
	if labels.EmployeeA != "AI验收员工A-ai_insight_accept_20260824" || labels.EmployeeB != "AI验收员工B-ai_insight_accept_20260824" {
		t.Fatalf("employee labels=%+v", labels)
	}
	if labels.Contact != "AI验收客户-ai_insight_accept_20260824" {
		t.Fatalf("contact label=%q", labels.Contact)
	}
	if labels.Room != "AI验收客户群-ai_insight_accept_20260824" {
		t.Fatalf("room label=%q", labels.Room)
	}
}
