package archivefixture

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"io"
	"sort"
	"strings"
	"sync"
	"time"
)

const dataZonePublicKeyVersion uint32 = 1

var dataZoneContentTypes = map[string]bool{
	"text": true, "image": true, "voice": true, "video": true, "file": true,
}

// DataZoneContent is content held inside the simulated data zone. Body is
// never returned by Fetch; it can only be obtained through Render with the
// per-message secret key.
type DataZoneContent struct {
	Sequence  int64
	MessageID string
	Type      string
	Sender    string
	Receivers []string
	RoomID    string
	Body      []byte
	FileName  string
	MIMEType  string
}

// DataZoneMessage mirrors the metadata allowed to leave the data zone.
type DataZoneMessage struct {
	Sequence           int64     `json:"seq"`
	MessageID          string    `json:"msgid"`
	Type               string    `json:"msgtype"`
	Sender             string    `json:"from"`
	Receivers          []string  `json:"tolist"`
	RoomID             string    `json:"roomid,omitempty"`
	SentAt             time.Time `json:"sentAt"`
	PublicKeyVersion   uint32    `json:"public_key_ver"`
	EncryptedSecretKey string    `json:"encrypted_secret_key"`
	FileName           string    `json:"fileName,omitempty"`
	MIMEType           string    `json:"mimeType,omitempty"`
}

type dataZoneEntry struct {
	message    DataZoneMessage
	ciphertext []byte
	expiresAt  time.Time
}

type DataZoneStateEntry struct {
	Message    DataZoneMessage `json:"message"`
	Ciphertext []byte          `json:"ciphertext"`
	ExpiresAt  time.Time       `json:"expiresAt"`
}

type DataZoneState struct {
	CorpID        string               `json:"corpId"`
	PrivateKeyPEM string               `json:"privateKeyPem"`
	Revoked       bool                 `json:"revoked"`
	Entries       []DataZoneStateEntry `json:"entries"`
}

type DataZoneProvider struct {
	mu         sync.RWMutex
	corpID     string
	privateKey *rsa.PrivateKey
	now        func() time.Time
	revoked    bool
	entries    map[string]dataZoneEntry
	contentTTL time.Duration
}

func NewDataZoneProvider(corpID string) (*DataZoneProvider, error) {
	corpID = strings.TrimSpace(corpID)
	if corpID == "" {
		return nil, errors.New("data zone fixture corp id is required")
	}
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, errors.New("data zone fixture key generation failed")
	}
	return &DataZoneProvider{corpID: corpID, privateKey: privateKey, now: time.Now, entries: map[string]dataZoneEntry{}, contentTTL: 5 * time.Minute}, nil
}

func NewDataZoneProviderFromState(state DataZoneState) (*DataZoneProvider, error) {
	state.CorpID = strings.TrimSpace(state.CorpID)
	block, _ := pem.Decode([]byte(state.PrivateKeyPEM))
	if state.CorpID == "" || block == nil {
		return nil, errors.New("data zone fixture state is invalid")
	}
	privateKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.New("data zone fixture state key is invalid")
	}
	rsaKey, ok := privateKey.(*rsa.PrivateKey)
	if !ok || rsaKey.Validate() != nil {
		return nil, errors.New("data zone fixture state key is invalid")
	}
	provider := &DataZoneProvider{corpID: state.CorpID, privateKey: rsaKey, now: time.Now, revoked: state.Revoked, entries: map[string]dataZoneEntry{}, contentTTL: 5 * time.Minute}
	for _, item := range state.Entries {
		if strings.TrimSpace(item.Message.MessageID) == "" || item.Message.PublicKeyVersion != dataZonePublicKeyVersion || len(item.Ciphertext) == 0 || item.ExpiresAt.IsZero() {
			return nil, errors.New("data zone fixture state entry is invalid")
		}
		provider.entries[item.Message.MessageID] = dataZoneEntry{message: item.Message, ciphertext: append([]byte(nil), item.Ciphertext...), expiresAt: item.ExpiresAt}
	}
	return provider, nil
}

func (p *DataZoneProvider) WithClock(now func() time.Time) *DataZoneProvider {
	if p != nil && now != nil {
		p.mu.Lock()
		p.now = now
		p.mu.Unlock()
	}
	return p
}

func (p *DataZoneProvider) WithContentTTL(ttl time.Duration) *DataZoneProvider {
	if p != nil && ttl > 0 {
		p.mu.Lock()
		p.contentTTL = ttl
		p.mu.Unlock()
	}
	return p
}

func (p *DataZoneProvider) ExportState() (DataZoneState, error) {
	if p == nil || p.privateKey == nil {
		return DataZoneState{}, errors.New("data zone fixture state is unavailable")
	}
	encoded, err := x509.MarshalPKCS8PrivateKey(p.privateKey)
	if err != nil {
		return DataZoneState{}, errors.New("data zone fixture state key is unavailable")
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	state := DataZoneState{CorpID: p.corpID, PrivateKeyPEM: string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded})), Revoked: p.revoked, Entries: make([]DataZoneStateEntry, 0, len(p.entries))}
	for _, entry := range p.entries {
		state.Entries = append(state.Entries, DataZoneStateEntry{Message: entry.message, Ciphertext: append([]byte(nil), entry.ciphertext...), ExpiresAt: entry.expiresAt})
	}
	sort.Slice(state.Entries, func(i, j int) bool { return state.Entries[i].Message.Sequence < state.Entries[j].Message.Sequence })
	return state, nil
}

func (p *DataZoneProvider) Append(content DataZoneContent) (DataZoneMessage, error) {
	if p == nil || p.privateKey == nil {
		return DataZoneMessage{}, protocolError("DATA_ZONE_UNAVAILABLE")
	}
	content.MessageID = strings.TrimSpace(content.MessageID)
	content.Type = strings.ToLower(strings.TrimSpace(content.Type))
	if content.Sequence <= 0 || content.MessageID == "" || !dataZoneContentTypes[content.Type] || len(content.Body) == 0 {
		return DataZoneMessage{}, protocolError("DATA_ZONE_CONTENT_INVALID")
	}
	secret := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, secret); err != nil {
		return DataZoneMessage{}, protocolError("DATA_ZONE_SECRET_UNAVAILABLE")
	}
	ciphertext, err := sealDataZoneContent(secret, p.corpID, content.MessageID, content.Body)
	if err != nil {
		return DataZoneMessage{}, protocolError("DATA_ZONE_CONTENT_INVALID")
	}
	wrapped, err := rsa.EncryptPKCS1v15(rand.Reader, &p.privateKey.PublicKey, secret)
	if err != nil {
		return DataZoneMessage{}, protocolError("DATA_ZONE_SECRET_UNAVAILABLE")
	}
	now := p.currentTime()
	message := DataZoneMessage{
		Sequence: content.Sequence, MessageID: content.MessageID, Type: content.Type,
		Sender: strings.TrimSpace(content.Sender), Receivers: append([]string(nil), content.Receivers...),
		RoomID: strings.TrimSpace(content.RoomID), SentAt: now.UTC(), PublicKeyVersion: dataZonePublicKeyVersion,
		EncryptedSecretKey: base64.StdEncoding.EncodeToString(wrapped), FileName: strings.TrimSpace(content.FileName), MIMEType: strings.TrimSpace(content.MIMEType),
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.revoked {
		return DataZoneMessage{}, protocolError("DATA_ZONE_AUTHORIZATION_REVOKED")
	}
	if _, exists := p.entries[message.MessageID]; exists {
		return DataZoneMessage{}, protocolError("DATA_ZONE_MESSAGE_EXISTS")
	}
	ttl := p.contentTTL
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	p.entries[message.MessageID] = dataZoneEntry{message: message, ciphertext: ciphertext, expiresAt: now.Add(ttl)}
	return message, nil
}

func (p *DataZoneProvider) Fetch(corpID string, after int64, limit int) ([]DataZoneMessage, error) {
	if p == nil || strings.TrimSpace(corpID) != p.corpID {
		return nil, protocolError("DATA_ZONE_SCOPE_MISMATCH")
	}
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.revoked {
		return nil, protocolError("DATA_ZONE_AUTHORIZATION_REVOKED")
	}
	items := make([]DataZoneMessage, 0, limit)
	for _, entry := range p.entries {
		if entry.message.Sequence > after {
			items = append(items, entry.message)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Sequence < items[j].Sequence })
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (p *DataZoneProvider) DecryptSecretKey(message DataZoneMessage) ([]byte, error) {
	if p == nil || p.privateKey == nil || message.PublicKeyVersion != dataZonePublicKeyVersion {
		return nil, protocolError("DATA_ZONE_SECRET_INVALID")
	}
	raw, err := base64.StdEncoding.DecodeString(message.EncryptedSecretKey)
	if err != nil {
		return nil, protocolError("DATA_ZONE_SECRET_INVALID")
	}
	secret, err := rsa.DecryptPKCS1v15(rand.Reader, p.privateKey, raw)
	if err != nil || len(secret) != 32 {
		return nil, protocolError("DATA_ZONE_SECRET_INVALID")
	}
	return secret, nil
}

func (p *DataZoneProvider) Render(corpID, messageID string, secret []byte) (DataZoneContent, error) {
	if p == nil || strings.TrimSpace(corpID) != p.corpID {
		return DataZoneContent{}, protocolError("DATA_ZONE_SCOPE_MISMATCH")
	}
	messageID = strings.TrimSpace(messageID)
	p.mu.RLock()
	entry, exists := p.entries[messageID]
	revoked := p.revoked
	p.mu.RUnlock()
	if revoked {
		return DataZoneContent{}, protocolError("DATA_ZONE_AUTHORIZATION_REVOKED")
	}
	if !exists {
		return DataZoneContent{}, protocolError("DATA_ZONE_CONTENT_UNAVAILABLE")
	}
	if !p.currentTime().Before(entry.expiresAt) {
		return DataZoneContent{}, protocolError("DATA_ZONE_CONTENT_EXPIRED")
	}
	body, err := openDataZoneContent(secret, p.corpID, messageID, entry.ciphertext)
	if err != nil {
		return DataZoneContent{}, protocolError("DATA_ZONE_SECRET_INVALID")
	}
	message := entry.message
	return DataZoneContent{
		Sequence: message.Sequence, MessageID: message.MessageID, Type: message.Type, Sender: message.Sender,
		Receivers: append([]string(nil), message.Receivers...), RoomID: message.RoomID, Body: body,
		FileName: message.FileName, MIMEType: message.MIMEType,
	}, nil
}

func (p *DataZoneProvider) Revoke() {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.revoked = true
	p.mu.Unlock()
}

func (p *DataZoneProvider) currentTime() time.Time {
	p.mu.RLock()
	now := p.now
	p.mu.RUnlock()
	if now == nil {
		return time.Now()
	}
	return now()
}

func sealDataZoneContent(secret []byte, corpID, messageID string, body []byte) ([]byte, error) {
	block, err := aes.NewCipher(secret)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return aead.Seal(nonce, nonce, body, []byte(strings.TrimSpace(corpID)+"\x00"+strings.TrimSpace(messageID))), nil
}

func openDataZoneContent(secret []byte, corpID, messageID string, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(secret)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil || len(ciphertext) < aead.NonceSize()+aead.Overhead() {
		return nil, errors.New("invalid data zone ciphertext")
	}
	return aead.Open(nil, ciphertext[:aead.NonceSize()], ciphertext[aead.NonceSize():], []byte(strings.TrimSpace(corpID)+"\x00"+strings.TrimSpace(messageID)))
}
