package migration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"jiyi/mochat-go/internal/migration"
)

func TestAIInsight0165LockIsScopedPerSelectedSchema(t *testing.T) {
	firstDatabase, root := newExternalMigrationIntegrationDBThrough(t, "0164_saas_tenant_ai_provider", "ai-0165-lock-schema-a")
	secondDatabase, _ := newExternalMigrationIntegrationDBThrough(t, "0164_saas_tenant_ai_provider", "ai-0165-lock-schema-b")
	firstController, err := migration.NewAIInsight0165Controller(firstDatabase, root)
	if err != nil {
		t.Fatal(err)
	}
	sameSchemaController, err := migration.NewAIInsight0165Controller(firstDatabase, root)
	if err != nil {
		t.Fatal(err)
	}
	secondController, err := migration.NewAIInsight0165Controller(secondDatabase, root)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	releaseFirst, err := migration.AcquireAIInsight0165LockForTest(ctx, firstController)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseFirst()
	if release, err := migration.AcquireAIInsight0165LockForTest(ctx, sameSchemaController); !errors.Is(err, migration.ErrAIInsight0165ConcurrentRun) {
		if release != nil {
			release()
		}
		t.Fatalf("same schema concurrent lock error = %v, want %v", err, migration.ErrAIInsight0165ConcurrentRun)
	}
	if _, err := secondController.Backup(ctx, "schema-b-backup"); err != nil {
		t.Fatalf("second schema did not progress beyond lock acquisition: %v", err)
	}
}
