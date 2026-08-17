package wecomarchivedemo

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGenerateConfigProducesWeComCompatibleSecretsAndProtectedFiles(t *testing.T) {
	dir := t.TempDir()
	config, err := GenerateConfig(dir, "http://139.196.34.133:19090")
	if err != nil {
		t.Fatal(err)
	}
	if len(config.CallbackToken) != 32 || len(config.EncodingAESKey) != 43 || len(config.AdminToken) < 40 {
		t.Fatalf("generated lengths token=%d aes=%d admin=%d", len(config.CallbackToken), len(config.EncodingAESKey), len(config.AdminToken))
	}
	if !strings.Contains(config.RSAPublicKey, "BEGIN PUBLIC KEY") || !strings.Contains(config.RSAPrivateKey, "BEGIN RSA PRIVATE KEY") {
		t.Fatal("generated RSA key pair is invalid")
	}
	for _, name := range []string{"config.json", "private_key.pem", "admin-token.txt", "wecom-fill.txt"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
			t.Fatalf("%s mode = %o, want no group/other permissions", name, info.Mode().Perm())
		}
	}
	fill, err := os.ReadFile(filepath.Join(dir, "wecom-fill.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"http://139.196.34.133:19090/wecom/callback", config.CallbackToken, config.EncodingAESKey, config.RSAPublicKey} {
		if !strings.Contains(string(fill), want) {
			t.Fatalf("fill list does not contain %q", want)
		}
	}
}

func TestValidateServeConfigRejectsMissingOrWeakSecurityFields(t *testing.T) {
	valid, err := GenerateConfig(t.TempDir(), "http://139.196.34.133:19090")
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateServeConfig(valid); err != nil {
		t.Fatalf("generated config rejected: %v", err)
	}
	for name, mutate := range map[string]func(*Config){
		"admin token":    func(config *Config) { config.AdminToken = "" },
		"callback token": func(config *Config) { config.CallbackToken = "" },
		"AES key":        func(config *Config) { config.EncodingAESKey = "short" },
		"private key":    func(config *Config) { config.RSAPrivateKey = "" },
		"public address": func(config *Config) { config.PublicAddr = "" },
		"admin address":  func(config *Config) { config.AdminAddr = "" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := valid
			mutate(&candidate)
			if err := ValidateServeConfig(candidate); err == nil {
				t.Fatal("invalid config was accepted")
			}
		})
	}
}
