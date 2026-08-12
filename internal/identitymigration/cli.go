package identitymigration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"jiyi/mochat-go/internal/saasbackup"
	"jiyi/mochat-go/internal/wecomcredentials"
)

func VerifyTargetSchema(ctx context.Context, db *sql.DB, schema string) error {
	if db == nil || strings.TrimSpace(schema) == "" {
		return errors.New("maintenance target schema is required")
	}
	var actual sql.NullString
	if err := db.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&actual); err != nil || !actual.Valid || actual.String != strings.TrimSpace(schema) {
		return errors.New("database schema does not match the maintenance target")
	}
	return nil
}

func ReadSecretFile(path string) ([]byte, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("secret file path is required")
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return nil, errors.New("secret file is unavailable")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New("secret file is unavailable")
	}
	data = []byte(strings.TrimSpace(string(data)))
	if len(data) == 0 {
		return nil, errors.New("secret file is empty")
	}
	return data, nil
}

func NewCredentialManagerFromFile(path, keyID string) (*wecomcredentials.Manager, error) {
	if strings.TrimSpace(keyID) == "" {
		return nil, errors.New("credential encryption key id is required")
	}
	key, err := ReadSecretFile(path)
	if err != nil {
		return nil, err
	}
	config := wecomcredentials.Config{
		EncryptionKeyID:     strings.TrimSpace(keyID),
		RequireEncryption:   true,
		DedicatedConfigured: true,
	}
	if _, ringErr := saasbackup.ParseEncryptionKeyRing(string(key)); ringErr == nil {
		config.EncryptionKeys = string(key)
	} else {
		config.EncryptionKey = string(key)
	}
	manager, err := wecomcredentials.NewManager(config)
	if err != nil {
		return nil, errors.New("credential encryption manager is invalid")
	}
	return manager, nil
}

func ControlledMigrationPath(projectRoot, action string) (string, error) {
	root := strings.TrimSpace(projectRoot)
	if root == "" {
		return "", errors.New("project root is required")
	}
	switch strings.TrimSpace(action) {
	case "up", "backfill":
		return filepath.Join(root, "deploy", "standalone", "migrations", "0130_identity_realms_single_corp_backfill.up.sql"), nil
	case "down":
		return filepath.Join(root, "deploy", "standalone", "migrations", "0130_identity_realms_single_corp_backfill.down.sql"), nil
	case "cutover":
		return filepath.Join(root, "deploy", "standalone", "migrations", "0131_identity_realms_single_corp_cutover.up.sql"), nil
	case "cutover-down":
		return filepath.Join(root, "deploy", "standalone", "migrations", "0131_identity_realms_single_corp_cutover.down.sql"), nil
	case "encrypt-credentials":
		return "", nil
	case "restore-legacy-credentials":
		return "", nil
	default:
		return "", fmt.Errorf("unknown controlled migration action %q", action)
	}
}

func ReadMappingDocument(mappingPath, signingKeyPath string) (MappingDocument, error) {
	if strings.TrimSpace(mappingPath) == "" && strings.TrimSpace(signingKeyPath) == "" {
		return MappingDocument{}, nil
	}
	if strings.TrimSpace(mappingPath) == "" || strings.TrimSpace(signingKeyPath) == "" {
		return MappingDocument{}, errors.New("mapping file and signing key file must be provided together")
	}
	document, err := os.ReadFile(mappingPath)
	if err != nil {
		return MappingDocument{}, errors.New("mapping document is unavailable")
	}
	key, err := ReadSecretFile(signingKeyPath)
	if err != nil {
		return MappingDocument{}, err
	}
	return ParseMappingDocument(document, key)
}
