package inventory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanRoutesSplitsMultipleMethods(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "api-server/app/demo/src/Action/Test.php", `<?php
/**
 * @RequestMapping(path="/dashboard/demo", methods="get,post")
 */
class Test {}
`)
	writeFile(t, root, "api-server/plugin/demo/placeholder.php", `<?php`)

	routes, err := ScanRoutes(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 2 {
		t.Fatalf("len(routes) = %d, want 2", len(routes))
	}
	if routes[0].Method != "GET" || routes[1].Method != "POST" {
		t.Fatalf("methods = %#v", routes)
	}
}

func TestScanRoutesIncludesManualRoutes(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "api-server/app/demo/placeholder.php", `<?php`)
	writeFile(t, root, "api-server/plugin/demo/placeholder.php", `<?php`)
	writeFile(t, root, "api-server/config/routes.php", `<?php
Router::addRoute(['GET', 'POST', 'HEAD'], '/', function () {
    return 'Hello MoChat ';
});
Router::get('/favicon.ico', function () {
    return '';
});
`)

	routes, err := ScanRoutes(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 4 {
		t.Fatalf("len(routes) = %d, want 4: %#v", len(routes), routes)
	}
}

func TestScanTables(t *testing.T) {
	root := t.TempDir()
	sqlPath := filepath.Join(root, "mochat.sql")
	if err := os.WriteFile(sqlPath, []byte("CREATE TABLE IF NOT EXISTS `mc_user` (\n);\nCREATE TABLE IF NOT EXISTS `mc_corp` (\n);\n"), 0644); err != nil {
		t.Fatal(err)
	}

	tables, err := ScanTables(sqlPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) != 2 {
		t.Fatalf("len(tables) = %d, want 2", len(tables))
	}
	if tables[0].Name != "mc_corp" || tables[1].Name != "mc_user" {
		t.Fatalf("tables sorted unexpectedly: %#v", tables)
	}
}

func TestScanAsyncAnnotations(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "api-server/app/demo/src/Task/Demo.php", `<?php
/**
 * @Crontab(name="demo", rule="*/5 * * * *", callback="execute", memo="测试")
 * @WeChatEventHandler(eventPath="event/change_contact/create_user")
 * @AsyncQueueMessage(pool="contact")
 */
class Demo {}

// @AsyncQueueMessage(pool="ignored")
`)
	writeFile(t, root, "api-server/plugin/demo/placeholder.php", `<?php`)

	crontabs, err := ScanCrontabs(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(crontabs) != 1 || crontabs[0].Name != "demo" || crontabs[0].Memo != "测试" {
		t.Fatalf("crontabs = %#v", crontabs)
	}

	events, err := ScanEventHandlers(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].EventPath != "event/change_contact/create_user" {
		t.Fatalf("events = %#v", events)
	}

	queues, err := ScanAsyncQueues(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(queues) != 1 || queues[0].Pool != "contact" {
		t.Fatalf("queues = %#v", queues)
	}
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
