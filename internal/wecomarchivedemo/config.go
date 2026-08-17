package wecomarchivedemo

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	CallbackToken  string `json:"callback_token"`
	EncodingAESKey string `json:"encoding_aes_key"`
	RSAPublicKey   string `json:"rsa_public_key"`
	RSAPrivateKey  string `json:"rsa_private_key"`
	AdminToken     string `json:"admin_token"`
	PublicURL      string `json:"public_url"`
	PublicAddr     string `json:"public_addr"`
	AdminAddr      string `json:"admin_addr"`
	DataDir        string `json:"data_dir"`
	PullLimit      uint32 `json:"pull_limit"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	CorpID         string `json:"corp_id,omitempty"`
	ArchiveSecret  string `json:"-"`
}

func GenerateConfig(dir, publicURL string) (Config, error) {
	if strings.TrimSpace(dir) == "" || strings.TrimSpace(publicURL) == "" {
		return Config{}, errors.New("output directory and public URL are required")
	}
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return Config{}, err
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return Config{}, err
	}
	publicPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}))
	privatePEM := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}))
	token, err := randomAlphaNumeric(32)
	if err != nil {
		return Config{}, err
	}
	aesKey, err := randomAlphaNumeric(43)
	if err != nil {
		return Config{}, err
	}
	adminBytes := make([]byte, 36)
	if _, err := rand.Read(adminBytes); err != nil {
		return Config{}, err
	}
	config := Config{
		CallbackToken: token, EncodingAESKey: aesKey,
		RSAPublicKey: publicPEM, RSAPrivateKey: privatePEM,
		AdminToken: base64.RawURLEncoding.EncodeToString(adminBytes),
		PublicURL:  strings.TrimRight(strings.TrimSpace(publicURL), "/"),
		PublicAddr: ":8080", AdminAddr: ":9091", DataDir: "/data",
		PullLimit: 100, TimeoutSeconds: 5,
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Config{}, err
	}
	_ = os.Chmod(dir, 0o700)
	if err := saveConfig(filepath.Join(dir, "config.json"), config); err != nil {
		return Config{}, err
	}
	if err := writeProtected(filepath.Join(dir, "private_key.pem"), []byte(privatePEM)); err != nil {
		return Config{}, err
	}
	if err := writeProtected(filepath.Join(dir, "admin-token.txt"), []byte(config.AdminToken+"\n")); err != nil {
		return Config{}, err
	}
	if err := os.WriteFile(filepath.Join(dir, "public_key.pem"), []byte(publicPEM), 0o644); err != nil {
		return Config{}, err
	}
	fill := fmt.Sprintf("企业微信回调配置\nURL: %s/wecom/callback\nToken: %s\nEncodingAESKey: %s\n\n会话内容存档 RSA 公钥：\n%s", config.PublicURL, config.CallbackToken, config.EncodingAESKey, config.RSAPublicKey)
	if err := os.WriteFile(filepath.Join(dir, "wecom-fill.txt"), []byte(fill), 0o644); err != nil {
		return Config{}, err
	}
	return config, nil
}

func LoadConfig(path string) (Config, error) {
	value, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var config Config
	if err := json.Unmarshal(value, &config); err != nil {
		return Config{}, err
	}
	config.ArchiveSecret = strings.TrimSpace(os.Getenv("WECOM_ARCHIVE_SECRET"))
	if value := strings.TrimSpace(os.Getenv("WECOM_ARCHIVE_CORP_ID")); value != "" {
		config.CorpID = value
	}
	if config.PullLimit == 0 || config.PullLimit > 1000 {
		config.PullLimit = 100
	}
	if config.TimeoutSeconds <= 0 {
		config.TimeoutSeconds = 5
	}
	return config, nil
}

func saveConfig(path string, config Config) error {
	value, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return writeProtected(path, append(value, '\n'))
}

func writeProtected(path string, value []byte) error {
	if err := os.WriteFile(path, value, 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

func randomAlphaNumeric(length int) (string, error) {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	value := make([]byte, length)
	random := make([]byte, length)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	for index := range value {
		value[index] = alphabet[int(random[index])%len(alphabet)]
	}
	return string(value), nil
}
