package saasbackup

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mysqldriver "github.com/go-sql-driver/mysql"
)

func TestEncryptedArtifactRoundTripAndTamperDetection(t *testing.T) {
	key := bytes.Repeat([]byte{0x42}, 32)
	plain := bytes.Repeat([]byte("INSERT INTO sample VALUES (1, 'backup');\n"), 5000)
	path := filepath.Join(t.TempDir(), "backup.sql.gz.mgbk")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := newChunkEncryptWriter(file, key)
	if err != nil {
		t.Fatal(err)
	}
	compressed := gzip.NewWriter(encrypted)
	if _, err := compressed.Write(plain); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	if err := encrypted.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	reader, err := openArtifactSQL(path, true, key)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := io.ReadAll(reader)
	if closeErr := reader.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(restored, plain) {
		t.Fatalf("restored SQL differs: got=%d want=%d", len(restored), len(plain))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data[len(data)-8] ^= 0xff
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	tampered, err := openArtifactSQL(path, true, key)
	if err == nil {
		_, err = io.Copy(io.Discard, tampered)
		_ = tampered.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "decrypt backup artifact chunk") {
		t.Fatalf("tampered artifact error = %v", err)
	}
}

func TestParseEncryptionKeyAndSafeArtifactPath(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	encoded := base64.StdEncoding.EncodeToString(key)
	parsed, err := ParseEncryptionKey(encoded)
	if err != nil || !bytes.Equal(parsed, key) {
		t.Fatalf("base64 key = %x, %v", parsed, err)
	}
	parsed, err = ParseEncryptionKey(strings.Repeat("07", 32))
	if err != nil || !bytes.Equal(parsed, key) {
		t.Fatalf("hex key = %x, %v", parsed, err)
	}
	if _, err := ParseEncryptionKey("short"); err == nil {
		t.Fatal("expected invalid key error")
	}
	root := t.TempDir()
	path, err := safeArtifactPath(root, "bkp_1.sql.gz.mgbk")
	if err != nil || filepath.Dir(path) != root {
		t.Fatalf("safe path = %q, %v", path, err)
	}
	for _, name := range []string{"../secret", "nested/file", `nested\\file`, ""} {
		if _, err := safeArtifactPath(root, name); err == nil {
			t.Fatalf("expected unsafe path error for %q", name)
		}
	}
}

func TestParseEncryptionKeyRing(t *testing.T) {
	keys, err := ParseEncryptionKeyRing(`{"2026-q2":"0707070707070707070707070707070707070707070707070707070707070707","2026-q3":"CAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAg="}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 || !bytes.Equal(keys["2026-q2"], bytes.Repeat([]byte{7}, 32)) || !bytes.Equal(keys["2026-q3"], bytes.Repeat([]byte{8}, 32)) {
		t.Fatalf("key ring = %#v", keys)
	}
	for _, value := range []string{`[]`, `{}`, `{"../bad":"0707070707070707070707070707070707070707070707070707070707070707"}`, `{"valid":"short"}`} {
		if _, err := ParseEncryptionKeyRing(value); err == nil {
			t.Fatalf("expected invalid key ring error for %s", value)
		}
	}
}

func TestRestoreTargetComparison(t *testing.T) {
	left, err := mysqldriver.ParseDSN("user:pass@tcp(localhost:3306)/mochat?parseTime=true")
	if err != nil {
		t.Fatal(err)
	}
	same, _ := mysqldriver.ParseDSN("other:secret@tcp(localhost:3306)/mochat?parseTime=true")
	other, _ := mysqldriver.ParseDSN("user:pass@tcp(localhost:3306)/mochat_restore_drill?parseTime=true")
	if !sameDatabaseTarget(left, same) {
		t.Fatal("same database target was not detected")
	}
	if sameDatabaseTarget(left, other) {
		t.Fatal("different database was treated as the source")
	}
}

func TestMySQLDefaultsFileMatchesDSNTLSMode(t *testing.T) {
	config, err := mysqldriver.ParseDSN(`backup-user:p@ss\"word@tcp(127.0.0.1:3306)/mochat?parseTime=true`)
	if err != nil {
		t.Fatal(err)
	}
	path, err := mysqlDefaultsFile(config)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("defaults file mode = %o", info.Mode().Perm())
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, expected := range []string{"[client]", `user="backup-user"`, `password="p@ss\\\"word"`, "ssl=0", "protocol=tcp", `host="127.0.0.1"`, "port=3306"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("defaults file missing %q: %s", expected, text)
		}
	}

	tlsConfig, err := mysqldriver.ParseDSN(`backup-user:secret@tcp(127.0.0.1:3306)/mochat?tls=true`)
	if err != nil {
		t.Fatal(err)
	}
	tlsPath, err := mysqlDefaultsFile(tlsConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tlsPath)
	tlsBody, err := os.ReadFile(tlsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(tlsBody), "ssl=1") {
		t.Fatalf("TLS defaults file did not enable SSL: %s", tlsBody)
	}
}
