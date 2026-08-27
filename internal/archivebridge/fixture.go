package archivebridge

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"jiyi/mochat-go/internal/archivefixture"
	"jiyi/mochat-go/internal/testfixtures/archivesource"
	"jiyi/mochat-go/internal/wecomarchivedemo"
)

const FixtureDatasetPrefix = "MOCHAT-LOCAL-SIM-"

type FixtureSendInput struct {
	Binding
	Dataset    string `json:"dataset"`
	Type       string `json:"type"`
	Text       string `json:"text,omitempty"`
	DataBase64 string `json:"dataBase64,omitempty"`
	FileName   string `json:"fileName,omitempty"`
	MIMEType   string `json:"mimeType,omitempty"`
}

type FixtureSendResult struct {
	Dataset   string `json:"dataset"`
	Mode      string `json:"mode"`
	Sequence  int64  `json:"sequence"`
	MessageID string `json:"messageId"`
	SDKFileID string `json:"sdkFileId,omitempty"`
}

type FixtureStatus struct {
	Enabled                  bool `json:"enabled"`
	DatasetCount             int  `json:"datasetCount"`
	MessageCount             int  `json:"messageCount"`
	DelegatedAuthorizedCount int  `json:"delegatedAuthorizedCount"`
}

type fixtureState struct {
	Version  int                        `json:"version"`
	Datasets map[string]*fixtureDataset `json:"datasets"`
}

type fixtureDataset struct {
	Binding         Binding                       `json:"binding"`
	Dataset         string                        `json:"dataset"`
	SelfMessages    []fixtureStoredMessage        `json:"selfMessages,omitempty"`
	DataZone        *archivefixture.DataZoneState `json:"dataZone,omitempty"`
	SuiteAuthorized bool                          `json:"suiteAuthorized,omitempty"`
}

type fixtureStoredMessage struct {
	Sequence  int64  `json:"sequence"`
	MessageID string `json:"messageId"`
	Type      string `json:"type"`
	Body      []byte `json:"body"`
	FileName  string `json:"fileName,omitempty"`
	MIMEType  string `json:"mimeType,omitempty"`
}

type fixtureRuntime struct {
	finance *archivesource.ArchiveFixture
	zone    *archivefixture.DataZoneProvider
}

type FixtureManager struct {
	mu        sync.Mutex
	statePath string
	store     *Store
	state     fixtureState
	runtimes  map[string]fixtureRuntime
	suite     *archivefixture.SuiteProvider
}

func NewFixtureManager(statePath string, store *Store) (*FixtureManager, error) {
	statePath = strings.TrimSpace(statePath)
	if statePath == "" || store == nil {
		return nil, errors.New("archive fixture manager configuration is invalid")
	}
	suite, err := archivefixture.NewSuiteProvider("ww-local-fixture-suite", "local-fixture-suite-secret", "local-fixture-callback-token", "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG")
	if err != nil {
		return nil, errors.New("archive fixture suite unavailable")
	}
	manager := &FixtureManager{statePath: statePath, store: store, state: fixtureState{Version: 1, Datasets: map[string]*fixtureDataset{}}, runtimes: map[string]fixtureRuntime{}, suite: suite}
	if err := manager.load(); err != nil {
		return nil, err
	}
	return manager, nil
}

func (m *FixtureManager) Seed(binding Binding, dataset string) (FixtureStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if normalizeBinding(&binding) != nil || !validFixtureDataset(dataset) {
		return FixtureStatus{}, &BridgeError{Code: "FIXTURE_REQUEST_INVALID"}
	}
	key := fixtureDatasetKey(binding, dataset)
	if existing, ok := m.state.Datasets[key]; ok {
		if existing.Binding != binding {
			return FixtureStatus{}, &BridgeError{Code: "FIXTURE_BINDING_CONFLICT"}
		}
		return m.statusLocked(), nil
	}
	entry := &fixtureDataset{Binding: binding, Dataset: dataset}
	runtime, err := m.createRuntime(entry, true)
	if err != nil {
		return FixtureStatus{}, err
	}
	m.state.Datasets[key] = entry
	m.runtimes[key] = runtime
	if binding.IntegrationMode == ModeThirdPartyDelegated {
		if err := m.authorizeDelegatedLocked(entry); err != nil {
			delete(m.state.Datasets, key)
			delete(m.runtimes, key)
			_ = m.store.Unregister(binding)
			return FixtureStatus{}, err
		}
	}
	if err := m.persistLocked(); err != nil {
		delete(m.state.Datasets, key)
		delete(m.runtimes, key)
		_ = m.store.Unregister(binding)
		return FixtureStatus{}, err
	}
	return m.statusLocked(), nil
}

func (m *FixtureManager) Send(input FixtureSendInput) (FixtureSendResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if normalizeBinding(&input.Binding) != nil || !validFixtureDataset(input.Dataset) {
		return FixtureSendResult{}, &BridgeError{Code: "FIXTURE_REQUEST_INVALID"}
	}
	key := fixtureDatasetKey(input.Binding, input.Dataset)
	entry, ok := m.state.Datasets[key]
	runtime := m.runtimes[key]
	if !ok || entry.Binding != input.Binding {
		return FixtureSendResult{}, &BridgeError{Code: "FIXTURE_DATASET_NOT_FOUND"}
	}
	messageType := strings.ToLower(strings.TrimSpace(input.Type))
	body, err := fixtureBody(input, messageType)
	if err != nil {
		return FixtureSendResult{}, err
	}
	sequence := nextFixtureSequence(entry)
	messageID := fmt.Sprintf("%s-%s-%04d", input.Dataset, strings.ReplaceAll(input.IntegrationMode, "_", "-"), sequence)
	result := FixtureSendResult{Dataset: input.Dataset, Mode: input.IntegrationMode, Sequence: sequence, MessageID: messageID}
	if runtime.finance != nil {
		item, err := runtime.finance.Append(archivesource.FixtureInput{Sequence: uint64(sequence), MessageID: messageID, Type: messageType, Body: body, FileName: input.FileName, MIMEType: input.MIMEType})
		if err != nil {
			return FixtureSendResult{}, &BridgeError{Code: "FIXTURE_SEND_FAILED"}
		}
		entry.SelfMessages = append(entry.SelfMessages, fixtureStoredMessage{Sequence: sequence, MessageID: messageID, Type: messageType, Body: append([]byte(nil), body...), FileName: input.FileName, MIMEType: input.MIMEType})
		result.SDKFileID = item.SDKFileID
	} else if runtime.zone != nil {
		_, err := runtime.zone.Append(archivefixture.DataZoneContent{Sequence: sequence, MessageID: messageID, Type: messageType, Sender: input.Dataset + "-STAFF", Receivers: []string{input.Dataset + "-EXTERNAL"}, Body: body, FileName: input.FileName, MIMEType: input.MIMEType})
		if err != nil {
			return FixtureSendResult{}, &BridgeError{Code: "FIXTURE_SEND_FAILED"}
		}
		state, err := runtime.zone.ExportState()
		if err != nil {
			return FixtureSendResult{}, &BridgeError{Code: "FIXTURE_PERSIST_FAILED"}
		}
		entry.DataZone = &state
	} else {
		return FixtureSendResult{}, &BridgeError{Code: "FIXTURE_RUNTIME_UNAVAILABLE"}
	}
	if err := m.persistLocked(); err != nil {
		return FixtureSendResult{}, err
	}
	return result, nil
}

func (m *FixtureManager) Cleanup(binding Binding, dataset, confirmation string) (FixtureStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if normalizeBinding(&binding) != nil || !validFixtureDataset(dataset) || strings.TrimSpace(confirmation) != strings.TrimSpace(dataset) {
		return FixtureStatus{}, &BridgeError{Code: "FIXTURE_CLEANUP_CONFIRMATION_REQUIRED"}
	}
	key := fixtureDatasetKey(binding, dataset)
	entry, exists := m.state.Datasets[key]
	if exists && entry.Binding == binding {
		_ = m.store.Unregister(entry.Binding)
		if runtime := m.runtimes[key]; runtime.finance != nil {
			_ = runtime.finance.Close()
		}
		delete(m.runtimes, key)
		delete(m.state.Datasets, key)
	}
	if err := m.persistLocked(); err != nil {
		return FixtureStatus{}, err
	}
	return m.statusLocked(), nil
}

func (m *FixtureManager) Status() FixtureStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.statusLocked()
}

func (m *FixtureManager) statusLocked() FixtureStatus {
	status := FixtureStatus{Enabled: true, DatasetCount: len(m.state.Datasets)}
	for _, entry := range m.state.Datasets {
		status.MessageCount += len(entry.SelfMessages)
		if entry.DataZone != nil {
			status.MessageCount += len(entry.DataZone.Entries)
		}
		if entry.SuiteAuthorized {
			status.DelegatedAuthorizedCount++
		}
	}
	return status
}

func (m *FixtureManager) load() error {
	raw, err := os.ReadFile(m.statePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read archive fixture state: %w", err)
	}
	var state fixtureState
	if json.Unmarshal(raw, &state) != nil || state.Version != 1 || state.Datasets == nil {
		return errors.New("archive fixture state is invalid")
	}
	m.state = state
	keys := make([]string, 0, len(state.Datasets))
	for key := range state.Datasets {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		entry := state.Datasets[key]
		if entry == nil || !validFixtureDataset(entry.Dataset) || normalizeBinding(&entry.Binding) != nil {
			return errors.New("archive fixture dataset state is invalid")
		}
		runtime, err := m.createRuntime(entry, false)
		if err != nil {
			return err
		}
		m.runtimes[key] = runtime
		if entry.Binding.IntegrationMode == ModeThirdPartyDelegated {
			if err := m.authorizeDelegatedLocked(entry); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *FixtureManager) authorizeDelegatedLocked(entry *fixtureDataset) error {
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	ticket := entry.Dataset + "-suite-ticket"
	values, encrypted, err := m.suite.BuildTicketCallback(ticket, timestamp, "fixture-nonce-"+fmt.Sprint(entry.Binding.TenantID))
	if err != nil {
		return &BridgeError{Code: "FIXTURE_SUITE_AUTH_FAILED"}
	}
	if err = m.suite.ReceiveTicket(values, encrypted); err != nil {
		return &BridgeError{Code: "FIXTURE_SUITE_AUTH_FAILED"}
	}
	suiteToken, err := m.suite.ExchangeSuiteToken("ww-local-fixture-suite", "local-fixture-suite-secret", ticket)
	if err != nil {
		return &BridgeError{Code: "FIXTURE_SUITE_AUTH_FAILED"}
	}
	preAuth, err := m.suite.CreatePreAuthCode(suiteToken.Value)
	if err != nil {
		return &BridgeError{Code: "FIXTURE_SUITE_AUTH_FAILED"}
	}
	authCode, err := m.suite.Authorize(preAuth, int(entry.Binding.TenantID), entry.Binding.WXCorpID)
	if err != nil {
		return &BridgeError{Code: "FIXTURE_SUITE_AUTH_FAILED"}
	}
	authorization, err := m.suite.ExchangePermanentCode(suiteToken.Value, authCode)
	if err != nil {
		return &BridgeError{Code: "FIXTURE_SUITE_AUTH_FAILED"}
	}
	if _, err = m.suite.CorpToken(suiteToken.Value, int(entry.Binding.TenantID), entry.Binding.WXCorpID, authorization.PermanentCode); err != nil {
		return &BridgeError{Code: "FIXTURE_SUITE_AUTH_FAILED"}
	}
	entry.SuiteAuthorized = true
	return nil
}

func (m *FixtureManager) createRuntime(entry *fixtureDataset, seed bool) (fixtureRuntime, error) {
	if entry.Binding.IntegrationMode == ModeSelfBuilt {
		fixture, err := archivesource.NewArchiveFixture()
		if err != nil {
			return fixtureRuntime{}, errors.New("create finance fixture failed")
		}
		for _, item := range entry.SelfMessages {
			if _, err := fixture.Append(archivesource.FixtureInput{Sequence: uint64(item.Sequence), MessageID: item.MessageID, Type: item.Type, Body: item.Body, FileName: item.FileName, MIMEType: item.MIMEType}); err != nil {
				return fixtureRuntime{}, errors.New("restore finance fixture failed")
			}
		}
		evidenceDir := filepath.Join(filepath.Dir(m.statePath), "evidence", fmt.Sprintf("%d-%d", entry.Binding.TenantID, entry.Binding.CorpID))
		evidence, err := wecomarchivedemo.NewEvidenceStore(evidenceDir)
		if err != nil {
			return fixtureRuntime{}, errors.New("create finance fixture evidence failed")
		}
		service, err := wecomarchivedemo.NewArchiveService(fixture, fixture.PrivateKeyPEM(), evidence, 100, 5)
		if err != nil {
			return fixtureRuntime{}, errors.New("create finance fixture service failed")
		}
		if err := m.store.RegisterFinance(entry.Binding, service); err != nil {
			return fixtureRuntime{}, err
		}
		return fixtureRuntime{finance: fixture}, nil
	}
	var zone *archivefixture.DataZoneProvider
	var err error
	if entry.DataZone != nil {
		zone, err = archivefixture.NewDataZoneProviderFromState(*entry.DataZone)
	} else {
		zone, err = archivefixture.NewDataZoneProvider(entry.Binding.WXCorpID)
		if err == nil {
			zone.WithContentTTL(30 * 24 * time.Hour)
			if seed {
				for index, kind := range []string{"text", "image", "voice", "video", "file"} {
					_, err = zone.Append(archivefixture.DataZoneContent{Sequence: int64(index + 1), MessageID: fmt.Sprintf("%s-DELEGATED-%02d", entry.Dataset, index+1), Type: kind, Sender: entry.Dataset + "-STAFF", Receivers: []string{entry.Dataset + "-EXTERNAL"}, Body: []byte(entry.Dataset + " delegated " + kind), FileName: entry.Dataset + ".bin", MIMEType: fixtureMIME(kind)})
					if err != nil {
						break
					}
				}
				if err == nil {
					state, exportErr := zone.ExportState()
					err = exportErr
					entry.DataZone = &state
				}
			}
		}
	}
	if err != nil {
		return fixtureRuntime{}, errors.New("create delegated fixture failed")
	}
	if err := m.store.RegisterDataZone(entry.Binding, zone); err != nil {
		return fixtureRuntime{}, err
	}
	return fixtureRuntime{zone: zone}, nil
}

func (m *FixtureManager) persistLocked() error {
	if err := os.MkdirAll(filepath.Dir(m.statePath), 0o700); err != nil {
		return errors.New("archive fixture state directory unavailable")
	}
	raw, err := json.MarshalIndent(m.state, "", "  ")
	if err != nil {
		return errors.New("archive fixture state encoding failed")
	}
	temporary := m.statePath + ".tmp"
	if err := os.WriteFile(temporary, raw, 0o600); err != nil {
		return errors.New("archive fixture state write failed")
	}
	_ = os.Chmod(temporary, 0o600)
	if err := os.Rename(temporary, m.statePath); err != nil {
		if removeErr := os.Remove(m.statePath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return errors.New("archive fixture state replace failed")
		}
		if err := os.Rename(temporary, m.statePath); err != nil {
			return errors.New("archive fixture state replace failed")
		}
	}
	return nil
}

func validFixtureDataset(value string) bool {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, FixtureDatasetPrefix) || len(value) > 96 {
		return false
	}
	for _, character := range value {
		if !(character == '-' || character == '_' || character >= '0' && character <= '9' || character >= 'A' && character <= 'Z' || character >= 'a' && character <= 'z') {
			return false
		}
	}
	return true
}

func fixtureDatasetKey(binding Binding, dataset string) string {
	return fmt.Sprintf("%d:%d:%s:%s", binding.TenantID, binding.CorpID, binding.IntegrationMode, strings.TrimSpace(dataset))
}

func nextFixtureSequence(entry *fixtureDataset) int64 {
	maximum := int64(10)
	for _, item := range entry.SelfMessages {
		if item.Sequence > maximum {
			maximum = item.Sequence
		}
	}
	if entry.DataZone != nil {
		for _, item := range entry.DataZone.Entries {
			if item.Message.Sequence > maximum {
				maximum = item.Message.Sequence
			}
		}
	}
	return maximum + 1
}

func fixtureBody(input FixtureSendInput, messageType string) ([]byte, error) {
	if messageType == "text" {
		value := strings.TrimSpace(input.Text)
		if value == "" {
			return nil, &BridgeError{Code: "FIXTURE_MESSAGE_INVALID"}
		}
		return []byte(input.Dataset + " " + value), nil
	}
	if messageType != "image" && messageType != "voice" && messageType != "video" && messageType != "file" {
		return nil, &BridgeError{Code: "FIXTURE_MESSAGE_TYPE_UNSUPPORTED"}
	}
	value, err := base64.StdEncoding.DecodeString(strings.TrimSpace(input.DataBase64))
	if err != nil || len(value) == 0 || len(value) > 20<<20 {
		return nil, &BridgeError{Code: "FIXTURE_MESSAGE_INVALID"}
	}
	return value, nil
}

func fixtureMIME(kind string) string {
	switch kind {
	case "image":
		return "image/png"
	case "voice":
		return "audio/wav"
	case "video":
		return "video/mp4"
	case "file":
		return "application/octet-stream"
	default:
		return "text/plain; charset=utf-8"
	}
}
