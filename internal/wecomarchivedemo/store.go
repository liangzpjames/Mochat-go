package wecomarchivedemo

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type State struct {
	BoundReceiveID       string    `json:"bound_receive_id,omitempty"`
	Seq                  uint64    `json:"seq"`
	CallbackCount        uint64    `json:"callback_count"`
	PulledMessageCount   uint64    `json:"pulled_message_count"`
	PullCount            uint64    `json:"pull_count"`
	LastCallbackAt       time.Time `json:"last_callback_at,omitempty"`
	LastPullAt           time.Time `json:"last_pull_at,omitempty"`
	LastPullError        string    `json:"last_pull_error,omitempty"`
	LastCallbackEvent    string    `json:"last_callback_event,omitempty"`
	LastPublicKeyVersion uint32    `json:"last_public_key_version,omitempty"`
}

type CallbackEvidence struct {
	ReceivedAt    time.Time `json:"received_at"`
	ReceiveID     string    `json:"receive_id"`
	EventPath     string    `json:"event_path"`
	ContentSHA256 string    `json:"content_sha256"`
}

type ArchiveEvidence struct {
	PulledAt         time.Time `json:"pulled_at"`
	Seq              uint64    `json:"seq"`
	MsgID            string    `json:"msgid"`
	PublicKeyVersion uint32    `json:"publickey_ver"`
	MsgType          string    `json:"msgtype,omitempty"`
	From             string    `json:"from,omitempty"`
	ToList           []string  `json:"tolist,omitempty"`
	RoomID           string    `json:"roomid,omitempty"`
	MessageTime      int64     `json:"msgtime,omitempty"`
	Preview          string    `json:"preview,omitempty"`
	ContentSHA256    string    `json:"content_sha256"`
}

type EvidenceStore struct {
	dir string
	mu  sync.Mutex
}

func NewEvidenceStore(dir string) (*EvidenceStore, error) {
	if dir == "" {
		return nil, errors.New("evidence directory is required")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	_ = os.Chmod(dir, 0o700)
	return &EvidenceStore{dir: dir}, nil
}

func (s *EvidenceStore) LoadState() (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadStateLocked()
}

func (s *EvidenceStore) loadStateLocked() (State, error) {
	value, err := os.ReadFile(filepath.Join(s.dir, "state.json"))
	if errors.Is(err, os.ErrNotExist) {
		return State{}, nil
	}
	if err != nil {
		return State{}, err
	}
	var state State
	if err := json.Unmarshal(value, &state); err != nil {
		return State{}, err
	}
	return state, nil
}

func (s *EvidenceStore) SaveState(state State) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveStateLocked(state)
}

func (s *EvidenceStore) saveStateLocked(state State) error {
	value, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(s.dir, "state.json")
	temporary, err := os.CreateTemp(s.dir, ".state-*.json")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(value); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

func (s *EvidenceStore) BindReceiveID(receiveID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	receiveID = strings.TrimSpace(receiveID)
	if receiveID == "" {
		return errors.New("callback receive ID is empty")
	}
	state, err := s.loadStateLocked()
	if err != nil {
		return err
	}
	if state.BoundReceiveID != "" && state.BoundReceiveID != receiveID {
		return errors.New("callback receive ID is already bound to another enterprise")
	}
	if state.BoundReceiveID == receiveID {
		return nil
	}
	state.BoundReceiveID = receiveID
	return s.saveStateLocked(state)
}

func (s *EvidenceStore) RecordCallback(value CallbackEvidence) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	value.ReceiveID = strings.TrimSpace(value.ReceiveID)
	if value.ReceiveID == "" {
		return errors.New("callback receive ID is empty")
	}
	state, err := s.loadStateLocked()
	if err != nil {
		return err
	}
	if state.BoundReceiveID != "" && state.BoundReceiveID != value.ReceiveID {
		return errors.New("callback receive ID is already bound to another enterprise")
	}
	state.BoundReceiveID = value.ReceiveID
	if err := s.appendJSONLineLocked("callback-events.jsonl", value); err != nil {
		return err
	}
	state.CallbackCount++
	state.LastCallbackAt = value.ReceivedAt
	state.LastCallbackEvent = value.EventPath
	return s.saveStateLocked(state)
}

func (s *EvidenceStore) AppendCallback(value CallbackEvidence) error {
	return s.appendJSONLine("callback-events.jsonl", value)
}

func (s *EvidenceStore) AppendArchive(value ArchiveEvidence) error {
	return s.appendJSONLine("archive-messages.jsonl", value)
}

func (s *EvidenceStore) CommitArchivePage(state State, nextSeq uint64, lastVersion uint32, pulledAt time.Time, values []ArchiveEvidence) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	seen, err := s.archiveEvidenceKeysLocked()
	if err != nil {
		return state, err
	}
	for _, value := range values {
		key := archiveEvidenceKey(value)
		if _, exists := seen[key]; exists {
			continue
		}
		if err := s.appendJSONLineLocked("archive-messages.jsonl", value); err != nil {
			return state, err
		}
		seen[key] = struct{}{}
	}
	state.Seq = nextSeq
	state.PullCount++
	state.PulledMessageCount = uint64(len(seen))
	state.LastPullAt = pulledAt
	state.LastPullError = ""
	if lastVersion != 0 {
		state.LastPublicKeyVersion = lastVersion
	}
	if err := s.saveStateLocked(state); err != nil {
		return state, err
	}
	return state, nil
}

func (s *EvidenceStore) archiveEvidenceKeysLocked() (map[string]struct{}, error) {
	seen := make(map[string]struct{})
	file, err := os.Open(filepath.Join(s.dir, "archive-messages.jsonl"))
	if errors.Is(err, os.ErrNotExist) {
		return seen, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	for {
		var value ArchiveEvidence
		if err := decoder.Decode(&value); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, fmt.Errorf("read archive evidence: %w", err)
		}
		seen[archiveEvidenceKey(value)] = struct{}{}
	}
	return seen, nil
}

func archiveEvidenceKey(value ArchiveEvidence) string {
	return fmt.Sprintf("%d|%s", value.Seq, strings.TrimSpace(value.MsgID))
}

func (s *EvidenceStore) appendJSONLine(name string, value any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.appendJSONLineLocked(name, value)
}

func (s *EvidenceStore) appendJSONLineLocked(name string, value any) error {
	file, err := os.OpenFile(filepath.Join(s.dir, name), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	_ = file.Chmod(0o600)
	writer := bufio.NewWriter(file)
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		return err
	}
	if err := writer.Flush(); err != nil {
		return err
	}
	return file.Sync()
}
