package archivebridge

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"

	"jiyi/mochat-go/internal/archivefixture"
	"jiyi/mochat-go/internal/wecomarchivedemo"
)

const (
	ModeSelfBuilt           = "self_built"
	ModeThirdPartyDelegated = "third_party_delegated"
)

type Binding struct {
	TenantID        int64  `json:"tenant_id"`
	CorpID          int64  `json:"corp_id"`
	WXCorpID        string `json:"wx_corpid"`
	IntegrationMode string `json:"integration_mode"`
}

type FinanceDriver interface {
	FetchPage(context.Context, uint64, uint32) (wecomarchivedemo.ArchivePage, error)
	FetchMediaWithTimeout(context.Context, string, string, int) (wecomarchivedemo.MediaChunk, error)
}

type DataZoneDriver interface {
	Fetch(string, int64, int) ([]archivefixture.DataZoneMessage, error)
	DecryptSecretKey(archivefixture.DataZoneMessage) ([]byte, error)
	Render(string, string, []byte) (archivefixture.DataZoneContent, error)
}

type Driver struct {
	Binding  Binding
	Finance  FinanceDriver
	DataZone DataZoneDriver
}

type BridgeError struct {
	Code  string
	Cause error
}

func (e *BridgeError) Error() string {
	if e == nil || strings.TrimSpace(e.Code) == "" {
		return "archive bridge request failed"
	}
	return "archive bridge request failed: " + strings.TrimSpace(e.Code)
}

func (e *BridgeError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func ErrorCode(err error) string {
	var typed *BridgeError
	if errors.As(err, &typed) && typed != nil {
		return strings.TrimSpace(typed.Code)
	}
	return ""
}

type Store struct {
	mu      sync.RWMutex
	drivers map[string]Driver
}

func NewStore() *Store { return &Store{drivers: map[string]Driver{}} }

func (s *Store) RegisterFinance(binding Binding, driver FinanceDriver) error {
	if driver == nil || normalizeBinding(&binding) != nil || binding.IntegrationMode != ModeSelfBuilt {
		return &BridgeError{Code: "ARCHIVE_BINDING_INVALID"}
	}
	return s.register(Driver{Binding: binding, Finance: driver})
}

func (s *Store) RegisterDataZone(binding Binding, driver DataZoneDriver) error {
	if driver == nil || normalizeBinding(&binding) != nil || binding.IntegrationMode != ModeThirdPartyDelegated {
		return &BridgeError{Code: "ARCHIVE_BINDING_INVALID"}
	}
	return s.register(Driver{Binding: binding, DataZone: driver})
}

func (s *Store) register(driver Driver) error {
	if s == nil {
		return &BridgeError{Code: "ARCHIVE_STORE_UNAVAILABLE"}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := bindingKey(driver.Binding.TenantID, driver.Binding.CorpID)
	if existing, ok := s.drivers[key]; ok {
		if existing.Binding != driver.Binding {
			return &BridgeError{Code: "ARCHIVE_BINDING_CONFLICT"}
		}
	}
	s.drivers[key] = driver
	return nil
}

func (s *Store) Resolve(binding Binding) (Driver, error) {
	if s == nil || normalizeBinding(&binding) != nil {
		return Driver{}, &BridgeError{Code: "ARCHIVE_BINDING_MISMATCH"}
	}
	s.mu.RLock()
	driver, ok := s.drivers[bindingKey(binding.TenantID, binding.CorpID)]
	s.mu.RUnlock()
	if !ok || driver.Binding != binding {
		return Driver{}, &BridgeError{Code: "ARCHIVE_BINDING_MISMATCH"}
	}
	return driver, nil
}

func (s *Store) Status() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	counts := map[string]int{ModeSelfBuilt: 0, ModeThirdPartyDelegated: 0}
	for _, driver := range s.drivers {
		counts[driver.Binding.IntegrationMode]++
	}
	return map[string]any{"status": "ok", "bindingCount": len(s.drivers), "modeCounts": counts}
}

func (s *Store) DriverCount() int {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.drivers)
}

func (s *Store) Unregister(binding Binding) error {
	if s == nil || normalizeBinding(&binding) != nil {
		return &BridgeError{Code: "ARCHIVE_BINDING_INVALID"}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := bindingKey(binding.TenantID, binding.CorpID)
	driver, ok := s.drivers[key]
	if ok && driver.Binding != binding {
		return &BridgeError{Code: "ARCHIVE_BINDING_CONFLICT"}
	}
	delete(s.drivers, key)
	return nil
}

func normalizeBinding(binding *Binding) error {
	if binding == nil {
		return &BridgeError{Code: "ARCHIVE_BINDING_INVALID"}
	}
	binding.WXCorpID = strings.TrimSpace(binding.WXCorpID)
	binding.IntegrationMode = strings.TrimSpace(binding.IntegrationMode)
	if binding.TenantID <= 0 || binding.CorpID <= 0 || binding.WXCorpID == "" || (binding.IntegrationMode != ModeSelfBuilt && binding.IntegrationMode != ModeThirdPartyDelegated) {
		return &BridgeError{Code: "ARCHIVE_BINDING_INVALID"}
	}
	return nil
}

func bindingKey(tenantID, corpID int64) string {
	raw, _ := json.Marshal([]int64{tenantID, corpID})
	return string(raw)
}
