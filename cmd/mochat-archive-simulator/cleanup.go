package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"jiyi/mochat-go/internal/archivebridge"
	"jiyi/mochat-go/internal/dashboard"

	"github.com/google/uuid"
)

type fixtureCleanupResult struct {
	Dataset             string `json:"dataset"`
	SourceIdentity      string `json:"sourceIdentity"`
	RemovedMessages     int    `json:"removedMessages"`
	RemovedMedia        int    `json:"removedMedia"`
	RemovedComponents   int    `json:"removedComponents"`
	RemovedParticipants int    `json:"removedParticipants"`
	RemovedRuns         int64  `json:"removedRuns"`
	RemovedFiles        int    `json:"removedFiles"`
}

func cleanupIngestedFixture(ctx context.Context, db *sql.DB, binding archivebridge.Binding, dataset, storageRoot string) (fixtureCleanupResult, error) {
	dataset = strings.TrimSpace(dataset)
	sourceID := "wecom:" + binding.IntegrationMode + ":" + binding.WXCorpID
	result := fixtureCleanupResult{Dataset: dataset, SourceIdentity: sourceID}
	if db == nil || binding.TenantID <= 0 || binding.CorpID <= 0 || !strings.HasPrefix(dataset, archivebridge.FixtureDatasetPrefix) {
		return result, errors.New("fixture cleanup scope is invalid")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `
		SELECT msgid
		FROM mochat_go_archive_message_sources
		WHERE tenant_id=? AND corp_id=? AND source_kind='external' AND source_id=? AND namespace=?
		ORDER BY id FOR UPDATE
	`, binding.TenantID, binding.CorpID, sourceID, sourceID)
	if err != nil {
		return result, err
	}
	messageIDs := make([]string, 0)
	for rows.Next() {
		var messageID string
		if err := rows.Scan(&messageID); err != nil {
			rows.Close()
			return result, err
		}
		messageIDs = append(messageIDs, messageID)
	}
	if err := rows.Close(); err != nil {
		return result, err
	}
	if err := validateFixtureCleanupMessageIDs(dataset, messageIDs); err != nil {
		return result, err
	}
	mediaIDs := make([]string, 0)
	for _, messageID := range messageIDs {
		mediaRows, err := tx.QueryContext(ctx, `SELECT id FROM mochat_go_archive_media_objects WHERE tenant_id=? AND corp_id=? AND msgid=? FOR UPDATE`, binding.TenantID, binding.CorpID, messageID)
		if err != nil {
			return result, err
		}
		for mediaRows.Next() {
			var mediaID string
			if err := mediaRows.Scan(&mediaID); err != nil {
				mediaRows.Close()
				return result, err
			}
			mediaIDs = append(mediaIDs, mediaID)
		}
		if err := mediaRows.Close(); err != nil {
			return result, err
		}
		if removed, err := deleteFixtureRows(ctx, tx, `DELETE FROM mochat_go_archive_component_locators WHERE tenant_id=? AND corp_id=? AND msgid=?`, binding.TenantID, binding.CorpID, messageID); err != nil {
			return result, err
		} else {
			result.RemovedComponents += int(removed)
		}
		if removed, err := deleteFixtureRows(ctx, tx, `DELETE FROM mochat_go_archive_media_objects WHERE tenant_id=? AND corp_id=? AND msgid=?`, binding.TenantID, binding.CorpID, messageID); err != nil {
			return result, err
		} else {
			result.RemovedMedia += int(removed)
		}
		for tableIndex := 1; tableIndex <= dashboard.WorkMessageArchiveMessageTableCount; tableIndex++ {
			query := fmt.Sprintf("DELETE FROM mc_work_message_%d WHERE corp_id=? AND msgid=?", tableIndex)
			if removed, err := deleteFixtureRows(ctx, tx, query, binding.CorpID, messageID); err != nil {
				return result, err
			} else {
				result.RemovedMessages += int(removed)
			}
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM mochat_go_archive_message_sources WHERE tenant_id=? AND corp_id=? AND source_kind='external' AND source_id=? AND namespace=?`, binding.TenantID, binding.CorpID, sourceID, sourceID); err != nil {
		return result, err
	}
	removedRuns, err := deleteFixtureRows(ctx, tx, `DELETE FROM mochat_go_archive_sync_runs WHERE tenant_id=? AND corp_id=? AND source_kind='external' AND source_id=? AND namespace=?`, binding.TenantID, binding.CorpID, sourceID, sourceID)
	if err != nil {
		return result, err
	}
	result.RemovedRuns = removedRuns
	staffWXID, externalWXID := fixtureParticipantIdentities(dataset)
	if removed, err := deleteFixtureRows(ctx, tx, `DELETE FROM mc_work_contact WHERE corp_id=? AND wx_external_userid=? AND name=?`, binding.CorpID, externalWXID, dataset+" 本地验收客户"); err != nil {
		return result, err
	} else {
		result.RemovedParticipants += int(removed)
	}
	if removed, err := deleteFixtureRows(ctx, tx, `DELETE FROM mc_work_employee WHERE corp_id=? AND wx_user_id=? AND name=?`, binding.CorpID, staffWXID, dataset+" 本地验收员工"); err != nil {
		return result, err
	} else {
		result.RemovedParticipants += int(removed)
	}
	if err := tx.Commit(); err != nil {
		return result, err
	}
	removedFiles, err := removeFixtureMediaFiles(storageRoot, mediaIDs)
	result.RemovedFiles = removedFiles
	return result, err
}

func validateFixtureCleanupMessageIDs(dataset string, messageIDs []string) error {
	prefix := strings.TrimSpace(dataset) + "-"
	if !strings.HasPrefix(prefix, archivebridge.FixtureDatasetPrefix) {
		return errors.New("fixture cleanup dataset is invalid")
	}
	for _, messageID := range messageIDs {
		if !strings.HasPrefix(messageID, prefix) {
			return errors.New("fixture cleanup refused because the source contains non-fixture messages")
		}
	}
	return nil
}

func deleteFixtureRows(ctx context.Context, tx *sql.Tx, query string, args ...any) (int64, error) {
	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func removeFixtureMediaFiles(storageRoot string, mediaIDs []string) (int, error) {
	storageRoot = strings.TrimSpace(storageRoot)
	if len(mediaIDs) == 0 {
		return 0, nil
	}
	if storageRoot == "" {
		return 0, errors.New("fixture media storage root is required for cleanup")
	}
	archiveRoot, err := filepath.Abs(filepath.Join(storageRoot, "archive-media"))
	if err != nil {
		return 0, errors.New("fixture media storage root is invalid")
	}
	removed := 0
	for _, rawID := range mediaIDs {
		parsed, err := uuid.Parse(strings.TrimSpace(rawID))
		if err != nil || parsed.String() != strings.ToLower(strings.TrimSpace(rawID)) {
			return removed, errors.New("fixture media ledger contains an unsafe object id")
		}
		paths := []string{filepath.Join(archiveRoot, parsed.String())}
		parts, err := filepath.Glob(filepath.Join(archiveRoot, parsed.String()+".attempt-*.part"))
		if err != nil {
			return removed, err
		}
		paths = append(paths, parts...)
		for _, path := range paths {
			if err := os.Remove(path); err == nil {
				removed++
			} else if !errors.Is(err, os.ErrNotExist) {
				return removed, err
			}
		}
	}
	return removed, nil
}
