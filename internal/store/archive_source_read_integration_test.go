package store

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"jiyi/mochat-go/internal/dashboard"
)

func TestArchiveSourceReadUsesRegistryForItemsCountsAndPages(t *testing.T) {
	db := newDashboardAdminProvisioningDB(t)
	createArchiveSyncCorpFixture(t, db)
	executeArchiveMigrationFile(t, db, "0138_archive_source_sync.up.sql")
	defer executeArchiveMigrationFile(t, db, "0138_archive_source_sync.down.sql")
	createArchiveReadFixture(t, db)
	seedArchiveReadMessages(t, db)

	store := NewMySQLStore(db)
	ctx := context.Background()
	if _, err := db.Exec(`UPDATE mc_corp SET chat_status=0 WHERE id=27`); err != nil {
		t.Fatal(err)
	}
	simulated, err := store.WorkMessagePage(ctx, dashboard.WorkMessageFilter{
		CorpID: 27, WorkEmployeeID: 1001, ArchiveSource: "simulated", Page: 1, PerPage: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if simulated.Total != 1 || simulated.TotalPage != 1 || len(simulated.Items) != 1 {
		t.Fatalf("simulated page=%#v", simulated)
	}
	if simulated.Items[0].MsgID != "registry-simulated" || simulated.Items[0].ArchiveSource != "simulated" || simulated.Items[0].ArchiveSourceID != "simulation:read" {
		t.Fatalf("simulated item=%#v", simulated.Items[0])
	}
	if _, err := db.Exec(`UPDATE mc_corp SET chat_status=1 WHERE id=27`); err != nil {
		t.Fatal(err)
	}

	external, err := store.WorkMessagePage(ctx, dashboard.WorkMessageFilter{
		CorpID: 27, WorkEmployeeID: 1001, ArchiveSource: "external", Page: 1, PerPage: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if external.Total != 2 || external.TotalPage != 2 || len(external.Items) != 1 {
		t.Fatalf("external page=%#v", external)
	}
	externalSecond, err := store.WorkMessagePage(ctx, dashboard.WorkMessageFilter{
		CorpID: 27, WorkEmployeeID: 1001, ArchiveSource: "external", Page: 2, PerPage: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if externalSecond.Total != 2 || externalSecond.TotalPage != 2 || len(externalSecond.Items) != 1 {
		t.Fatalf("external second page=%#v", externalSecond)
	}
	externalIDs := map[string]bool{}
	for _, item := range append(external.Items, externalSecond.Items...) {
		if item.ArchiveSource != "external" || item.ArchiveSourceID != "wecom" && item.ArchiveSourceID != "wecom:read" {
			t.Fatalf("external item=%#v", item)
		}
		externalIDs[item.MsgID] = true
	}
	if len(externalIDs) != 2 || !externalIDs["MOCHAT-SIM:external-prefix"] || !externalIDs["historical-real"] {
		t.Fatalf("external ids=%v", externalIDs)
	}
	defaultExternal, err := store.WorkMessagePage(ctx, dashboard.WorkMessageFilter{
		CorpID: 27, WorkEmployeeID: 1001, Page: 1, PerPage: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if defaultExternal.Total != 2 || defaultExternal.TotalPage != 1 || len(defaultExternal.Items) != 2 {
		t.Fatalf("default real page=%#v", defaultExternal)
	}
	for _, item := range defaultExternal.Items {
		if item.ArchiveSource != "external" || item.MsgID == "registry-simulated" {
			t.Fatalf("default real item=%#v", item)
		}
	}
	detail, found, err := store.WorkMessageByArchiveID(ctx, dashboard.WorkMessageArchiveFilter{
		CorpID: 27, ArchiveMessageID: "msg:MOCHAT-SIM:external-prefix",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !found || detail.MsgID != "MOCHAT-SIM:external-prefix" || detail.ArchiveSource != "external" || detail.ArchiveSourceID != "wecom:read" {
		t.Fatalf("default real detail=%#v found=%v", detail, found)
	}

	if _, err := db.Exec(`UPDATE mc_corp SET chat_status=0 WHERE id=27`); err != nil {
		t.Fatal(err)
	}
	conversations, err := store.WorkMessageToUsers(ctx, dashboard.WorkMessageUserFilter{
		CorpID: 27, AllowAllEmployees: true, ToUserType: -1, ArchiveSource: "simulated", Page: 1, PerPage: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if conversations.Total != 1 || conversations.TotalPage != 1 || len(conversations.Items) != 1 || conversations.Items[0].ArchiveSource != "simulated" || conversations.Items[0].ArchiveSourceID != "simulation:read" {
		t.Fatalf("simulated conversations=%#v", conversations)
	}
	if _, err := db.Exec(`UPDATE mc_corp SET chat_status=0 WHERE id=27`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mochat_go_archive_simulation_batches (corp_id,batch_key,status,message_count) VALUES (27,'read-default','complete',1)`); err != nil {
		t.Fatal(err)
	}
	defaultSimulation, err := store.WorkMessagePage(ctx, dashboard.WorkMessageFilter{
		CorpID: 27, WorkEmployeeID: 1001, Page: 1, PerPage: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if defaultSimulation.Total != 1 || defaultSimulation.TotalPage != 1 || len(defaultSimulation.Items) != 1 || defaultSimulation.Items[0].ArchiveSource != "simulated" {
		t.Fatalf("default simulation page=%#v", defaultSimulation)
	}
	defaultSimulationConversations, err := store.WorkMessageToUsers(ctx, dashboard.WorkMessageUserFilter{
		CorpID: 27, AllowAllEmployees: true, ToUserType: -1, Page: 1, PerPage: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if defaultSimulationConversations.Total != 1 || defaultSimulationConversations.TotalPage != 1 || len(defaultSimulationConversations.Items) != 1 || defaultSimulationConversations.Items[0].ArchiveSource != "simulated" {
		t.Fatalf("default simulation conversations=%#v", defaultSimulationConversations)
	}
}

func TestArchiveSourceReadLegacySimulationRegistryStaysOutOfExternalDefault(t *testing.T) {
	db := newDashboardAdminProvisioningDB(t)
	createArchiveSyncCorpFixture(t, db)
	executeArchiveMigrationFile(t, db, "0133_archive_simulation_registry.up.sql")
	defer executeArchiveMigrationFile(t, db, "0133_archive_simulation_registry.down.sql")
	createArchiveReadBusinessFixture(t, db)

	batch, err := db.Exec(`INSERT INTO mochat_go_archive_simulation_batches (corp_id,batch_key,status,message_count) VALUES (27,'legacy-old','complete',1)`)
	if err != nil {
		t.Fatal(err)
	}
	batchID, err := batch.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mc_work_message_1 (corp_id,msgid,seq,work_employee_id,to_user_type,to_user_id,content,content_text,msg_data_time) VALUES (27,'legacy-simulated',11,1001,1,2001,'{}','legacy-simulated',NOW())`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mochat_go_archive_simulation_messages (batch_id,corp_id,msgid,table_index) VALUES (?,?,?,1)`, batchID, 27, "legacy-simulated"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mc_work_message_1 (corp_id,msgid,seq,work_employee_id,to_user_type,to_user_id,content,content_text,msg_data_time) VALUES (27,'legacy-real',12,1001,1,2001,'{}','legacy-real',NOW())`); err != nil {
		t.Fatal(err)
	}

	store := NewMySQLStore(db)
	ctx := context.Background()
	realPage, err := store.WorkMessagePage(ctx, dashboard.WorkMessageFilter{CorpID: 27, WorkEmployeeID: 1001, Page: 1, PerPage: 10})
	if err != nil {
		t.Fatal(err)
	}
	if realPage.Total != 1 || realPage.TotalPage != 1 || len(realPage.Items) != 1 || realPage.Items[0].MsgID != "legacy-real" || realPage.Items[0].ArchiveSource != "external" {
		t.Fatalf("legacy default real page=%#v", realPage)
	}
	if _, err := db.Exec(`UPDATE mc_corp SET chat_status=0 WHERE id=27`); err != nil {
		t.Fatal(err)
	}
	simulationPage, err := store.WorkMessagePage(ctx, dashboard.WorkMessageFilter{CorpID: 27, WorkEmployeeID: 1001, Page: 1, PerPage: 10})
	if err != nil {
		t.Fatal(err)
	}
	if simulationPage.Total != 1 || simulationPage.TotalPage != 1 || len(simulationPage.Items) != 1 || simulationPage.Items[0].MsgID != "legacy-simulated" || simulationPage.Items[0].ArchiveSource != "simulated" || simulationPage.Items[0].ArchiveSourceID != "simulation:legacy-old" {
		t.Fatalf("legacy default simulation page=%#v", simulationPage)
	}
}

func createArchiveReadFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`CREATE TABLE mochat_go_archive_simulation_batches (id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY, corp_id INT UNSIGNED NOT NULL, batch_key VARCHAR(128) NOT NULL, status VARCHAR(16) NOT NULL, message_count INT UNSIGNED NOT NULL DEFAULT 0, created_at DATETIME NULL, updated_at DATETIME NULL) ENGINE=InnoDB`); err != nil {
		t.Fatal(err)
	}
	createArchiveReadBusinessFixture(t, db)
}

func createArchiveReadBusinessFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, statement := range []string{
		`CREATE TABLE mc_work_employee (id INT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY, corp_id INT UNSIGNED NOT NULL, name VARCHAR(255) NOT NULL DEFAULT '', alias VARCHAR(255) NOT NULL DEFAULT '', avatar VARCHAR(255) NOT NULL DEFAULT '', wx_user_id VARCHAR(255) NOT NULL, deleted_at DATETIME NULL) ENGINE=InnoDB`,
		`CREATE TABLE mc_work_contact (id INT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY, corp_id INT UNSIGNED NOT NULL, name VARCHAR(255) NOT NULL DEFAULT '', alias VARCHAR(255) NOT NULL DEFAULT '', avatar VARCHAR(255) NOT NULL DEFAULT '', wx_external_userid VARCHAR(255) NOT NULL, deleted_at DATETIME NULL) ENGINE=InnoDB`,
		`CREATE TABLE mc_work_room (id INT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY, corp_id INT UNSIGNED NOT NULL, name VARCHAR(255) NOT NULL DEFAULT '', avatar VARCHAR(255) NOT NULL DEFAULT '', wx_chat_id VARCHAR(255) NOT NULL, deleted_at DATETIME NULL) ENGINE=InnoDB`,
		`INSERT INTO mc_work_employee (id, corp_id, name, wx_user_id) VALUES (1001, 27, 'Read employee', 'employee-read')`,
		`INSERT INTO mc_work_contact (id, corp_id, name, wx_external_userid) VALUES (2001, 27, 'Read contact', 'contact-read')`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	for index := 1; index <= 10; index++ {
		if _, err := db.Exec(fmt.Sprintf(`CREATE TABLE mc_work_message_%d (
			id INT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY, corp_id INT UNSIGNED NOT NULL, msgid VARCHAR(255) NOT NULL,
			seq BIGINT NOT NULL, work_employee_id INT NOT NULL, to_user_type INT NOT NULL, to_user_id INT NOT NULL,
			sender_type INT NOT NULL DEFAULT 0, action INT NOT NULL DEFAULT 0, type INT NOT NULL DEFAULT 1, msg_type INT NOT NULL DEFAULT 1,
			content TEXT NOT NULL, content_text TEXT NOT NULL, room_id INT NOT NULL DEFAULT 0, msg_data_time DATETIME NULL,
			deleted_at DATETIME NULL
		) ENGINE=InnoDB`, index)); err != nil {
			t.Fatal(err)
		}
	}
}

func seedArchiveReadMessages(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO mochat_go_archive_simulation_batches (corp_id,batch_key,status,message_count) VALUES (27,'read','complete',1)`); err != nil {
		t.Fatal(err)
	}
	result, err := db.Exec(`INSERT INTO mochat_go_archive_sync_runs (tenant_id,corp_id,source_kind,source_id,namespace,idempotency_key,status) VALUES (11,27,'simulated','simulation:read','MOCHAT-SIM:read','read-sim','succeeded')`)
	if err != nil {
		t.Fatal(err)
	}
	simRun, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	result, err = db.Exec(`INSERT INTO mochat_go_archive_sync_runs (tenant_id,corp_id,source_kind,source_id,namespace,idempotency_key,status) VALUES (11,27,'external','wecom:read','wecom:read','read-ext','succeeded')`)
	if err != nil {
		t.Fatal(err)
	}
	extRun, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		msgID, kind, sourceID, namespace string
		runID                            int64
		seq                              int
	}{
		{msgID: "registry-simulated", kind: "simulated", sourceID: "simulation:read", namespace: "MOCHAT-SIM:read", runID: simRun, seq: 1},
		{msgID: "MOCHAT-SIM:external-prefix", kind: "external", sourceID: "wecom:read", namespace: "wecom:read", runID: extRun, seq: 2},
	} {
		if _, err := db.Exec(`INSERT INTO mc_work_message_1 (corp_id,msgid,seq,work_employee_id,to_user_type,to_user_id,content,content_text,msg_data_time) VALUES (27,?,?,1001,1,2001,'{}',?,NOW())`, row.msgID, row.seq, row.msgID); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO mochat_go_archive_message_sources (tenant_id,corp_id,msgid,source_kind,source_id,namespace,run_id) VALUES (11,27,?,?,?,?,?)`, row.msgID, row.kind, row.sourceID, row.namespace, row.runID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO mc_work_message_1 (corp_id,msgid,seq,work_employee_id,to_user_type,to_user_id,content,content_text,msg_data_time) VALUES (27,'historical-real',3,1001,1,2001,'{}','historical-real',NOW())`); err != nil {
		t.Fatal(err)
	}
}
