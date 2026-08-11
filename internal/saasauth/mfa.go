package saasauth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

type MFAProtector struct {
	key   []byte
	keyID string
}

type MFAEnrollment struct {
	Token      string
	Secret     string
	OTPAuthURL string
	ExpiresAt  time.Time
}

type mfaSecretEnvelope struct {
	Version    int    `json:"v"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

func NewMFAProtector(key []byte, keyID string) (*MFAProtector, error) {
	if len(key) != 32 || strings.TrimSpace(keyID) == "" {
		return nil, fmt.Errorf("SaaS MFA encryption key must be 32 bytes and key id must be non-empty")
	}
	copyKey := append([]byte(nil), key...)
	return &MFAProtector{key: copyKey, keyID: strings.TrimSpace(keyID)}, nil
}

func (protector *MFAProtector) GenerateEnrollment(identity SaaSIdentity, now time.Time) (MFAEnrollment, string, error) {
	if protector == nil || identity.ID <= 0 {
		return MFAEnrollment{}, "", ErrMFAInvalid
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	account := strings.TrimSpace(identity.LoginName)
	if account == "" {
		account = "saas-user-" + strconv.Itoa(identity.ID)
	}
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "MoChat SaaS",
		AccountName: account,
		Period:      30,
		SecretSize:  20,
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
	})
	if err != nil {
		return MFAEnrollment{}, "", err
	}
	ciphertext, err := protector.encrypt(identity.ID, key.Secret())
	if err != nil {
		return MFAEnrollment{}, "", err
	}
	return MFAEnrollment{Secret: key.Secret(), OTPAuthURL: key.URL(), ExpiresAt: now.Add(10 * time.Minute)}, ciphertext, nil
}

func (protector *MFAProtector) Verify(secretCiphertext, keyID string, userID int, code string, now time.Time) (int64, error) {
	if protector == nil || strings.TrimSpace(keyID) != protector.keyID || userID <= 0 {
		return 0, ErrMFAChallengeInvalid
	}
	secret, err := protector.decrypt(userID, secretCiphertext)
	if err != nil {
		return 0, ErrMFAChallengeInvalid
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return 0, ErrMFAChallengeInvalid
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	for _, offset := range []int64{-1, 0, 1} {
		candidateTime := now.Add(time.Duration(offset) * 30 * time.Second)
		valid, validateErr := totp.ValidateCustom(code, secret, candidateTime, totp.ValidateOpts{
			Period: 30, Skew: 0, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1,
		})
		if validateErr == nil && valid {
			return candidateTime.Unix() / 30, nil
		}
	}
	return 0, ErrMFAChallengeInvalid
}

func (protector *MFAProtector) encrypt(userID int, secret string) (string, error) {
	block, err := aes.NewCipher(protector.key)
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
	envelope := mfaSecretEnvelope{
		Version:    1,
		Nonce:      base64.RawURLEncoding.EncodeToString(nonce),
		Ciphertext: base64.RawURLEncoding.EncodeToString(aead.Seal(nil, nonce, []byte(secret), protector.aad(userID))),
	}
	encoded, err := json.Marshal(envelope)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func (protector *MFAProtector) decrypt(userID int, encoded string) (string, error) {
	var envelope mfaSecretEnvelope
	if err := json.Unmarshal([]byte(encoded), &envelope); err != nil || envelope.Version != 1 {
		return "", errors.New("invalid MFA secret envelope")
	}
	nonce, err := base64.RawURLEncoding.DecodeString(envelope.Nonce)
	if err != nil {
		return "", err
	}
	ciphertext, err := base64.RawURLEncoding.DecodeString(envelope.Ciphertext)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(protector.key)
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil || len(nonce) != aead.NonceSize() {
		return "", errors.New("invalid MFA secret nonce")
	}
	plaintext, err := aead.Open(nil, nonce, ciphertext, protector.aad(userID))
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func (protector *MFAProtector) aad(userID int) []byte {
	digest := sha256.Sum256([]byte("mochat-go/saas-admin-mfa/v1\x00" + protector.keyID + "\x00" + strconv.Itoa(userID)))
	return digest[:]
}
