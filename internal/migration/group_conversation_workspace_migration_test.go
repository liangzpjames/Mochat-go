package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGroupConversationWorkspace0143MigrationContract(t *testing.T) {
	root := filepath.Join("..", "..")
	upBody, err := os.ReadFile(filepath.Join(root, "deploy", "standalone", "migrations", "0143_group_conversation_workspace.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	downBody, err := os.ReadFile(filepath.Join(root, "deploy", "standalone", "migrations", "0143_group_conversation_workspace.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	up := strings.ToLower(string(upBody))
	down := strings.Join(strings.Fields(strings.ToLower(string(downBody))), " ")
	for _, required := range []string{
		"create table if not exists `mochat_go_work_message_participant_identity`",
		"unique key `uk_mg_wmpi_corp_msg_seq`",
		"key `idx_mg_wmpi_corp_sender`",
		"key `idx_mg_wmpi_corp_room`",
		"dashboard.chat.v2_group",
		"/dashboard/workmessage/roomdirectory",
		"/dashboard/workmessage/roomprofile",
		"/dashboard/workmessage/roommessages",
		"/dashboard/workmessage/roommembers",
		"/dashboard/workmessage/roomfilteroptions",
		"where not exists",
	} {
		if !strings.Contains(up, required) {
			t.Fatalf("0143 up migration missing %q", required)
		}
	}
	for _, required := range []string{
		"delete resource from",
		"/dashboard/workmessage/roomdirectory",
		"/dashboard/workmessage/roomprofile",
		"/dashboard/workmessage/roommessages",
		"/dashboard/workmessage/roommembers",
		"/dashboard/workmessage/roomfilteroptions",
		"drop table if exists `mochat_go_work_message_participant_identity`",
	} {
		if !strings.Contains(down, required) {
			t.Fatalf("0143 down migration missing %q", required)
		}
	}
}
