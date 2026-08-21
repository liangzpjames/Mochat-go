package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboard"
)

const workMessageExportDirectoryPageSize = 50

func (s *MySQLStore) WorkMessageExportCandidates(ctx context.Context, filter dashboard.WorkMessageExportCandidateFilter) (dashboard.WorkMessageExportCandidatesPage, error) {
	filter.Page = positiveExportPage(filter.Page)
	filter.PageSize = 20
	underlyingPage := ((filter.Page-1)*filter.PageSize)/workMessageExportDirectoryPageSize + 1
	underlyingOffset := ((filter.Page - 1) * filter.PageSize) % workMessageExportDirectoryPageSize

	page := dashboard.WorkMessageExportCandidatesPage{Page: filter.Page, PageSize: filter.PageSize}
	switch filter.ExportType {
	case dashboard.WorkMessageExportTypeEmployee:
		staff, err := s.StaffDirectory(ctx, dashboard.WorkMessageStaffDirectoryFilter{
			TenantID: filter.TenantID, CorpID: filter.CorpID, UserID: filter.UserID,
			Mode: dashboard.WorkMessageStaffModeAll, Keyword: filter.Keyword, DepartmentID: filter.DepartmentID,
			Page: underlyingPage, PageSize: workMessageExportDirectoryPageSize,
			RestrictEmployeeIDs: filter.RestrictEmployeeIDs, EmployeeIDs: append([]int(nil), filter.EmployeeIDs...),
		})
		if err != nil {
			return page, err
		}
		page.Total = staff.Total
		page.Limitations = make([]dashboard.WorkMessageExportLimitation, 0, len(staff.Limitations))
		for _, limitation := range staff.Limitations {
			page.Limitations = append(page.Limitations, dashboard.WorkMessageExportLimitation{Key: limitation.Key, Reason: limitation.Reason})
		}
		page.Capabilities = exportCapabilitiesFromStaff(staff.Capabilities)
		for _, employee := range exportPageWindowStaff(staff.Employees, underlyingOffset, filter.PageSize) {
			page.Items = append(page.Items, dashboard.WorkMessageExportCandidate{
				ID: employee.ID, Name: employee.Name, Avatar: employee.Avatar, Subtitle: "employee",
				ConversationCount: employee.ConversationCount, MessageCount: employee.ConversationCount,
				LastMessageAt: employee.LastConversationAt, Selectable: true,
			})
		}
	case dashboard.WorkMessageExportTypeCustomer:
		customers, err := s.WorkMessageCustomerDirectory(ctx, dashboard.WorkMessageCustomerDirectoryFilter{
			TenantID: filter.TenantID, CorpID: filter.CorpID, UserID: filter.UserID,
			Mode: dashboard.WorkMessageCustomerModeAll, Keyword: filter.Keyword,
			Page: underlyingPage, PageSize: workMessageExportDirectoryPageSize,
			RestrictEmployeeIDs: filter.RestrictEmployeeIDs, EmployeeIDs: append([]int(nil), filter.EmployeeIDs...),
		})
		if err != nil {
			return page, err
		}
		page.Total = customers.Total
		for _, limitation := range customers.Limitations {
			page.Limitations = append(page.Limitations, dashboard.WorkMessageExportLimitation{Key: limitation.Key, Reason: limitation.Reason})
		}
		page.Capabilities = exportCapabilitiesFromCustomer(customers.Capabilities)
		for _, customer := range exportPageWindowCustomer(customers.Customers, underlyingOffset, filter.PageSize) {
			page.Items = append(page.Items, dashboard.WorkMessageExportCandidate{
				ID: customer.ID, Name: customer.Name, Avatar: customer.Avatar, Subtitle: "customer",
				ConversationCount: customer.DirectConversationCount + customer.GroupConversationCount,
				MessageCount:      customer.DirectConversationCount + customer.GroupConversationCount,
				Selectable:        customer.ProfileStatus != "missing",
				Limitation:        exportCustomerLimitation(customer.ProfileStatus),
			})
		}
	case dashboard.WorkMessageExportTypeRoom:
		rooms, err := s.RoomDirectory(ctx, dashboard.WorkMessageRoomDirectoryFilter{
			TenantID: filter.TenantID, CorpID: filter.CorpID, UserID: filter.UserID,
			RoomMode: dashboard.WorkMessageRoomModeActive, Keyword: filter.Keyword,
			Page: underlyingPage, PageSize: workMessageExportDirectoryPageSize,
			RestrictEmployeeIDs: filter.RestrictEmployeeIDs, EmployeeIDs: append([]int(nil), filter.EmployeeIDs...),
		})
		if err != nil {
			return page, err
		}
		page.Total = rooms.Total
		for _, limitation := range rooms.Limitations {
			page.Limitations = append(page.Limitations, dashboard.WorkMessageExportLimitation{Key: limitation.Key, Reason: limitation.Reason})
		}
		page.Capabilities = exportCapabilitiesFromRoom(rooms.Capabilities)
		for _, room := range exportPageWindowRoom(rooms.Items, underlyingOffset, filter.PageSize) {
			page.Items = append(page.Items, dashboard.WorkMessageExportCandidate{
				ID: room.ID, Name: room.Name, Avatar: room.Avatar, ExternalID: room.ExternalID, Subtitle: "room",
				ConversationCount: 1, MessageCount: room.MessageCount, LastMessageAt: room.LastMessageAt,
				Selectable: !room.Dissolved, Limitation: exportRoomLimitation(room.Dissolved),
			})
		}
	}
	return page, nil
}

func (s *MySQLStore) WorkMessageExportTasks(ctx context.Context, query dashboard.WorkMessageExportTaskQuery) (dashboard.WorkMessageExportTaskPage, error) {
	page := positiveExportPage(query.Page)
	pageSize := 20
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mochat_go_work_message_export_tasks
		WHERE tenant_id=? AND corp_id=? AND user_id=?`, query.TenantID, query.CorpID, query.UserID).Scan(&total); err != nil {
		return dashboard.WorkMessageExportTaskPage{}, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, export_type, JSON_LENGTH(selected_objects_json), start_at, end_at,
		file_mode, format, status, estimated_message_count, message_count, file_count, artifact_name, artifact_size,
		error_code, error_message, expires_at, created_at, finished_at
		FROM mochat_go_work_message_export_tasks
		WHERE tenant_id=? AND corp_id=? AND user_id=?
		ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`, query.TenantID, query.CorpID, query.UserID, pageSize, (page-1)*pageSize)
	if err != nil {
		return dashboard.WorkMessageExportTaskPage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.WorkMessageExportTask, 0)
	for rows.Next() {
		item, err := scanWorkMessageExportTask(rows)
		if err != nil {
			return dashboard.WorkMessageExportTaskPage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.WorkMessageExportTaskPage{}, err
	}
	return dashboard.WorkMessageExportTaskPage{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (s *MySQLStore) CreateWorkMessageExportTask(ctx context.Context, input dashboard.WorkMessageExportTaskInput) (dashboard.WorkMessageExportCreateResult, error) {
	request := input.Request
	if existing, ok, err := s.workMessageExportTaskByIdempotency(ctx, input.TenantID, input.CorpID, input.UserID, request.IdempotencyKey); err != nil {
		return dashboard.WorkMessageExportCreateResult{}, err
	} else if ok {
		return dashboard.WorkMessageExportCreateResult{Task: existing, Reused: true}, nil
	}
	estimated, err := s.estimateWorkMessageExportCount(ctx, input)
	if err != nil {
		return dashboard.WorkMessageExportCreateResult{}, err
	}
	if estimated > 100000 {
		return dashboard.WorkMessageExportCreateResult{}, fmt.Errorf("%w：当前预计 %d 条", dashboard.ErrWorkMessageExportMessageLimit, estimated)
	}
	id, err := s.insertWorkMessageExportTask(ctx, input, estimated, time.Now().Add(7*24*time.Hour))
	if err != nil {
		if isMySQLDuplicateKeyError(err) {
			if existing, ok, lookupErr := s.workMessageExportTaskByIdempotency(ctx, input.TenantID, input.CorpID, input.UserID, request.IdempotencyKey); lookupErr == nil && ok {
				return dashboard.WorkMessageExportCreateResult{Task: existing, Reused: true}, nil
			}
		}
		return dashboard.WorkMessageExportCreateResult{}, err
	}
	task, err := s.workMessageExportTaskByID(ctx, input.TenantID, input.CorpID, input.UserID, id)
	if err != nil {
		return dashboard.WorkMessageExportCreateResult{}, err
	}
	return dashboard.WorkMessageExportCreateResult{Task: task}, nil
}

func (s *MySQLStore) insertWorkMessageExportTask(ctx context.Context, input dashboard.WorkMessageExportTaskInput, estimated int, expiresAt time.Time) (int64, error) {
	request := input.Request
	objectsJSON, _ := json.Marshal(uniqueExportIDs(request.ObjectIDs))
	scopesJSON, _ := json.Marshal(uniqueExportStrings(request.ConversationScopes))
	employeeScopeJSON, _ := json.Marshal(uniqueExportIDs(input.EmployeeIDs))
	result, err := s.db.ExecContext(ctx, `INSERT INTO mochat_go_work_message_export_tasks
		(tenant_id, corp_id, user_id, idempotency_key, export_type, selected_objects_json, conversation_scopes_json,
		employee_scope_json, start_at, end_at, file_mode, format, status, estimated_message_count, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending', ?, ?)`,
		input.TenantID, input.CorpID, input.UserID, request.IdempotencyKey, request.ExportType, objectsJSON, scopesJSON,
		employeeScopeJSON, request.StartAt, request.EndAt, request.FileMode, request.Format, estimated, expiresAt)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (s *MySQLStore) WorkMessageExportArtifact(ctx context.Context, tenantID, corpID, userID int, taskID int64) (dashboard.WorkMessageExportArtifact, error) {
	var status, path, name string
	var expiresAt sql.NullTime
	err := s.db.QueryRowContext(ctx, `SELECT status, artifact_path, artifact_name, expires_at
		FROM mochat_go_work_message_export_tasks WHERE id=? AND tenant_id=? AND corp_id=? AND user_id=?`, taskID, tenantID, corpID, userID).Scan(&status, &path, &name, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.WorkMessageExportArtifact{}, dashboard.ErrWorkMessageExportTaskNotFound
	}
	if err != nil {
		return dashboard.WorkMessageExportArtifact{}, err
	}
	if status != "completed" || strings.TrimSpace(path) == "" {
		return dashboard.WorkMessageExportArtifact{}, dashboard.ErrWorkMessageExportTaskNotFound
	}
	if expiresAt.Valid && time.Now().After(expiresAt.Time) {
		return dashboard.WorkMessageExportArtifact{}, dashboard.ErrWorkMessageExportTaskExpired
	}
	cleanPath := filepath.Clean(path)
	if cleanPath == "." || !filepath.IsAbs(cleanPath) {
		return dashboard.WorkMessageExportArtifact{}, errors.New("导出文件路径不安全")
	}
	if _, err := os.Stat(cleanPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return dashboard.WorkMessageExportArtifact{}, dashboard.ErrWorkMessageExportTaskNotFound
		}
		return dashboard.WorkMessageExportArtifact{}, err
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE mochat_go_work_message_export_tasks SET download_count=download_count+1, last_downloaded_at=NOW() WHERE id=?`, taskID); err != nil {
		return dashboard.WorkMessageExportArtifact{}, err
	}
	return dashboard.WorkMessageExportArtifact{Path: cleanPath, Filename: filepath.Base(name), ContentType: "application/zip"}, nil
}

func (s *MySQLStore) workMessageExportTaskByIdempotency(ctx context.Context, tenantID, corpID, userID int, key string) (dashboard.WorkMessageExportTask, bool, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM mochat_go_work_message_export_tasks WHERE tenant_id=? AND corp_id=? AND user_id=? AND idempotency_key=?`, tenantID, corpID, userID, key).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.WorkMessageExportTask{}, false, nil
	}
	if err != nil {
		return dashboard.WorkMessageExportTask{}, false, err
	}
	task, err := s.workMessageExportTaskByID(ctx, tenantID, corpID, userID, id)
	return task, err == nil, err
}

func (s *MySQLStore) workMessageExportTaskByID(ctx context.Context, tenantID, corpID, userID int, id int64) (dashboard.WorkMessageExportTask, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, export_type, JSON_LENGTH(selected_objects_json), start_at, end_at,
		file_mode, format, status, estimated_message_count, message_count, file_count, artifact_name, artifact_size,
		error_code, error_message, expires_at, created_at, finished_at
		FROM mochat_go_work_message_export_tasks WHERE id=? AND tenant_id=? AND corp_id=? AND user_id=?`, id, tenantID, corpID, userID)
	return scanWorkMessageExportTask(row)
}

type workMessageExportScanner interface {
	Scan(dest ...any) error
}

func scanWorkMessageExportTask(scanner workMessageExportScanner) (dashboard.WorkMessageExportTask, error) {
	var item dashboard.WorkMessageExportTask
	var finishedAt, expiresAt, createdAt, startAt, endAt sql.NullTime
	if err := scanner.Scan(&item.ID, &item.ExportType, &item.ObjectCount, &startAt, &endAt, &item.FileMode, &item.Format,
		&item.Status, &item.EstimatedMessageCount, &item.MessageCount, &item.FileCount, &item.ArtifactName, &item.ArtifactSize,
		&item.ErrorCode, &item.ErrorMessage, &expiresAt, &createdAt, &finishedAt); err != nil {
		return dashboard.WorkMessageExportTask{}, err
	}
	if startAt.Valid {
		item.StartAt = startAt.Time
	}
	if endAt.Valid {
		item.EndAt = endAt.Time
	}
	if expiresAt.Valid {
		item.ExpiresAt = expiresAt.Time
	}
	if createdAt.Valid {
		item.CreatedAt = createdAt.Time
	}
	if finishedAt.Valid {
		item.FinishedAt = finishedAt.Time
	}
	return item, nil
}

func (s *MySQLStore) estimateWorkMessageExportCount(ctx context.Context, input dashboard.WorkMessageExportTaskInput) (int, error) {
	state, err := s.archiveSourceRegistryState(ctx)
	if err != nil {
		return 0, err
	}
	archiveFilter := dashboard.WorkMessageUserFilter{
		TenantID: input.TenantID, UserID: input.UserID, CorpID: input.CorpID, ToUserType: -1,
		DateTimeStart: input.Request.StartAt.Format("2006-01-02 15:04:05"), DateTimeEnd: input.Request.EndAt.Format("2006-01-02 15:04:05"),
		RestrictEmployeeIDs: input.RestrictEmployeeIDs, EmployeeIDs: append([]int(nil), input.EmployeeIDs...), AllowAllEmployees: !input.RestrictEmployeeIDs,
	}
	sourceSQL, sourceArgs, ok := workMessageFilteredUnionSQLWithArchiveSourceState(archiveFilter, state)
	if !ok {
		return 0, nil
	}
	where, args := workMessageExportObjectWhere(input.Request, input.Request.ObjectIDs)
	queryArgs := append(append([]any{}, sourceArgs...), args...)
	var count int
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM (`+sourceSQL+`) export_messages WHERE `+where, queryArgs...).Scan(&count)
	return count, err
}

func workMessageExportObjectWhere(request dashboard.WorkMessageExportCreateRequest, objectIDs []int) (string, []any) {
	ids := uniqueExportIDs(objectIDs)
	placeholders := exportPlaceholders(len(ids))
	args := make([]any, 0, len(ids)*2)
	scopePredicate := exportConversationScopePredicate(request.ConversationScopes)
	var predicate string
	switch dashboard.WorkMessageExportType(request.ExportType) {
	case dashboard.WorkMessageExportTypeEmployee:
		predicate = "work_employee_id IN (" + placeholders + ")" + scopePredicate
		for _, id := range ids {
			args = append(args, id)
		}
	case dashboard.WorkMessageExportTypeRoom:
		predicate = "to_user_type=2 AND to_user_id IN (" + placeholders + ") AND EXISTS (SELECT 1 FROM mc_work_contact_room export_membership WHERE export_membership.room_id=export_messages.to_user_id AND export_membership.deleted_at IS NULL)" + scopePredicate
		for _, id := range ids {
			args = append(args, id)
		}
	case dashboard.WorkMessageExportTypeCustomer:
		branches := make([]string, 0, 2)
		if exportConversationScopeIncludes(request.ConversationScopes, "customer_direct") {
			branches = append(branches, "(to_user_type=1 AND to_user_id IN ("+placeholders+"))")
			for _, id := range ids {
				args = append(args, id)
			}
		}
		if exportConversationScopeIncludes(request.ConversationScopes, "external_group") {
			branches = append(branches, "(to_user_type=2 AND EXISTS (SELECT 1 FROM mc_work_contact_room export_membership WHERE export_membership.room_id=export_messages.to_user_id AND export_membership.contact_id IN ("+placeholders+") AND export_membership.deleted_at IS NULL))")
			for _, id := range ids {
				args = append(args, id)
			}
		}
		if len(branches) == 0 {
			predicate = "1=0"
		} else {
			predicate = "(" + strings.Join(branches, " OR ") + ")"
		}
	default:
		predicate = "1=0"
	}
	return predicate, args
}

func exportConversationScopeIncludes(scopes []string, target string) bool {
	if len(scopes) == 0 {
		return true
	}
	for _, scope := range scopes {
		if strings.TrimSpace(scope) == target {
			return true
		}
	}
	return false
}

func exportConversationScopePredicate(scopes []string) string {
	includesDirect := exportConversationScopeIncludes(scopes, "customer_direct")
	includesGroup := exportConversationScopeIncludes(scopes, "external_group")
	switch {
	case includesDirect && includesGroup:
		return ""
	case includesDirect:
		return " AND to_user_type=1"
	case includesGroup:
		return " AND to_user_type=2"
	default:
		return " AND 1=0"
	}
}

func positiveExportPage(page int) int {
	if page < 1 {
		return 1
	}
	return page
}
func exportPlaceholders(n int) string {
	if n <= 0 {
		return "NULL"
	}
	return strings.TrimRight(strings.Repeat("?,", n), ",")
}
func uniqueExportIDs(values []int) []int {
	result := []int{}
	seen := map[int]struct{}{}
	for _, value := range values {
		if value > 0 {
			if _, ok := seen[value]; !ok {
				seen[value] = struct{}{}
				result = append(result, value)
			}
		}
	}
	return result
}
func uniqueExportStrings(values []string) []string {
	result := []string{}
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			if _, ok := seen[value]; !ok {
				seen[value] = struct{}{}
				result = append(result, value)
			}
		}
	}
	return result
}

func exportPageWindowStaff(items []dashboard.WorkMessageStaffEmployee, offset, size int) []dashboard.WorkMessageStaffEmployee {
	if offset >= len(items) {
		return []dashboard.WorkMessageStaffEmployee{}
	}
	end := offset + size
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end]
}
func exportPageWindowCustomer(items []dashboard.WorkMessageCustomerDirectoryItem, offset, size int) []dashboard.WorkMessageCustomerDirectoryItem {
	if offset >= len(items) {
		return []dashboard.WorkMessageCustomerDirectoryItem{}
	}
	end := offset + size
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end]
}
func exportPageWindowRoom(items []dashboard.WorkMessageRoomDirectoryItem, offset, size int) []dashboard.WorkMessageRoomDirectoryItem {
	if offset >= len(items) {
		return []dashboard.WorkMessageRoomDirectoryItem{}
	}
	end := offset + size
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end]
}
func exportCustomerLimitation(status string) string {
	if status == "missing" {
		return "客户资料缺失，仅能依据归档记录导出"
	}
	return ""
}
func exportRoomLimitation(dissolved bool) string {
	if dissolved {
		return "群聊已解散，不能导出"
	}
	return ""
}
func exportCapabilitiesFromStaff(values []dashboard.WorkMessageCapability) []dashboard.WorkMessageExportCapability {
	return convertExportCapabilities(values)
}
func exportCapabilitiesFromCustomer(values []dashboard.WorkMessageCapability) []dashboard.WorkMessageExportCapability {
	return convertExportCapabilities(values)
}
func exportCapabilitiesFromRoom(values []dashboard.WorkMessageCapability) []dashboard.WorkMessageExportCapability {
	return convertExportCapabilities(values)
}
func convertExportCapabilities(values []dashboard.WorkMessageCapability) []dashboard.WorkMessageExportCapability {
	result := make([]dashboard.WorkMessageExportCapability, 0, len(values))
	for _, value := range values {
		result = append(result, dashboard.WorkMessageExportCapability{Key: value.Key, Available: value.Available, Reason: value.Reason})
	}
	return result
}
