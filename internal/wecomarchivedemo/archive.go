package wecomarchivedemo

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ArchiveService struct {
	sdk            FinanceSDK
	privateKey     *rsa.PrivateKey
	store          *EvidenceStore
	limit          uint32
	timeoutSeconds int
	now            func() time.Time
	mu             sync.Mutex
}

type PullResult struct {
	StartSeq     uint64    `json:"start_seq"`
	NextSeq      uint64    `json:"next_seq"`
	MessageCount int       `json:"message_count"`
	PulledAt     time.Time `json:"pulled_at"`
}

type ArchivePage struct {
	StartSeq uint64            `json:"start_seq"`
	NextSeq  uint64            `json:"next_seq"`
	Messages []json.RawMessage `json:"messages"`
}

type getChatDataResponse struct {
	ErrCode  int                  `json:"errcode"`
	ErrMsg   string               `json:"errmsg"`
	ChatData []encryptedChatDatum `json:"chatdata"`
}

type encryptedChatDatum struct {
	Seq                uint64 `json:"seq"`
	MsgID              string `json:"msgid"`
	PublicKeyVersion   uint32 `json:"publickey_ver"`
	EncryptedRandomKey string `json:"encrypt_random_key"`
	EncryptedChatMsg   string `json:"encrypt_chat_msg"`
}

func NewArchiveService(sdk FinanceSDK, privatePEM string, store *EvidenceStore, limit uint32, timeoutSeconds int) (*ArchiveService, error) {
	if sdk == nil || store == nil {
		return nil, errors.New("finance SDK and evidence store are required")
	}
	key, err := parseRSAPrivateKey(privatePEM)
	if err != nil {
		return nil, err
	}
	if limit == 0 || limit > 1000 {
		return nil, errors.New("pull limit must be between 1 and 1000")
	}
	if timeoutSeconds <= 0 {
		return nil, errors.New("timeout must be positive")
	}
	return &ArchiveService{sdk: sdk, privateKey: key, store: store, limit: limit, timeoutSeconds: timeoutSeconds, now: time.Now}, nil
}

func (s *ArchiveService) FetchPage(ctx context.Context, startSeq uint64, limit uint32) (ArchivePage, error) {
	if ctx == nil {
		return ArchivePage{}, errors.New("context is required")
	}
	if err := ctx.Err(); err != nil {
		return ArchivePage{}, err
	}
	if limit == 0 || limit > 1000 {
		return ArchivePage{}, errors.New("limit must be between 1 and 1000")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	value, err := s.sdk.GetChatData(startSeq, limit, s.timeoutSeconds)
	if err != nil {
		return ArchivePage{}, err
	}
	var response getChatDataResponse
	if err := json.Unmarshal(value, &response); err != nil {
		return ArchivePage{}, fmt.Errorf("parse GetChatData response: %w", err)
	}
	if response.ErrCode != 0 {
		return ArchivePage{}, SDKError{Operation: "GetChatData", Code: response.ErrCode}
	}
	page := ArchivePage{StartSeq: startSeq, NextSeq: startSeq, Messages: make([]json.RawMessage, 0, len(response.ChatData))}
	for _, item := range response.ChatData {
		randomKey, err := decryptRandomKey(s.privateKey, item.EncryptedRandomKey)
		if err != nil {
			return ArchivePage{}, fmt.Errorf("decrypt random key for seq %d: %w", item.Seq, err)
		}
		plain, err := s.sdk.DecryptData(string(randomKey), item.EncryptedChatMsg)
		if err != nil {
			return ArchivePage{}, fmt.Errorf("decrypt chat message for seq %d: %w", item.Seq, err)
		}
		var message map[string]json.RawMessage
		if err := json.Unmarshal(plain, &message); err != nil {
			return ArchivePage{}, fmt.Errorf("parse chat message for seq %d: %w", item.Seq, err)
		}
		message["seq"] = json.RawMessage(strconv.FormatUint(item.Seq, 10))
		if rawMsgID, ok := message["msgid"]; !ok || strings.TrimSpace(string(rawMsgID)) == "" || string(rawMsgID) == `""` {
			encodedMsgID, err := json.Marshal(item.MsgID)
			if err != nil {
				return ArchivePage{}, err
			}
			message["msgid"] = encodedMsgID
		}
		raw, err := json.Marshal(message)
		if err != nil {
			return ArchivePage{}, fmt.Errorf("encode chat message for seq %d: %w", item.Seq, err)
		}
		page.Messages = append(page.Messages, raw)
		if item.Seq > page.NextSeq {
			page.NextSeq = item.Seq
		}
	}
	return page, nil
}

func (s *ArchiveService) Pull(ctx context.Context) (PullResult, error) {
	if ctx == nil {
		return PullResult{}, errors.New("context is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.store.LoadState()
	if err != nil {
		return PullResult{}, err
	}
	startSeq := state.Seq
	value, err := s.sdk.GetChatData(startSeq, s.limit, s.timeoutSeconds)
	if err != nil {
		s.recordPullError(err)
		return PullResult{}, err
	}
	var response getChatDataResponse
	if err := json.Unmarshal(value, &response); err != nil {
		s.recordPullError(err)
		return PullResult{}, fmt.Errorf("parse GetChatData response: %w", err)
	}
	if response.ErrCode != 0 {
		err := SDKError{Operation: "GetChatData", Code: response.ErrCode}
		s.recordPullError(err)
		return PullResult{}, err
	}
	now := s.now().UTC()
	evidence := make([]ArchiveEvidence, 0, len(response.ChatData))
	nextSeq := startSeq
	var lastVersion uint32
	for _, item := range response.ChatData {
		randomKey, err := decryptRandomKey(s.privateKey, item.EncryptedRandomKey)
		if err != nil {
			s.recordPullError(err)
			return PullResult{}, fmt.Errorf("decrypt random key for seq %d: %w", item.Seq, err)
		}
		plain, err := s.sdk.DecryptData(string(randomKey), item.EncryptedChatMsg)
		if err != nil {
			s.recordPullError(err)
			return PullResult{}, fmt.Errorf("decrypt chat message for seq %d: %w", item.Seq, err)
		}
		evidence = append(evidence, archiveEvidenceFromPlain(now, item, plain))
		if item.Seq > nextSeq {
			nextSeq = item.Seq
			lastVersion = item.PublicKeyVersion
		}
	}
	if _, err := s.store.CommitArchivePage(state, nextSeq, lastVersion, now, evidence); err != nil {
		return PullResult{}, err
	}
	return PullResult{StartSeq: startSeq, NextSeq: nextSeq, MessageCount: len(evidence), PulledAt: now}, nil
}

func (s *ArchiveService) recordPullError(pullErr error) {
	_ = s.store.RecordPullError(s.now().UTC(), pullErr)
}

func parseRSAPrivateKey(value string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(value)))
	if block == nil {
		return nil, errors.New("invalid RSA private key PEM")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.New("invalid RSA private key")
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not RSA")
	}
	return key, nil
}

func decryptRandomKey(key *rsa.PrivateKey, encoded string) ([]byte, error) {
	value, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	return rsa.DecryptPKCS1v15(rand.Reader, key, value)
}

func archiveEvidenceFromPlain(now time.Time, encrypted encryptedChatDatum, plain []byte) ArchiveEvidence {
	var message struct {
		MsgID   string   `json:"msgid"`
		From    string   `json:"from"`
		ToList  []string `json:"tolist"`
		RoomID  string   `json:"roomid"`
		MsgTime int64    `json:"msgtime"`
		MsgType string   `json:"msgtype"`
		Text    struct {
			Content string `json:"content"`
		} `json:"text"`
	}
	_ = json.Unmarshal(plain, &message)
	preview := []rune(strings.TrimSpace(message.Text.Content))
	if len(preview) > 200 {
		preview = preview[:200]
	}
	sum := sha256.Sum256(plain)
	msgID := message.MsgID
	if msgID == "" {
		msgID = encrypted.MsgID
	}
	return ArchiveEvidence{
		PulledAt: now, Seq: encrypted.Seq, MsgID: msgID,
		PublicKeyVersion: encrypted.PublicKeyVersion, MsgType: message.MsgType,
		From: message.From, ToList: message.ToList, RoomID: message.RoomID,
		MessageTime: message.MsgTime, Preview: string(preview), ContentSHA256: hex.EncodeToString(sum[:]),
	}
}
