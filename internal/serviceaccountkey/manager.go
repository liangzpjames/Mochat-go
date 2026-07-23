package serviceaccountkey

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"jiyi/mochat-go/internal/saasbackup"
)

const LegacyJWTKeyID = "legacy-jwt"

var keyIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

type Config struct {
	ActiveKeyID      string
	ActiveKey        string
	Keys             string
	LegacyJWTSecret  string
	AllowLegacyJWT   bool
	RequireDedicated bool
}

type ConfigStatus struct {
	ActiveKeyID         string
	KeyIDs              []string
	KeyCount            int
	DedicatedConfigured bool
	LegacyJWTEnabled    bool
	RequireDedicated    bool
}

type Manager struct {
	activeKeyID      string
	keys             map[string][]byte
	dedicatedKeyIDs  map[string]struct{}
	legacyJWTEnabled bool
	requireDedicated bool
}

func NewManager(config Config) (*Manager, error) {
	activeKeyID := strings.TrimSpace(config.ActiveKeyID)
	if activeKeyID == "" {
		activeKeyID = "primary"
	}
	if !validKeyID(activeKeyID) {
		return nil, fmt.Errorf("service account pepper active key ID %q is invalid", activeKeyID)
	}

	parsed, err := saasbackup.ParseEncryptionKeyRing(config.Keys)
	if err != nil {
		return nil, fmt.Errorf("service account pepper key ring: %w", err)
	}
	dedicated := make(map[string][]byte, len(parsed)+1)
	for keyID, key := range parsed {
		keyID = strings.TrimSpace(keyID)
		if !validKeyID(keyID) || keyID == LegacyJWTKeyID {
			return nil, fmt.Errorf("service account pepper key ID %q is invalid or reserved", keyID)
		}
		dedicated[keyID] = append([]byte(nil), key...)
	}
	if strings.TrimSpace(config.ActiveKey) != "" {
		if activeKeyID == LegacyJWTKeyID {
			return nil, fmt.Errorf("service account pepper key ID %q is reserved", LegacyJWTKeyID)
		}
		key, parseErr := saasbackup.ParseEncryptionKey(config.ActiveKey)
		if parseErr != nil {
			return nil, fmt.Errorf("service account active pepper: %w", parseErr)
		}
		if existing, ok := dedicated[activeKeyID]; ok && !hmac.Equal(existing, key) {
			return nil, fmt.Errorf("service account active pepper %q conflicts with the key ring", activeKeyID)
		}
		dedicated[activeKeyID] = key
	}

	dedicatedConfigured := len(dedicated) > 0
	if dedicatedConfigured {
		if _, ok := dedicated[activeKeyID]; !ok {
			return nil, fmt.Errorf("service account active pepper %q is missing from the key ring", activeKeyID)
		}
	} else {
		if config.RequireDedicated {
			return nil, errors.New("dedicated service account pepper is required")
		}
		if !config.AllowLegacyJWT || strings.TrimSpace(config.LegacyJWTSecret) == "" {
			return nil, errors.New("service account pepper is not configured")
		}
		activeKeyID = LegacyJWTKeyID
	}

	keys := make(map[string][]byte, len(dedicated)+1)
	dedicatedIDs := make(map[string]struct{}, len(dedicated))
	for keyID, key := range dedicated {
		keys[keyID] = append([]byte(nil), key...)
		dedicatedIDs[keyID] = struct{}{}
	}
	legacyEnabled := config.AllowLegacyJWT && strings.TrimSpace(config.LegacyJWTSecret) != ""
	if legacyEnabled {
		keys[LegacyJWTKeyID] = []byte(config.LegacyJWTSecret)
	}
	return &Manager{
		activeKeyID: activeKeyID, keys: keys, dedicatedKeyIDs: dedicatedIDs,
		legacyJWTEnabled: legacyEnabled, requireDedicated: config.RequireDedicated,
	}, nil
}

func NewLegacyManager(secret string) *Manager {
	manager, err := NewManager(Config{LegacyJWTSecret: secret, AllowLegacyJWT: true})
	if err != nil {
		return nil
	}
	return manager
}

func (m *Manager) HashActive(plainText string) (string, string, error) {
	if m == nil || strings.TrimSpace(m.activeKeyID) == "" {
		return "", "", errors.New("service account pepper is not configured")
	}
	digest, ok := m.Hash(m.activeKeyID, plainText)
	if !ok {
		return "", "", fmt.Errorf("service account active pepper %q is unavailable", m.activeKeyID)
	}
	return m.activeKeyID, digest, nil
}

func (m *Manager) Hash(keyID, plainText string) (string, bool) {
	if m == nil {
		return "", false
	}
	keyID = normalizeKeyID(keyID)
	key, ok := m.keys[keyID]
	if !ok || len(key) == 0 {
		return "", false
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(plainText))
	return hex.EncodeToString(mac.Sum(nil)), true
}

func (m *Manager) HasKey(keyID string) bool {
	if m == nil {
		return false
	}
	_, ok := m.keys[normalizeKeyID(keyID)]
	return ok
}

func (m *Manager) MissingKeyIDs(keyIDs []string) []string {
	missing := map[string]struct{}{}
	for _, keyID := range keyIDs {
		keyID = normalizeKeyID(keyID)
		if !m.HasKey(keyID) {
			missing[keyID] = struct{}{}
		}
	}
	result := make([]string, 0, len(missing))
	for keyID := range missing {
		result = append(result, keyID)
	}
	sort.Strings(result)
	return result
}

func (m *Manager) ConfigStatus() ConfigStatus {
	if m == nil {
		return ConfigStatus{}
	}
	keyIDs := make([]string, 0, len(m.keys))
	for keyID := range m.keys {
		keyIDs = append(keyIDs, keyID)
	}
	sort.Strings(keyIDs)
	return ConfigStatus{
		ActiveKeyID: m.activeKeyID, KeyIDs: keyIDs, KeyCount: len(keyIDs),
		DedicatedConfigured: len(m.dedicatedKeyIDs) > 0,
		LegacyJWTEnabled:    m.legacyJWTEnabled, RequireDedicated: m.requireDedicated,
	}
}

func (m *Manager) CheckConfiguration(context.Context) error {
	if m == nil {
		return errors.New("service account pepper manager is not configured")
	}
	status := m.ConfigStatus()
	if status.RequireDedicated && !status.DedicatedConfigured {
		return errors.New("dedicated service account pepper is required")
	}
	if !m.HasKey(status.ActiveKeyID) {
		return fmt.Errorf("service account active pepper %q is unavailable", status.ActiveKeyID)
	}
	return nil
}

func normalizeKeyID(keyID string) string {
	keyID = strings.TrimSpace(keyID)
	if keyID == "" {
		return LegacyJWTKeyID
	}
	return keyID
}

func validKeyID(keyID string) bool {
	return keyIDPattern.MatchString(strings.TrimSpace(keyID))
}
