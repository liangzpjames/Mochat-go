package identitysecurity

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"jiyi/mochat-go/internal/saasbackup"
)

type secretEnvelope struct {
	Version    int    `json:"v"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

func parseIdentityKeys(single, ring, activeID string) (map[string][]byte, string, error) {
	keys, err := saasbackup.ParseEncryptionKeyRing(ring)
	if err != nil {
		return nil, "", fmt.Errorf("identity MFA encryption key ring: %w", err)
	}
	activeID = strings.TrimSpace(activeID)
	if activeID == "" {
		activeID = "primary"
	}
	legacy, err := saasbackup.ParseEncryptionKey(single)
	if err != nil {
		return nil, "", fmt.Errorf("identity MFA encryption key: %w", err)
	}
	if len(legacy) > 0 {
		if existing, ok := keys[activeID]; ok && !hmac.Equal(existing, legacy) {
			return nil, "", fmt.Errorf("identity MFA encryption key %q differs between single-key and key-ring configuration", activeID)
		}
		keys[activeID] = legacy
	}
	if len(keys) > 0 {
		if _, ok := keys[activeID]; !ok {
			return nil, "", fmt.Errorf("identity MFA active encryption key %q is missing from the key ring", activeID)
		}
	}
	return keys, activeID, nil
}

func deriveIdentityKey(master []byte, purpose string) []byte {
	hash := hmac.New(sha256.New, master)
	_, _ = hash.Write([]byte("mochat-go/identity-security/v1/" + purpose))
	return hash.Sum(nil)
}

func encryptIdentitySecret(master []byte, keyID string, userID int, plaintext string) (string, error) {
	block, err := aes.NewCipher(deriveIdentityKey(master, "mfa-secret"))
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	aad := []byte(keyID + "\x00" + strconv.Itoa(userID))
	ciphertext := aead.Seal(nil, nonce, []byte(plaintext), aad)
	envelope, err := json.Marshal(secretEnvelope{
		Version: 1, Nonce: base64.RawURLEncoding.EncodeToString(nonce), Ciphertext: base64.RawURLEncoding.EncodeToString(ciphertext),
	})
	if err != nil {
		return "", err
	}
	return string(envelope), nil
}

func decryptIdentitySecret(master []byte, keyID string, userID int, encoded string) (string, error) {
	var envelope secretEnvelope
	if err := json.Unmarshal([]byte(encoded), &envelope); err != nil || envelope.Version != 1 {
		return "", Invalid("MFA 密钥工件格式无效")
	}
	nonce, err := base64.RawURLEncoding.DecodeString(envelope.Nonce)
	if err != nil {
		return "", Invalid("MFA 密钥工件 nonce 无效")
	}
	ciphertext, err := base64.RawURLEncoding.DecodeString(envelope.Ciphertext)
	if err != nil {
		return "", Invalid("MFA 密钥工件密文无效")
	}
	block, err := aes.NewCipher(deriveIdentityKey(master, "mfa-secret"))
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(nonce) != aead.NonceSize() {
		return "", Invalid("MFA 密钥工件 nonce 长度无效")
	}
	aad := []byte(keyID + "\x00" + strconv.Itoa(userID))
	plaintext, err := aead.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return "", Invalid("MFA 密钥工件认证失败")
	}
	return string(plaintext), nil
}

func randomToken(bytes int) (string, error) {
	if bytes <= 0 {
		bytes = 32
	}
	buffer := make([]byte, bytes)
	if _, err := io.ReadFull(rand.Reader, buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func sha256Hex(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func identityHMAC(master []byte, purpose string, userID int, value string) string {
	hash := hmac.New(sha256.New, deriveIdentityKey(master, purpose))
	_, _ = hash.Write([]byte(strconv.Itoa(userID)))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(value))
	return hex.EncodeToString(hash.Sum(nil))
}

func normalizeRecoveryCode(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	code = strings.ReplaceAll(code, "-", "")
	code = strings.ReplaceAll(code, " ", "")
	return code
}

func generateRecoveryCodes(master []byte, userID int, count int) ([]string, []string, error) {
	if count <= 0 {
		count = 8
	}
	codes := make([]string, 0, count)
	hashes := make([]string, 0, count)
	for len(codes) < count {
		raw, err := randomToken(6)
		if err != nil {
			return nil, nil, err
		}
		normalized := strings.ToUpper(strings.NewReplacer("-", "", "_", "").Replace(raw))
		if len(normalized) < 8 {
			continue
		}
		normalized = normalized[:8]
		code := normalized[:4] + "-" + normalized[4:]
		digest := identityHMAC(master, "recovery-code", userID, normalized)
		duplicate := false
		for _, existing := range hashes {
			if hmac.Equal([]byte(existing), []byte(digest)) {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		codes = append(codes, code)
		hashes = append(hashes, digest)
	}
	return codes, hashes, nil
}
