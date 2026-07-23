package saasauditanchor

import "testing"

func TestNormalizeRemoteS3Endpoint(t *testing.T) {
	tests := []struct {
		input      string
		defaultTLS bool
		endpoint   string
		secure     bool
	}{
		{"minio.internal:9000", true, "minio.internal:9000", true},
		{"http://127.0.0.1:9000", true, "127.0.0.1:9000", false},
		{"https://s3.example.cn", false, "s3.example.cn", true},
	}
	for _, test := range tests {
		endpoint, secure, err := normalizeRemoteS3Endpoint(test.input, test.defaultTLS)
		if err != nil || endpoint != test.endpoint || secure != test.secure {
			t.Fatalf("normalize %q = %q/%v/%v", test.input, endpoint, secure, err)
		}
	}
	for _, value := range []string{"ftp://s3.example.cn", "https://s3.example.cn/path", "host/path"} {
		if _, _, err := normalizeRemoteS3Endpoint(value, true); err == nil {
			t.Fatalf("expected invalid endpoint %q", value)
		}
	}
}

func TestNewS3RemoteArtifactStoreConfiguration(t *testing.T) {
	store, err := NewS3RemoteArtifactStore(S3RemoteArtifactConfig{
		Endpoint: "127.0.0.1:9000", Bucket: "audit-lock", AccessKeyID: "access",
		SecretAccessKey: "secret", UseSSL: false, Prefix: "company/audit",
		RetentionMode: "governance", RetentionDays: 730,
	})
	if err != nil {
		t.Fatal(err)
	}
	if store.Provider() != "s3-object-lock" || store.Bucket() != "audit-lock" || store.Prefix() != "company/audit" ||
		store.RetentionMode() != "governance" || store.RetentionDays() != 730 {
		t.Fatalf("unexpected remote store configuration: %s %s %s %s %d", store.Provider(), store.Bucket(), store.Prefix(), store.RetentionMode(), store.RetentionDays())
	}
	for _, config := range []S3RemoteArtifactConfig{
		{Endpoint: "127.0.0.1:9000", Bucket: "audit-lock", AccessKeyID: "access", SecretAccessKey: "secret", Prefix: "../audit"},
		{Endpoint: "127.0.0.1:9000", Bucket: "audit-lock", AccessKeyID: "access", SecretAccessKey: "secret", RetentionMode: "invalid"},
		{Endpoint: "127.0.0.1:9000", Bucket: "audit-lock", AccessKeyID: "access", SecretAccessKey: "secret", RetentionDays: 36501},
	} {
		if _, err := NewS3RemoteArtifactStore(config); err == nil {
			t.Fatalf("expected invalid remote store config: %+v", config)
		}
	}
}
