package archivesim

import (
	"context"
	"testing"
	"time"

	"jiyi/mochat-go/internal/modules/providers"
	archiveprovider "jiyi/mochat-go/internal/modules/providers/archive"
)

func TestSimulationBatchSourceRuntimeContract(t *testing.T) {
	batch := "runtime-contract"
	source := newSimulationBatchSource(batch, buildMessages(batch, time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC), "employee-a", "employee-b", "contact-a", "room-a"))
	if source.Kind() != providers.SourceSimulated || source.SourceID() != "simulation:"+batch || source.Namespace() != "MOCHAT-SIM:"+batch {
		t.Fatalf("source identity=%s,%s,%s", source.Kind(), source.SourceID(), source.Namespace())
	}
	page, err := source.Fetch(context.Background(), archiveprovider.Scope{TenantID: 11, CorpID: 27}, archiveprovider.Cursor{}, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Messages) != 13 {
		t.Fatalf("message count=%d, want 13", len(page.Messages))
	}
	for _, message := range page.Messages {
		if message.Source != providers.SourceSimulated || message.SourceID != source.SourceID() || message.Namespace != source.Namespace() {
			t.Fatalf("message identity=%#v", message)
		}
	}
}
