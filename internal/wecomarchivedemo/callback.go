package wecomarchivedemo

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

const weComPKCS7BlockSize = 32

type CallbackPlaintext struct {
	Message   []byte
	ReceiveID string
}

func VerifyAndDecryptCallback(token, encodingAESKey, expectedReceiveID string, values url.Values, encrypted string) (CallbackPlaintext, error) {
	if strings.TrimSpace(token) == "" || strings.TrimSpace(encrypted) == "" {
		return CallbackPlaintext{}, errors.New("callback credentials or ciphertext are empty")
	}
	items := []string{token, values.Get("timestamp"), values.Get("nonce"), encrypted}
	sort.Strings(items)
	sum := sha1.Sum([]byte(strings.Join(items, "")))
	want := hex.EncodeToString(sum[:])
	got := strings.TrimSpace(values.Get("msg_signature"))
	if len(got) != len(want) || subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
		return CallbackPlaintext{}, errors.New("invalid callback signature")
	}
	key, err := decodeCallbackAESKey(encodingAESKey)
	if err != nil {
		return CallbackPlaintext{}, err
	}
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return CallbackPlaintext{}, fmt.Errorf("decode callback ciphertext: %w", err)
	}
	if len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return CallbackPlaintext{}, errors.New("invalid callback ciphertext length")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return CallbackPlaintext{}, fmt.Errorf("initialize callback cipher: %w", err)
	}
	plain := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, key[:aes.BlockSize]).CryptBlocks(plain, ciphertext)
	plain, err = unpadCallbackPKCS7(plain)
	if err != nil {
		return CallbackPlaintext{}, err
	}
	if len(plain) < 20 {
		return CallbackPlaintext{}, errors.New("invalid callback plaintext length")
	}
	messageLength := int(binary.BigEndian.Uint32(plain[16:20]))
	if messageLength < 0 || 20+messageLength > len(plain) {
		return CallbackPlaintext{}, errors.New("invalid callback message length")
	}
	result := CallbackPlaintext{
		Message:   append([]byte(nil), plain[20:20+messageLength]...),
		ReceiveID: string(plain[20+messageLength:]),
	}
	if expected := strings.TrimSpace(expectedReceiveID); expected != "" && result.ReceiveID != expected {
		return CallbackPlaintext{}, errors.New("callback receive id mismatch")
	}
	return result, nil
}

func decodeCallbackAESKey(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if len(value) != 43 {
		return nil, errors.New("EncodingAESKey must contain 43 characters")
	}
	decoded, err := base64.StdEncoding.DecodeString(value + "=")
	if err != nil || len(decoded) != 32 {
		return nil, errors.New("EncodingAESKey is not a valid 32-byte key")
	}
	return decoded, nil
}

func unpadCallbackPKCS7(value []byte) ([]byte, error) {
	if len(value) == 0 {
		return nil, errors.New("invalid callback padding")
	}
	padding := int(value[len(value)-1])
	if padding < 1 || padding > weComPKCS7BlockSize || padding > len(value) {
		return nil, errors.New("invalid callback padding")
	}
	for _, item := range value[len(value)-padding:] {
		if int(item) != padding {
			return nil, errors.New("invalid callback padding")
		}
	}
	return value[:len(value)-padding], nil
}
