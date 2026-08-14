package wecom

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"

	"jiyi/mochat-go/internal/modules/providers"
	archiveprovider "jiyi/mochat-go/internal/modules/providers/archive"
)

// Config carries the enterprise WeChat conversation archive credentials.
type Config struct {
	CorpID     string
	Secret     string
	PublicKey  string
	PrivateKey string
}

// Archive is the registered adapter for WeCom conversation archives.
type Archive struct {
	corpID     string
	secret     string
	publicKey  string
	privateKey string
}

var _ providers.ArchiveProvider = (*Archive)(nil)
var _ archiveprovider.ArchiveSource = (*Archive)(nil)

func New(config Config) (*Archive, error) {
	return &Archive{
		corpID:     strings.TrimSpace(config.CorpID),
		secret:     strings.TrimSpace(config.Secret),
		publicKey:  strings.TrimSpace(config.PublicKey),
		privateKey: strings.TrimSpace(config.PrivateKey),
	}, nil
}

func (a *Archive) Status() providers.Status {
	missing := make([]string, 0, 4)
	if a.corpID == "" {
		missing = append(missing, "MOCHAT_GO_WECOM_ARCHIVE_CORP_ID")
	}
	if a.secret == "" {
		missing = append(missing, "MOCHAT_GO_WECOM_ARCHIVE_SECRET")
	}
	if a.publicKey == "" {
		missing = append(missing, "MOCHAT_GO_WECOM_ARCHIVE_PUBLIC_KEY")
	}
	if a.privateKey == "" {
		missing = append(missing, "MOCHAT_GO_WECOM_ARCHIVE_PRIVATE_KEY")
	}
	if len(missing) > 0 {
		return providers.Status{
			Kind:          "wecom_archive",
			State:         providers.StateLimited,
			Code:          "archive.credentials_missing",
			Source:        providers.SourceExternal,
			Reason:        "企业微信会话存档凭据未完整配置",
			Action:        "在企业设置中配置会话存档凭据",
			Missing:       missing,
			LastErrorCode: "archive.credentials_missing",
		}
	}
	return providers.Status{
		Kind:   "wecom_archive",
		State:  providers.StateLimited,
		Code:   "archive.getchatdata_unimplemented",
		Source: providers.SourceExternal,
		Reason: "真实会话存档 getchatdata source 尚未实现",
		Action: "接入并验证真实会话存档 source 后再启用同步",
	}
}

func (a *Archive) Sync(ctx context.Context, _ providers.SyncOptions) (providers.SyncResult, error) {
	status := a.Status()
	if status.Code == "archive.credentials_missing" {
		return providers.SyncResult{}, providers.ErrNotConfigured
	}
	if ctx == nil {
		return providers.SyncResult{}, errors.New("context is required")
	}
	// The adapter intentionally fails closed until a real getchatdata source is
	// injected. Complete credentials alone are not evidence of a usable source.
	return providers.SyncResult{}, providers.ErrCapabilityUnavailable
}

func (a *Archive) Kind() providers.Source { return providers.SourceExternal }

func (a *Archive) SourceID() string {
	if a == nil || strings.TrimSpace(a.corpID) == "" {
		return "wecom"
	}
	return "wecom:" + a.corpID
}

func (a *Archive) Namespace() string { return "wecom" }

func (a *Archive) Fetch(ctx context.Context, scope archiveprovider.Scope, _ archiveprovider.Cursor, _ int) (archiveprovider.Page, error) {
	if ctx == nil {
		return archiveprovider.Page{}, errors.New("context is required")
	}
	if scope.TenantID <= 0 || scope.CorpID <= 0 {
		return archiveprovider.Page{}, archiveprovider.ErrInvalidScope
	}
	status := a.Status()
	if status.Code == "archive.credentials_missing" {
		return archiveprovider.Page{}, providers.ErrNotConfigured
	}
	// The adapter is intentionally source-compatible but capability-incomplete:
	// credentials and RSA helpers are not evidence that getchatdata is usable.
	return archiveprovider.Page{}, providers.ErrCapabilityUnavailable
}

// Message is the normalized archive message shape stored into mc_work_message_*.
type Message struct {
	MsgID          string
	Seq            int64
	WorkEmployeeID int64
	ToUserType     int
	ToUserID       int64
	SenderType     int
	Type           int
	MsgType        int
	Content        any
	ContentText    string
	RoomID         int64
	MsgDataTime    time.Time
}

// EncryptAESCBC encrypts plaintext with AES-256-CBC and PKCS7 padding.
func EncryptAESCBC(key []byte, plaintext []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, errors.New("AES key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	padded := pkcs7Pad(plaintext, block.BlockSize())
	ciphertext := make([]byte, aes.BlockSize+len(padded))
	if _, err := rand.Read(ciphertext[:aes.BlockSize]); err != nil {
		return nil, err
	}
	cipher.NewCBCEncrypter(block, ciphertext[:aes.BlockSize]).CryptBlocks(ciphertext[aes.BlockSize:], padded)
	return ciphertext, nil
}

// DecryptAESCBC decrypts AES-256-CBC ciphertext and removes PKCS7 padding.
func DecryptAESCBC(key []byte, ciphertext []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, errors.New("AES key must be 32 bytes")
	}
	if len(ciphertext) < aes.BlockSize || len(ciphertext)%aes.BlockSize != 0 {
		return nil, errors.New("invalid AES ciphertext length")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	plaintext := make([]byte, len(ciphertext)-aes.BlockSize)
	cipher.NewCBCDecrypter(block, ciphertext[:aes.BlockSize]).CryptBlocks(plaintext, ciphertext[aes.BlockSize:])
	unpadded, err := pkcs7Unpad(plaintext, block.BlockSize())
	if err != nil {
		return nil, err
	}
	return unpadded, nil
}

// GenerateRSAKeyPair returns a PEM-encoded RSA-2048 key pair for tests.
func GenerateRSAKeyPair() (publicPEM string, privatePEM string, err error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", "", err
	}
	publicPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}))
	privatePEM = string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}))
	return publicPEM, privatePEM, nil
}

// RSADecrypt decrypts a base64 RSA-OAEP message with SHA-256.
func RSADecrypt(privatePEM string, encoded string) ([]byte, error) {
	block, _ := pem.Decode([]byte(privatePEM))
	if block == nil {
		return nil, errors.New("invalid RSA private key PEM")
	}
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	payload, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	return rsa.DecryptOAEP(sha256.New(), rand.Reader, privateKey, payload, nil)
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	return append(data, bytes.Repeat([]byte{byte(padding)}, padding)...)
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, errors.New("invalid padded data length")
	}
	padding := int(data[len(data)-1])
	if padding <= 0 || padding > blockSize || padding > len(data) {
		return nil, errors.New("invalid PKCS7 padding")
	}
	for _, value := range data[len(data)-padding:] {
		if int(value) != padding {
			return nil, errors.New("invalid PKCS7 padding bytes")
		}
	}
	return data[:len(data)-padding], nil
}

func (a *Archive) String() string {
	return fmt.Sprintf("wecom-archive(corp=%q, state=%s)", a.corpID, a.Status().State)
}
