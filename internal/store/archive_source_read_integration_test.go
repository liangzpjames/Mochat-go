package store

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"jiyi/mochat-go/internal/dashboard"
)

// The fixture rows use to_user_type=1. Keep every read filter explicit so the
// WorkMessageFilter zero value cannot silently narrow the result to type 0.
const archiveReadFixtureToUserType = 1

func TestArchiveSourceReadUsesRegistryForItemsCountsAndPages(t *testing.T) {
	db := newDashboardAdminProvisioningDB(t)
	createArchiveSyncCorpFixture(t, db)
	executeArchiveMigrationFile(t, db, "0138_archive_source_sync.up.sql")
	defer executeArchiveMigrationFile(t, db, "0138_archive_source_sync.down.sql")
	executeArchiveMigrationFile(t, db, "0133_archive_simulation_registry.up.sql")
	defer executeArchiveMigrationFile(t, db, "0133_archive_simulation_registry.down.sql")
	createArchiveReadFixture(t, db)
	seedArchiveReadMessages(t, db)

	store := NewMySQLStore(db)
	ctx := context.Background()
	if _, err := db.Exec(`UPDATE mc_corp SET chat_status=0 WHERE id=27`); err != nil {
		t.Fatal(err)
	}
	assertArchiveReadModeDiagnostics(t, NewMySQLStore(db), workMessageArchiveSimulation)
	assertArchiveReadSourceDiagnostics(t, db, NewMySQLStore(db), "simulated", []string{"registry-simulated"}, 1)
	simulated, err := store.WorkMessagePage(ctx, dashboard.WorkMessageFilter{
		CorpID: 27, WorkEmployeeID: 1001, ToUserType: archiveReadFixtureToUserType, ArchiveSource: "simulated", Page: 1, PerPage: 10,
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
	assertArchiveReadModeDiagnostics(t, store, workMessageArchiveReal)
	assertArchiveReadSourceDiagnostics(t, db, store, "external", []string{"MOCHAT-SIM:external-prefix", "historical-real"}, 2)

	external, err := store.WorkMessagePage(ctx, dashboard.WorkMessageFilter{
		CorpID: 27, WorkEmployeeID: 1001, ToUserType: archiveReadFixtureToUserType, ArchiveSource: "external", Page: 1, PerPage: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if external.Total != 2 || external.TotalPage != 2 || len(external.Items) != 1 {
		t.Fatalf("external page=%#v", external)
	}
	externalSecond, err := store.WorkMessagePage(ctx, dashboard.WorkMessageFilter{
		CorpID: 27, WorkEmployeeID: 1001, ToUserType: archiveReadFixtureToUserType, ArchiveSource: "external", Page: 2, PerPage: 1,
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
		CorpID: 27, WorkEmployeeID: 1001, ToUserType: archiveReadFixtureToUserType, Page: 1, PerPage: 10,
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
		CorpID: 27, AllowAllEmployees: true, ToUserType: archiveReadFixtureToUserType, ArchiveSource: "simulated", Page: 1, PerPage: 10,
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
		CorpID: 27, WorkEmployeeID: 1001, ToUserType: archiveReadFixtureToUserType, Page: 1, PerPage: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if defaultSimulation.Total != 1 || defaultSimulation.TotalPage != 1 || len(defaultSimulation.Items) != 1 || defaultSimulation.Items[0].ArchiveSource != "simulated" {
		t.Fatalf("default simulation page=%#v", defaultSimulation)
	}
	defaultSimulationConversations, err := store.WorkMessageToUsers(ctx, dashboard.WorkMessageUserFilter{
		CorpID: 27, AllowAllEmployees: true, ToUserType: archiveReadFixtureToUserType, Page: 1, PerPage: 10,
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
	assertArchiveReadModeDiagnostics(t, store, workMessageArchiveReal)
	realPage, err := store.WorkMessagePage(ctx, dashboard.WorkMessageFilter{CorpID: 27, WorkEmployeeID: 1001, ToUserType: archiveReadFixtureToUserType, Page: 1, PerPage: 10})
	if err != nil {
		t.Fatal(err)
	}
	if realPage.Total != 1 || realPage.TotalPage != 1 || len(realPage.Items) != 1 || realPage.Items[0].MsgID != "legacy-real" || realPage.Items[0].ArchiveSource != "external" {
		t.Fatalf("legacy default real page=%#v", realPage)
	}
	if _, err := db.Exec(`UPDATE mc_corp SET chat_status=0 WHERE id=27`); err != nil {
		t.Fatal(err)
	}
	assertArchiveReadModeDiagnostics(t, store, workMessageArchiveSimulation)
	assertArchiveReadSourceDiagnostics(t, db, store, "simulated", []string{"legacy-simulated"}, 1)
	simulationPage, err := store.WorkMessagePage(ctx, dashboard.WorkMessageFilter{CorpID: 27, WorkEmployeeID: 1001, ToUserType: archiveReadFixtureToUserType, Page: 1, PerPage: 10})
	if err != nil {
		t.Fatal(err)
	}
	if simulationPage.Total != 1 || simulationPage.TotalPage != 1 || len(simulationPage.Items) != 1 || simulationPage.Items[0].MsgID != "legacy-simulated" || simulationPage.Items[0].ArchiveSource != "simulated" || simulationPage.Items[0].ArchiveSourceID != "simulation:legacy-old" {
		t.Fatalf("legacy default simulation page=%#v", simulationPage)
	}
}

func assertArchiveReadModeDiagnostics(t *testing.T, store *MySQLStore, want workMessageArchiveMode) {
	t.Helper()
	mode, err := store.workMessageArchiveMode(context.Background(), 0, 27)
	if err != nil {
		t.Fatalf("archive mode diagnostic want=%v: %v", want, err)
	}
	effective, sourceOK := effectiveArchiveSource(mode, "")
	predicate, available := workMessageArchivePredicate(mode)
	state, stateErr := store.archiveSourceRegistryState(context.Background())
	t.Logf("archive mode diagnostic mode=%v want=%v effective=%q sourceOK=%v available=%v predicate=%q registryState=%#v stateErr=%v", mode, want, effective, sourceOK, available, predicate, state, stateErr)
	if mode != want || !sourceOK || !available || predicate != "1 = 1" {
		t.Fatalf("archive mode diagnostic mismatch mode=%v want=%v effective=%q sourceOK=%v available=%v predicate=%q registryState=%#v stateErr=%v", mode, want, effective, sourceOK, available, predicate, state, stateErr)
	}
}

func assertArchiveReadSourceDiagnostics(t *testing.T, db *sql.DB, store *MySQLStore, source string, expectedIDs []string, expected int) {
	t.Helper()
	ctx := context.Background()
	var shardRows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mc_work_message_1 WHERE corp_id=27 AND work_employee_id=1001`).Scan(&shardRows); err != nil {
		t.Fatalf("diagnostic shard count source=%s: %v", source, err)
	}
	var registryRows int
	registryErr := db.QueryRow(`
		SELECT COUNT(*)
		FROM mochat_go_archive_message_sources source_row
		INNER JOIN mc_corp corp ON corp.id=source_row.corp_id AND corp.tenant_id=source_row.tenant_id
		WHERE source_row.tenant_id=11 AND source_row.corp_id=27 AND source_row.source_kind=?
	`, source).Scan(&registryRows)
	var legacyRows int
	legacyErr := db.QueryRow(`
		SELECT COUNT(*)
		FROM mochat_go_archive_simulation_messages message_row
		INNER JOIN mochat_go_archive_simulation_batches batch_row
			ON batch_row.id=message_row.batch_id AND batch_row.corp_id=message_row.corp_id AND batch_row.status='complete'
		WHERE message_row.corp_id=27
	`).Scan(&legacyRows)
	state, err := store.archiveSourceRegistryState(ctx)
	if err != nil {
		t.Fatalf("diagnostic registry state source=%s: %v", source, err)
	}
	unionSQL, unionArgs, ok := workMessageUnionSQLWithArchiveSourceState(27, source, state)
	if !ok {
		t.Fatalf("diagnostic source union rejected source=%s state=%#v", source, state)
	}
	var unionRows, filteredRows int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ("+unionSQL+") diagnostic_union", unionArgs...).Scan(&unionRows); err != nil {
		t.Fatalf("diagnostic union count source=%s state=%#v args=%#v err=%v sql=%s", source, state, unionArgs, err, unionSQL)
	}
	filteredArgs := append(append([]any{}, unionArgs...), 1001)
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ("+unionSQL+") diagnostic_union WHERE diagnostic_union.work_employee_id=?", filteredArgs...).Scan(&filteredRows); err != nil {
		t.Fatalf("diagnostic filtered union source=%s state=%#v args=%#v err=%v sql=%s", source, state, filteredArgs, err, unionSQL)
	}
	actualIDs := make([]string, 0, expected)
	rows, err := db.QueryContext(ctx, "SELECT diagnostic_union.msgid FROM ("+unionSQL+") diagnostic_union WHERE diagnostic_union.work_employee_id=? ORDER BY diagnostic_union.msgid", filteredArgs...)
	if err != nil {
		t.Fatalf("diagnostic union ids source=%s state=%#v args=%#v err=%v sql=%s", source, state, filteredArgs, err, unionSQL)
	}
	for rows.Next() {
		var msgID string
		if err := rows.Scan(&msgID); err != nil {
			rows.Close()
			t.Fatalf("diagnostic union id scan source=%s: %v", source, err)
		}
		actualIDs = append(actualIDs, msgID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		t.Fatalf("diagnostic union id rows source=%s: %v", source, err)
	}
	rows.Close()
	t.Logf("archive read diagnostic source=%s state=%#v shardRows=%d registryRows=%d registryErr=%v legacyRows=%d legacyErr=%v unionRows=%d filteredRows=%d actualIDs=%v expectedIDs=%v args=%#v sql=%s", source, state, shardRows, registryRows, registryErr, legacyRows, legacyErr, unionRows, filteredRows, actualIDs, expectedIDs, unionArgs, unionSQL)
	if state.explicit {
		if registryErr != nil || registryRows != 1 {
			t.Fatalf("archive read registry diagnostic source=%s state=%#v registryRows=%d registryErr=%v", source, state, registryRows, registryErr)
		}
	} else if state.legacySimulation {
		if legacyErr != nil || legacyRows != expected {
			t.Fatalf("archive read legacy diagnostic source=%s state=%#v legacyRows=%d legacyErr=%v", source, state, legacyRows, legacyErr)
		}
	}
	actualSet := make(map[string]bool, len(actualIDs))
	for _, msgID := range actualIDs {
		actualSet[msgID] = true
	}
	if len(actualSet) != len(expectedIDs) {
		t.Fatalf("archive read diagnostic source=%s actualIDs=%v expectedIDs=%v", source, actualIDs, expectedIDs)
	}
	for _, msgID := range expectedIDs {
		if !actualSet[msgID] {
			t.Fatalf("archive read diagnostic source=%s actualIDs=%v missing=%q expectedIDs=%v", source, actualIDs, msgID, expectedIDs)
		}
	}
	if shardRows < expected || filteredRows != expected || unionRows < expected {
		t.Fatalf("archive read diagnostic source=%s state=%#v shardRows=%d registryRows=%d registryErr=%v legacyRows=%d legacyErr=%v unionRows=%d filteredRows=%d actualIDs=%v expectedIDs=%v args=%#v", source, state, shardRows, registryRows, registryErr, legacyRows, legacyErr, unionRows, filteredRows, actualIDs, expectedIDs, unionArgs)
	}
}

func createArchiveReadFixture(t *testing.T, db *sql.DB) {
	t.Helper()
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
