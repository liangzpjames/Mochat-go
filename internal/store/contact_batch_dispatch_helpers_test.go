package store

import (
	"strings"
	"testing"
)

// TestContactBatchDispatchKeyIsStablePerEmployeeAndChunk locks the chunk
// idempotency identity: stable for identical (seed, employee, chunk, values)
// and distinct across employees, chunks and value sets.
func TestContactBatchDispatchKeyIsStablePerEmployeeAndChunk(t *testing.T) {
	values := []string{"external-a", "external-b"}
	first := contactBatchDispatchKey("seed-1", 101, 0, values)
	if contactBatchDispatchKey("seed-1", 101, 0, values) != first {
		t.Fatal("dispatch key must be stable for identical inputs")
	}
	if contactBatchDispatchKey("seed-1", 102, 0, values) == first {
		t.Fatal("dispatch key must differ across employees")
	}
	if contactBatchDispatchKey("seed-1", 101, 1, values) == first {
		t.Fatal("dispatch key must differ across chunks")
	}
	if contactBatchDispatchKey("seed-2", 101, 0, values) == first {
		t.Fatal("dispatch key must differ across seeds")
	}
	if contactBatchDispatchKey("seed-1", 101, 0, []string{"external-a"}) == first {
		t.Fatal("dispatch key must differ across value sets")
	}
	if !strings.HasPrefix(first, "contact-") || len(first) != len("contact-")+64 {
		t.Fatalf("dispatch key format=%q", first)
	}
}

// TestContactBatchDispatchTargetRoundTrip locks the target identity
// {batchID}:{employeeID}:{chunkNo} encoding used by sender payload rebuilds.
func TestContactBatchDispatchTargetRoundTrip(t *testing.T) {
	for _, target := range []struct {
		batch, employee, chunk int
	}{
		{701, 101, 0},
		{701, 101, 3},
		{2, 202, 11},
	} {
		encoded := contactBatchDispatchTargetString(target.batch, target.employee, target.chunk)
		decoded, ok := parseContactBatchDispatchTarget(encoded)
		if !ok || decoded.BatchID != target.batch || decoded.EmployeeID != target.employee || decoded.ChunkNo != target.chunk {
			t.Fatalf("round trip %d/%d/%d -> %q -> %#v", target.batch, target.employee, target.chunk, encoded, decoded)
		}
	}
	for _, malformed := range []string{"", "contact_batch:701:employee:101", "contact_batch:701:employee:101:chunk", "room_batch:701:employee:101:chunk:0", "contact_batch:0:employee:101:chunk:0", "contact_batch:701:employee:0:chunk:0", "contact_batch:701:employee:101:chunk:-1"} {
		if _, ok := parseContactBatchDispatchTarget(malformed); ok {
			t.Fatalf("malformed target %q accepted", malformed)
		}
	}
}

// TestContactBatchStringChunksSplitsAtBoundary locks chunk splitting so every
// target lands in exactly one dispatch chunk.
func TestContactBatchStringChunksSplitsAtBoundary(t *testing.T) {
	chunks := contactBatchStringChunks([]string{"a", "b", "c"}, 2)
	if len(chunks) != 2 || len(chunks[0]) != 2 || len(chunks[1]) != 1 || chunks[0][0] != "a" || chunks[0][1] != "b" || chunks[1][0] != "c" {
		t.Fatalf("chunks=%#v", chunks)
	}
	if got := contactBatchStringChunks(nil, 2); len(got) != 0 {
		t.Fatalf("empty chunks=%#v", got)
	}
	values := make([]string, contactBatchDispatchChunkSize+1)
	for i := range values {
		values[i] = "v"
	}
	big := contactBatchStringChunks(values, 0)
	if len(big) != 2 || len(big[0]) != contactBatchDispatchChunkSize || len(big[1]) != 1 {
		t.Fatalf("default-size chunks=%d/%d/%d", len(big), len(big[0]), len(big[1]))
	}
	total := 0
	for _, chunk := range big {
		total += len(chunk)
	}
	if total != len(values) {
		t.Fatalf("chunk total=%d want %d", total, len(values))
	}
}

// TestContactBatchRequestIDRoundTrip locks the business-row <-> operation
// request_id binding used by durable read projections and the legacy cron
// exclusion query.
func TestContactBatchRequestIDRoundTrip(t *testing.T) {
	if got := contactBatchRequestID(701); got != "contact-batch:701" {
		t.Fatalf("request id=%q", got)
	}
	id, ok := contactBatchIDFromRequestID("contact-batch:701")
	if !ok || id != 701 {
		t.Fatalf("parse id=%d ok=%v", id, ok)
	}
	for _, malformed := range []string{"", "contact-batch:0", "contact-batch:-1", "room-batch:701", "contact-batch:abc", "contact-batch:701:extra"} {
		if _, ok := contactBatchIDFromRequestID(malformed); ok {
			t.Fatalf("malformed request id %q accepted", malformed)
		}
	}
}