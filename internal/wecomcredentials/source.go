package wecomcredentials

import "strings"

type EncryptionSource struct {
	Key   string
	Keys  string
	KeyID string
}

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
