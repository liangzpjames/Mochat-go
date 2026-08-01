package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboard"
)

type sensitiveWordState struct {
	ID        int
	GroupID   int
	Name      string
	Status    int
	UpdatedAt time.Time
}

func (s *MySQLStore) SensitiveWordMutationReplay(ctx context.Context, mutation dashboard.SensitiveWordMutation) (dashboard.SensitiveWordMutationResult, bool, error) {
	mutation = normalizeSensitiveWordMutation(mutation)
	return sensitiveWordIdempotentResult(ctx, s.db.QueryRowContext, mutation)
}

func (s *MySQLStore) MutateSensitiveWords(ctx context.Context, mutation dashboard.SensitiveWordMutation) (dashboard.SensitiveWordMutationResult, error) {
	mutation = normalizeSensitiveWordMutation(mutation)
	if mutation.TenantID <= 0 || mutation.CorpID <= 0 || mutation.ActorUserID <= 0 || mutation.Version == "" || mutation.IdempotencyKey == "" {
		return dashboard.SensitiveWordMutationResult{}, sensitiveWordOperationError(http.StatusBadRequest, "敏感词操作参数不完整")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SensitiveWordMutationResult{}, err
	}
	defer tx.Rollback()

	if err := sensitiveWordLockTenantTx(ctx, tx, mutation.TenantID, mutation.CorpID); err != nil {
		return dashboard.SensitiveWordMutationResult{}, err
	}
	if previous, found, err := sensitiveWordIdempotentResultTx(ctx, tx, mutation); err != nil {
		return dashboard.SensitiveWordMutationResult{}, err
	} else if found {
		previous.Idempotent = true
		return previous, nil
	}

	before := map[string]any{}
	after := map[string]any{}
	targetType := "sensitive_word"
	targetID := strconv.Itoa(mutation.WordID)
	targetName := mutation.Name
	result := dashboard.SensitiveWordMutationResult{}

	switch mutation.Action {
	case dashboard.SensitiveWordMutationCreateWords:
		if mutation.Version != "0" || mutation.GroupID <= 0 || len(mutation.Names) == 0 {
			return dashboard.SensitiveWordMutationResult{}, sensitiveWordOperationError(http.StatusBadRequest, "新增敏感词参数错误")
		}
		if err := sensitiveWordEnforceCreateQuotaTx(ctx, tx, mutation.TenantID, int64(len(mutation.Names))); err != nil {
			return dashboard.SensitiveWordMutationResult{}, err
		}
		if _, err := sensitiveWordGroupStateTx(ctx, tx, mutation.CorpID, mutation.GroupID, true); err != nil {
			return dashboard.SensitiveWordMutationResult{}, err
		}
		created := make([]string, 0, len(mutation.Names))
		for _, raw := range mutation.Names {
			name := strings.TrimSpace(raw)
			if name == "" {
				continue
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO mc_sensitive_word (corp_id, group_id, name, status, created_at, updated_at)
				VALUES (?, ?, ?, 1, NOW(), NOW())
			`, mutation.CorpID, mutation.GroupID, name); err != nil {
				return dashboard.SensitiveWordMutationResult{}, err
			}
			created = append(created, name)
		}
		if len(created) == 0 {
			return dashboard.SensitiveWordMutationResult{}, sensitiveWordOperationError(http.StatusBadRequest, "敏感词名称不能为空")
		}
		targetType, targetID, targetName = "sensitive_word_batch", strconv.Itoa(mutation.GroupID), strings.Join(created, ",")
		after = map[string]any{"groupId": mutation.GroupID, "names": created, "version": "0"}
		result.Version = "0"

	case dashboard.SensitiveWordMutationCreateGroup:
		if mutation.Version != "0" || len(mutation.Names) == 0 {
			return dashboard.SensitiveWordMutationResult{}, sensitiveWordOperationError(http.StatusBadRequest, "新增词组参数错误")
		}
		created := make([]string, 0, len(mutation.Names))
		for _, raw := range mutation.Names {
			name := strings.TrimSpace(raw)
			if name == "" {
				continue
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO mc_sensitive_word_group (corp_id, name, created_at, updated_at)
				VALUES (?, ?, NOW(), NOW())
			`, mutation.CorpID, name); err != nil {
				return dashboard.SensitiveWordMutationResult{}, err
			}
			created = append(created, name)
		}
		if len(created) == 0 {
			return dashboard.SensitiveWordMutationResult{}, sensitiveWordOperationError(http.StatusBadRequest, "词组名称不能为空")
		}
		targetType, targetID, targetName = "sensitive_word_group_batch", "0", strings.Join(created, ",")
		after = map[string]any{"names": created, "version": "0"}
		result.Version = "0"

	case dashboard.SensitiveWordMutationSetStatus, dashboard.SensitiveWordMutationMoveWord, dashboard.SensitiveWordMutationDeleteWord:
		state, err := sensitiveWordStateTx(ctx, tx, mutation.CorpID, mutation.WordID, true)
		if err != nil {
			return dashboard.SensitiveWordMutationResult{}, err
		}
		currentVersion := sensitiveWordVersion(state)
		if mutation.Version != currentVersion {
			return dashboard.SensitiveWordMutationResult{}, dashboard.NewSensitiveWordConflict("敏感词版本已变化，请刷新后重试")
		}
		before = sensitiveWordStatePayload(state, currentVersion)
		targetID, targetName = strconv.Itoa(state.ID), state.Name
		switch mutation.Action {
		case dashboard.SensitiveWordMutationSetStatus:
			if mutation.Status != 1 && mutation.Status != 2 {
				return dashboard.SensitiveWordMutationResult{}, sensitiveWordOperationError(http.StatusBadRequest, "敏感词状态必须为 1 或 2")
			}
			if _, err := tx.ExecContext(ctx, `UPDATE mc_sensitive_word SET status = ?, updated_at = NOW() WHERE id = ? AND corp_id = ? AND deleted_at IS NULL`, mutation.Status, state.ID, mutation.CorpID); err != nil {
				return dashboard.SensitiveWordMutationResult{}, err
			}
			state.Status = mutation.Status
		case dashboard.SensitiveWordMutationMoveWord:
			if mutation.GroupID <= 0 {
				return dashboard.SensitiveWordMutationResult{}, sensitiveWordOperationError(http.StatusBadRequest, "目标词组不能为空")
			}
			if _, err := sensitiveWordGroupStateTx(ctx, tx, mutation.CorpID, mutation.GroupID, true); err != nil {
				return dashboard.SensitiveWordMutationResult{}, err
			}
			if _, err := tx.ExecContext(ctx, `UPDATE mc_sensitive_word SET group_id = ?, updated_at = NOW() WHERE id = ? AND corp_id = ? AND deleted_at IS NULL`, mutation.GroupID, state.ID, mutation.CorpID); err != nil {
				return dashboard.SensitiveWordMutationResult{}, err
			}
			state.GroupID = mutation.GroupID
		case dashboard.SensitiveWordMutationDeleteWord:
			if _, err := tx.ExecContext(ctx, `UPDATE mc_sensitive_word SET deleted_at = NOW(), updated_at = NOW() WHERE id = ? AND corp_id = ? AND deleted_at IS NULL`, state.ID, mutation.CorpID); err != nil {
				return dashboard.SensitiveWordMutationResult{}, err
			}
		}
		state, err = sensitiveWordStateAfterMutationTx(ctx, tx, mutation.CorpID, state.ID)
		if err != nil {
			return dashboard.SensitiveWordMutationResult{}, err
		}
		result.Version = sensitiveWordVersion(state)
		after = sensitiveWordStatePayload(state, result.Version)
		if mutation.Action == dashboard.SensitiveWordMutationDeleteWord {
			after["deleted"] = true
		}

	case dashboard.SensitiveWordMutationRenameGroup:
		if mutation.GroupID <= 0 || strings.TrimSpace(mutation.Name) == "" {
			return dashboard.SensitiveWordMutationResult{}, sensitiveWordOperationError(http.StatusBadRequest, "词组改名参数错误")
		}
		state, err := sensitiveWordGroupStateTx(ctx, tx, mutation.CorpID, mutation.GroupID, true)
		if err != nil {
			return dashboard.SensitiveWordMutationResult{}, err
		}
		currentVersion := sensitiveWordGroupVersion(state)
		if mutation.Version != currentVersion {
			return dashboard.SensitiveWordMutationResult{}, dashboard.NewSensitiveWordConflict("敏感词组版本已变化，请刷新后重试")
		}
		before = map[string]any{"id": state.ID, "name": state.Name, "version": currentVersion}
		if _, err := tx.ExecContext(ctx, `UPDATE mc_sensitive_word_group SET name = ?, updated_at = NOW() WHERE id = ? AND corp_id = ? AND deleted_at IS NULL`, strings.TrimSpace(mutation.Name), state.ID, mutation.CorpID); err != nil {
			return dashboard.SensitiveWordMutationResult{}, err
		}
		state, err = sensitiveWordGroupStateTx(ctx, tx, mutation.CorpID, mutation.GroupID, true)
		if err != nil {
			return dashboard.SensitiveWordMutationResult{}, err
		}
		result.Version = sensitiveWordGroupVersion(state)
		after = map[string]any{"id": state.ID, "name": state.Name, "version": result.Version}
		targetType, targetID, targetName = "sensitive_word_group", strconv.Itoa(state.ID), state.Name

	default:
		return dashboard.SensitiveWordMutationResult{}, sensitiveWordOperationError(http.StatusBadRequest, "未知敏感词操作")
	}

	after["requestFingerprint"] = sensitiveWordMutationFingerprint(mutation)
	beforeJSON, _ := json.Marshal(before)
	afterJSON, _ := json.Marshal(after)
	if _, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: mutation.TenantID, ActorUserID: mutation.ActorUserID, ActorTenantID: mutation.TenantID,
		Action: "sensitive_word." + mutation.Action, TargetType: targetType, TargetID: targetID, TargetName: targetName,
		BeforeJSON: string(beforeJSON), AfterJSON: string(afterJSON), Remark: sensitiveWordAuditRemark(mutation),
	}); err != nil {
		return dashboard.SensitiveWordMutationResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SensitiveWordMutationResult{}, err
	}
	return result, nil
}

func sensitiveWordIdempotentResultTx(ctx context.Context, tx *sql.Tx, mutation dashboard.SensitiveWordMutation) (dashboard.SensitiveWordMutationResult, bool, error) {
	return sensitiveWordIdempotentResult(ctx, tx.QueryRowContext, mutation)
}

func sensitiveWordIdempotentResult(ctx context.Context, queryRow func(context.Context, string, ...any) *sql.Row, mutation dashboard.SensitiveWordMutation) (dashboard.SensitiveWordMutationResult, bool, error) {
	var raw sql.NullString
	err := queryRow(ctx, `
		SELECT after_json
		FROM mochat_go_saas_admin_operation_logs
		WHERE tenant_id = ? AND actor_user_id = ? AND action = ? AND remark = ? AND deleted_at IS NULL
		ORDER BY id DESC LIMIT 1
	`, mutation.TenantID, mutation.ActorUserID, "sensitive_word."+mutation.Action, sensitiveWordAuditRemark(mutation)).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SensitiveWordMutationResult{}, false, nil
	}
	if err != nil {
		return dashboard.SensitiveWordMutationResult{}, false, err
	}
	var payload struct {
		Version            string `json:"version"`
		RequestFingerprint string `json:"requestFingerprint"`
	}
	if err := json.Unmarshal([]byte(raw.String), &payload); err != nil {
		return dashboard.SensitiveWordMutationResult{}, false, err
	}
	if payload.RequestFingerprint == "" || payload.RequestFingerprint != sensitiveWordMutationFingerprint(mutation) {
		return dashboard.SensitiveWordMutationResult{}, false, dashboard.NewSensitiveWordConflict("幂等键已用于不同的敏感词请求")
	}
	return dashboard.SensitiveWordMutationResult{Version: payload.Version}, true, nil
}

func normalizeSensitiveWordMutation(mutation dashboard.SensitiveWordMutation) dashboard.SensitiveWordMutation {
	mutation.Action = strings.TrimSpace(mutation.Action)
	mutation.Version = strings.TrimSpace(mutation.Version)
	mutation.IdempotencyKey = strings.TrimSpace(mutation.IdempotencyKey)
	mutation.Name = strings.TrimSpace(mutation.Name)
	seen := make(map[string]struct{}, len(mutation.Names))
	names := make([]string, 0, len(mutation.Names))
	for _, raw := range mutation.Names {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	mutation.Names = names
	return mutation
}

func sensitiveWordMutationFingerprint(mutation dashboard.SensitiveWordMutation) string {
	names := append([]string(nil), mutation.Names...)
	sort.Strings(names)
	payload := struct {
		Action  string   `json:"action"`
		CorpID  int      `json:"corpId"`
		WordID  int      `json:"wordId"`
		GroupID int      `json:"groupId"`
		Status  int      `json:"status"`
		Name    string   `json:"name"`
		Names   []string `json:"names"`
		Version string   `json:"version"`
	}{
		Action: mutation.Action, CorpID: mutation.CorpID, WordID: mutation.WordID, GroupID: mutation.GroupID,
		Status: mutation.Status, Name: mutation.Name, Names: names, Version: mutation.Version,
	}
	encoded, _ := json.Marshal(payload)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func sensitiveWordAuditRemark(mutation dashboard.SensitiveWordMutation) string {
	return fmt.Sprintf("corpId=%d;idempotencyKey=%s", mutation.CorpID, mutation.IdempotencyKey)
}

func sensitiveWordStateTx(ctx context.Context, tx *sql.Tx, corpID, wordID int, forUpdate bool) (sensitiveWordState, error) {
	query := `SELECT id, group_id, name, status, COALESCE(updated_at, created_at, NOW()) FROM mc_sensitive_word WHERE id = ? AND corp_id = ? AND deleted_at IS NULL`
	if forUpdate {
		query += " FOR UPDATE"
	}
	var state sensitiveWordState
	err := tx.QueryRowContext(ctx, query, wordID, corpID).Scan(&state.ID, &state.GroupID, &state.Name, &state.Status, &state.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return sensitiveWordState{}, sensitiveWordOperationError(http.StatusNotFound, "敏感词不存在")
	}
	return state, err
}

func sensitiveWordStateAfterMutationTx(ctx context.Context, tx *sql.Tx, corpID, wordID int) (sensitiveWordState, error) {
	var state sensitiveWordState
	err := tx.QueryRowContext(ctx, `
		SELECT id, group_id, name, status, COALESCE(updated_at, created_at, NOW())
		FROM mc_sensitive_word
		WHERE id = ? AND corp_id = ?
		FOR UPDATE
	`, wordID, corpID).Scan(&state.ID, &state.GroupID, &state.Name, &state.Status, &state.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return sensitiveWordState{}, sensitiveWordOperationError(http.StatusNotFound, "敏感词不存在")
	}
	return state, err
}

func sensitiveWordLockTenantTx(ctx context.Context, tx *sql.Tx, tenantID, corpID int) error {
	var actualTenantID int
	err := tx.QueryRowContext(ctx, `
		SELECT tenant_id
		FROM mc_corp
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
		FOR UPDATE
	`, corpID).Scan(&actualTenantID)
	if errors.Is(err, sql.ErrNoRows) {
		return sensitiveWordOperationError(http.StatusNotFound, "企业不存在")
	}
	if err != nil {
		return err
	}
	if actualTenantID != tenantID {
		return sensitiveWordOperationError(http.StatusForbidden, "企业不属于当前租户")
	}
	var lockedTenantID int
	err = tx.QueryRowContext(ctx, `
		SELECT id
		FROM mc_tenant
		WHERE id = ? AND deleted_at IS NULL
		FOR UPDATE
	`, tenantID).Scan(&lockedTenantID)
	if errors.Is(err, sql.ErrNoRows) {
		return sensitiveWordOperationError(http.StatusForbidden, "租户不存在或不可用")
	}
	return err
}

func sensitiveWordEnforceCreateQuotaTx(ctx context.Context, tx *sql.Tx, tenantID int, additional int64) error {
	if additional <= 0 {
		return nil
	}
	var current int64
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM mc_sensitive_word word
		JOIN mc_corp corp ON corp.id = word.corp_id
		WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND word.deleted_at IS NULL
	`, tenantID).Scan(&current); err != nil {
		return err
	}
	limit, err := sensitiveWordQuotaLimitTx(ctx, tx, tenantID)
	if err != nil {
		return err
	}
	status := dashboard.SaaSQuotaStatus{
		Metric: dashboard.SaaSMetricSensitiveWords, TenantID: tenantID,
		Current: current, Limit: limit, Additional: additional,
	}
	if status.Exceeded() {
		return dashboard.NewSaaSQuotaExceededError(status)
	}
	return nil
}

func sensitiveWordQuotaLimitTx(ctx context.Context, tx *sql.Tx, tenantID int) (int64, error) {
	var rawLimits sql.NullString
	var packageLimit int64
	err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(CAST(tp.limits_json AS CHAR), ''), COALESCE(pkg.sensitive_words, 0)
		FROM mochat_go_saas_tenant_packages tp
		LEFT JOIN mochat_go_saas_packages pkg ON pkg.code = tp.package_code AND pkg.deleted_at IS NULL
		WHERE tp.tenant_id = ? AND tp.status = 1 AND tp.deleted_at IS NULL
		LIMIT 1
	`, tenantID).Scan(&rawLimits, &packageLimit)
	if err == nil {
		if raw := strings.TrimSpace(rawLimits.String); raw != "" && raw != "null" {
			limits := saasUsageLimitSnapshot{SensitiveWords: packageLimit}
			if json.Unmarshal([]byte(raw), &limits) == nil {
				return limits.SensitiveWords, nil
			}
		}
		return packageLimit, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	var counterLimit int64
	err = tx.QueryRowContext(ctx, `
		SELECT limit_value
		FROM mochat_go_saas_usage_counters
		WHERE tenant_id = ? AND metric = ? AND period_key = 'lifetime' AND deleted_at IS NULL
		LIMIT 1
	`, tenantID, dashboard.SaaSMetricSensitiveWords).Scan(&counterLimit)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return counterLimit, err
}

func sensitiveWordGroupStateTx(ctx context.Context, tx *sql.Tx, corpID, groupID int, forUpdate bool) (sensitiveWordState, error) {
	query := `SELECT id, name, COALESCE(updated_at, created_at, NOW()) FROM mc_sensitive_word_group WHERE id = ? AND corp_id = ? AND deleted_at IS NULL`
	if forUpdate {
		query += " FOR UPDATE"
	}
	var state sensitiveWordState
	err := tx.QueryRowContext(ctx, query, groupID, corpID).Scan(&state.ID, &state.Name, &state.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return sensitiveWordState{}, sensitiveWordOperationError(http.StatusNotFound, "敏感词组不存在")
	}
	return state, err
}

func sensitiveWordVersion(state sensitiveWordState) string {
	return sensitiveWordVersionValue("word", state.ID, state.GroupID, state.Status, state.Name, state.UpdatedAt)
}

func sensitiveWordGroupVersion(state sensitiveWordState) string {
	return sensitiveWordVersionValue("group", state.ID, 0, 0, state.Name, state.UpdatedAt)
}

func sensitiveWordVersionValue(kind string, id, groupID, status int, name string, updatedAt time.Time) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%d:%d:%s:%d", kind, id, groupID, status, name, updatedAt.UnixNano())))
	return hex.EncodeToString(sum[:8])
}

func sensitiveWordStatePayload(state sensitiveWordState, version string) map[string]any {
	return map[string]any{"id": state.ID, "groupId": state.GroupID, "name": state.Name, "status": state.Status, "version": version}
}

func sensitiveWordOperationError(status int, message string) error {
	return &dashboard.SensitiveWordOperationError{Status: status, Message: message}
}
