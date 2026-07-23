package saasalertcredentials

import "strings"

// EncryptionSource keeps one encryption domain's active key, historical key ring, and key ID together.
type EncryptionSource struct {
	Key   string
	Keys  string
	KeyID string
}

// SelectEncryptionSource selects the dedicated source when it contains key material, otherwise the
// first fallback source containing key material. Fields from different encryption domains are never mixed.
func SelectEncryptionSource(dedicated EncryptionSource, fallbacks ...EncryptionSource) (EncryptionSource, bool) {
	if hasEncryptionKeyMaterial(dedicated) {
		return normalizeEncryptionSource(dedicated), true
	}
	for _, fallback := range fallbacks {
		if hasEncryptionKeyMaterial(fallback) {
			return normalizeEncryptionSource(fallback), false
		}
	}
	return EncryptionSource{KeyID: "primary"}, false
}

func hasEncryptionKeyMaterial(source EncryptionSource) bool {
	return strings.TrimSpace(source.Key) != "" || strings.TrimSpace(source.Keys) != ""
}

func normalizeEncryptionSource(source EncryptionSource) EncryptionSource {
	source.KeyID = strings.TrimSpace(source.KeyID)
	if source.KeyID == "" {
		source.KeyID = "primary"
	}
	return source
}
