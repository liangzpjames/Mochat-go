package archivefixture

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"io"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"jiyi/mochat-go/internal/wecomarchivedemo"
)

const suiteCallbackPaddingBlockSize = 32

type ProtocolError struct{ Code string }

func (e *ProtocolError) Error() string {
	if e == nil || strings.TrimSpace(e.Code) == "" {
		return "archive fixture protocol error"
	}
	return "archive fixture protocol error: " + strings.TrimSpace(e.Code)
}

func ErrorCode(err error) string {
	var protocolErr *ProtocolError
	if errors.As(err, &protocolErr) {
		return strings.TrimSpace(protocolErr.Code)
	}
	return ""
}

func protocolError(code string) error { return &ProtocolError{Code: code} }

type AccessToken struct {
	Value     string
	ExpiresAt time.Time
}

type SuiteAuthorization struct {
	TenantID      int
	CorpID        string
	PermanentCode string
}

type suitePreAuth struct {
	expiresAt time.Time
	used      bool
}

type suiteAuthCode struct {
	tenantID  int
	corpID    string
	expiresAt time.Time
	used      bool
}

type suiteAuthorizationState struct {
	SuiteAuthorization
	revoked bool
}

type SuiteProvider struct {
	mu             sync.Mutex
	suiteID        string
	suiteSecret    string
	callbackToken  string
	encodingAESKey string
	now            func() time.Time
	latestTicket   string
	suiteTokens    map[string]time.Time
	preAuthCodes   map[string]*suitePreAuth
	authCodes      map[string]*suiteAuthCode
	authorizations map[string]*suiteAuthorizationState
}

func NewSuiteProvider(suiteID, suiteSecret, callbackToken, encodingAESKey string) (*SuiteProvider, error) {
	suiteID, suiteSecret = strings.TrimSpace(suiteID), strings.TrimSpace(suiteSecret)
	callbackToken, encodingAESKey = strings.TrimSpace(callbackToken), strings.TrimSpace(encodingAESKey)
	if suiteID == "" || suiteSecret == "" || callbackToken == "" || len(encodingAESKey) != 43 {
		return nil, errors.New("suite fixture configuration is invalid")
	}
	if _, err := base64.StdEncoding.DecodeString(encodingAESKey + "="); err != nil {
		return nil, errors.New("suite fixture configuration is invalid")
	}
	return &SuiteProvider{
		suiteID: suiteID, suiteSecret: suiteSecret, callbackToken: callbackToken, encodingAESKey: encodingAESKey,
		now: time.Now, suiteTokens: map[string]time.Time{}, preAuthCodes: map[string]*suitePreAuth{},
		authCodes: map[string]*suiteAuthCode{}, authorizations: map[string]*suiteAuthorizationState{},
	}, nil
}

func (p *SuiteProvider) WithClock(now func() time.Time) *SuiteProvider {
	if p != nil && now != nil {
		p.mu.Lock()
		p.now = now
		p.mu.Unlock()
	}
	return p
}

func (p *SuiteProvider) LatestTicketConfigured() bool {
	if p == nil {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.latestTicket != ""
}

func (p *SuiteProvider) BuildTicketCallback(ticket, timestamp, nonce string) (url.Values, string, error) {
	if p == nil || strings.TrimSpace(ticket) == "" || strings.TrimSpace(timestamp) == "" || strings.TrimSpace(nonce) == "" {
		return nil, "", protocolError("SUITE_TICKET_INVALID")
	}
	message, err := xml.Marshal(struct {
		XMLName     xml.Name `xml:"xml"`
		SuiteID     string   `xml:"SuiteId"`
		InfoType    string   `xml:"InfoType"`
		SuiteTicket string   `xml:"SuiteTicket"`
	}{SuiteID: p.suiteID, InfoType: "suite_ticket", SuiteTicket: strings.TrimSpace(ticket)})
	if err != nil {
		return nil, "", protocolError("SUITE_TICKET_INVALID")
	}
	encrypted, err := encryptSuiteCallback(p.encodingAESKey, message, p.suiteID)
	if err != nil {
		return nil, "", protocolError("SUITE_CALLBACK_ENCRYPT_FAILED")
	}
	values := url.Values{"timestamp": {timestamp}, "nonce": {nonce}}
	values.Set("msg_signature", suiteCallbackSignature(p.callbackToken, timestamp, nonce, encrypted))
	return values, encrypted, nil
}

func (p *SuiteProvider) ReceiveTicket(values url.Values, encrypted string) error {
	if p == nil {
		return protocolError("SUITE_TICKET_INVALID")
	}
	plain, err := wecomarchivedemo.VerifyAndDecryptCallback(p.callbackToken, p.encodingAESKey, p.suiteID, values, encrypted)
	if err != nil {
		return protocolError("SUITE_CALLBACK_INVALID")
	}
	var event struct {
		SuiteID     string `xml:"SuiteId"`
		InfoType    string `xml:"InfoType"`
		SuiteTicket string `xml:"SuiteTicket"`
	}
	if xml.Unmarshal(plain.Message, &event) != nil || event.SuiteID != p.suiteID || event.InfoType != "suite_ticket" || strings.TrimSpace(event.SuiteTicket) == "" {
		return protocolError("SUITE_TICKET_INVALID")
	}
	p.mu.Lock()
	p.latestTicket = strings.TrimSpace(event.SuiteTicket)
	p.mu.Unlock()
	return nil
}

func (p *SuiteProvider) ExchangeSuiteToken(suiteID, suiteSecret, ticket string) (AccessToken, error) {
	if p == nil {
		return AccessToken{}, protocolError("SUITE_TICKET_INVALID")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if strings.TrimSpace(suiteID) != p.suiteID || strings.TrimSpace(suiteSecret) != p.suiteSecret || strings.TrimSpace(ticket) == "" || strings.TrimSpace(ticket) != p.latestTicket {
		return AccessToken{}, protocolError("SUITE_TICKET_INVALID")
	}
	value, err := randomFixtureToken("suite")
	if err != nil {
		return AccessToken{}, protocolError("SUITE_TOKEN_UNAVAILABLE")
	}
	expiresAt := p.now().UTC().Add(2 * time.Hour)
	p.suiteTokens[value] = expiresAt
	return AccessToken{Value: value, ExpiresAt: expiresAt}, nil
}

func (p *SuiteProvider) CreatePreAuthCode(suiteToken string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.validateSuiteTokenLocked(suiteToken); err != nil {
		return "", err
	}
	value, err := randomFixtureToken("preauth")
	if err != nil {
		return "", protocolError("PRE_AUTH_UNAVAILABLE")
	}
	p.preAuthCodes[value] = &suitePreAuth{expiresAt: p.now().UTC().Add(20 * time.Minute)}
	return value, nil
}

func (p *SuiteProvider) Authorize(preAuthCode string, tenantID int, corpID string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	state, ok := p.preAuthCodes[strings.TrimSpace(preAuthCode)]
	corpID = strings.TrimSpace(corpID)
	if !ok || state.used || !p.now().UTC().Before(state.expiresAt) || tenantID <= 0 || corpID == "" {
		return "", protocolError("PRE_AUTH_INVALID")
	}
	state.used = true
	value, err := randomFixtureToken("auth")
	if err != nil {
		return "", protocolError("AUTH_CODE_UNAVAILABLE")
	}
	p.authCodes[value] = &suiteAuthCode{tenantID: tenantID, corpID: corpID, expiresAt: p.now().UTC().Add(10 * time.Minute)}
	return value, nil
}

func (p *SuiteProvider) ExchangePermanentCode(suiteToken, authCode string) (SuiteAuthorization, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.validateSuiteTokenLocked(suiteToken); err != nil {
		return SuiteAuthorization{}, err
	}
	state, ok := p.authCodes[strings.TrimSpace(authCode)]
	if !ok || !p.now().UTC().Before(state.expiresAt) {
		return SuiteAuthorization{}, protocolError("AUTH_CODE_INVALID")
	}
	if state.used {
		return SuiteAuthorization{}, protocolError("AUTH_CODE_CONSUMED")
	}
	state.used = true
	permanentCode, err := randomFixtureToken("permanent")
	if err != nil {
		return SuiteAuthorization{}, protocolError("PERMANENT_CODE_UNAVAILABLE")
	}
	authorization := SuiteAuthorization{TenantID: state.tenantID, CorpID: state.corpID, PermanentCode: permanentCode}
	p.authorizations[suiteAuthorizationKey(state.tenantID, state.corpID)] = &suiteAuthorizationState{SuiteAuthorization: authorization}
	return authorization, nil
}

func (p *SuiteProvider) CorpToken(suiteToken string, tenantID int, corpID, permanentCode string) (AccessToken, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.validateSuiteTokenLocked(suiteToken); err != nil {
		return AccessToken{}, err
	}
	corpID = strings.TrimSpace(corpID)
	state, ok := p.authorizations[suiteAuthorizationKey(tenantID, corpID)]
	if !ok || state.PermanentCode != strings.TrimSpace(permanentCode) {
		return AccessToken{}, protocolError("CORP_SCOPE_MISMATCH")
	}
	if state.revoked {
		return AccessToken{}, protocolError("AUTHORIZATION_REVOKED")
	}
	value, err := randomFixtureToken("corp")
	if err != nil {
		return AccessToken{}, protocolError("CORP_TOKEN_UNAVAILABLE")
	}
	return AccessToken{Value: value, ExpiresAt: p.now().UTC().Add(2 * time.Hour)}, nil
}

func (p *SuiteProvider) RevokeAuthorization(tenantID int, corpID string) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if state := p.authorizations[suiteAuthorizationKey(tenantID, strings.TrimSpace(corpID))]; state != nil {
		state.revoked = true
	}
}

func (p *SuiteProvider) validateSuiteTokenLocked(token string) error {
	expiresAt, ok := p.suiteTokens[strings.TrimSpace(token)]
	if !ok {
		return protocolError("SUITE_TOKEN_INVALID")
	}
	if !p.now().UTC().Before(expiresAt) {
		return protocolError("SUITE_TOKEN_EXPIRED")
	}
	return nil
}

func suiteAuthorizationKey(tenantID int, corpID string) string {
	return strings.Join([]string{strconv.Itoa(tenantID), strings.TrimSpace(corpID)}, "\x00")
}

func randomFixtureToken(prefix string) (string, error) {
	raw := make([]byte, 24)
	if _, err := io.ReadFull(rand.Reader, raw); err != nil {
		return "", err
	}
	return prefix + "_" + base64.RawURLEncoding.EncodeToString(raw), nil
}

func encryptSuiteCallback(encodingAESKey string, message []byte, receiveID string) (string, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encodingAESKey) + "=")
	if err != nil || len(key) != 32 {
		return "", errors.New("invalid callback key")
	}
	randomPrefix := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, randomPrefix); err != nil {
		return "", err
	}
	plain := bytes.NewBuffer(randomPrefix)
	var size [4]byte
	binary.BigEndian.PutUint32(size[:], uint32(len(message)))
	plain.Write(size[:])
	plain.Write(message)
	plain.WriteString(receiveID)
	padding := suiteCallbackPaddingBlockSize - plain.Len()%suiteCallbackPaddingBlockSize
	padded := append(plain.Bytes(), bytes.Repeat([]byte{byte(padding)}, padding)...)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, key[:aes.BlockSize]).CryptBlocks(out, padded)
	return base64.StdEncoding.EncodeToString(out), nil
}

func suiteCallbackSignature(token, timestamp, nonce, encrypted string) string {
	items := []string{token, timestamp, nonce, encrypted}
	sort.Strings(items)
	sum := sha1.Sum([]byte(strings.Join(items, "")))
	return hex.EncodeToString(sum[:])
}
