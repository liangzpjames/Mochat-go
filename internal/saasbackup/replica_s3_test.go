package saasbackup

import (
	"strings"
	"testing"
	"time"
)

func TestNormalizeS3Endpoint(t *testing.T) {
	tests := []struct {
		input      string
		defaultTLS bool
		endpoint   string
		secure     bool
	}{
		{"https://s3.example.com", false, "s3.example.com", true},
		{"https://s3.example.com/", false, "s3.example.com", true},
		{"http://127.0.0.1:9000", true, "127.0.0.1:9000", false},
		{"s3.internal:9000", true, "s3.internal:9000", true},
	}
	for _, test := range tests {
		endpoint, secure, err := normalizeS3Endpoint(test.input, test.defaultTLS)
		if err != nil || endpoint != test.endpoint || secure != test.secure {
			t.Fatalf("normalize %q = %q/%v, %v", test.input, endpoint, secure, err)
		}
	}
	for _, value := range []string{"ftp://s3.example.com", "https://s3.example.com/path", "s3.example.com/path"} {
		if _, _, err := normalizeS3Endpoint(value, true); err == nil {
			t.Fatalf("expected invalid endpoint error for %q", value)
		}
	}
}

func TestS3ReplicaObjectKey(t *testing.T) {
	store, err := NewS3ReplicaStore(S3ReplicaConfig{
		Endpoint: "http://127.0.0.1:9000", Bucket: "mochat-backups", AccessKeyID: "access",
		SecretAccessKey: "secret", Prefix: "production/database",
	})
	if err != nil {
		t.Fatal(err)
	}
	key, err := store.ObjectKey("bkp_test.sql.gz.mgbk", time.Date(2026, 7, 11, 12, 0, 0, 0, time.FixedZone("CST", 8*3600)))
	if err != nil || key != "production/database/2026/07/11/bkp_test.sql.gz.mgbk" {
		t.Fatalf("object key = %q, %v", key, err)
	}
	if _, err := store.ObjectKey("../secret", time.Now()); err == nil {
		t.Fatal("expected unsafe artifact name error")
	}
	_, err = NewS3ReplicaStore(S3ReplicaConfig{
		Endpoint: "127.0.0.1:9000", Bucket: "mochat-backups", AccessKeyID: "access",
		SecretAccessKey: "secret", Prefix: "../unsafe",
	})
	if err == nil || !strings.Contains(err.Error(), "前缀") {
		t.Fatalf("invalid prefix error = %v", err)
	}
}
