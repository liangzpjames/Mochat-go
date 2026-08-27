package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"jiyi/mochat-go/internal/dashboard"
)

type workMessageMediaProjection struct {
	MsgID          string
	SourceIdentity string
	Content        *any
}

type workMessageMediaRow struct {
	ID, MsgID, SourceIdentity, MediaType, Name, MIMEType, Status, ErrorCode string
	Size                                                                    int64
}

func (s *MySQLStore) projectWorkMessageMedia(ctx context.Context, tenantID, corpID int, refs []workMessageMediaProjection) error {
	if tenantID <= 0 || corpID <= 0 || len(refs) == 0 {
		return nil
	}
	msgIDs := make([]string, 0, len(refs))
	seen := map[string]bool{}
	for _, ref := range refs {
		msgID := strings.TrimSpace(ref.MsgID)
		if msgID != "" && ref.Content != nil && !seen[msgID] {
			seen[msgID] = true
			msgIDs = append(msgIDs, msgID)
		}
	}
	if len(msgIDs) == 0 {
		return nil
	}
	args := []any{tenantID, corpID}
	for _, msgID := range msgIDs {
		args = append(args, msgID)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT media.id,media.msgid,media.source_identity,media.media_type,media.media_name,media.mime_type,
		       CASE WHEN media.status='ready' THEN media.bytes_received ELSE media.expected_size_bytes END AS size_bytes,
		       media.status,media.last_error_code
		FROM mochat_go_archive_media_objects media
		INNER JOIN mochat_go_archive_message_sources source
		  ON source.tenant_id=media.tenant_id AND source.corp_id=media.corp_id AND source.msgid=media.msgid
		 AND source.source_id=media.source_identity
		WHERE media.tenant_id=? AND media.corp_id=? AND media.msgid IN (`+placeholders(len(msgIDs))+`)
		ORDER BY media.msgid,media.id
	`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	byMessage := map[string][]workMessageMediaRow{}
	for rows.Next() {
		var row workMessageMediaRow
		if err := rows.Scan(&row.ID, &row.MsgID, &row.SourceIdentity, &row.MediaType, &row.Name, &row.MIMEType, &row.Size, &row.Status, &row.ErrorCode); err != nil {
			return err
		}
		byMessage[row.MsgID] = append(byMessage[row.MsgID], row)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, ref := range refs {
		mediaItems := make([]map[string]any, 0, len(byMessage[strings.TrimSpace(ref.MsgID)]))
		for _, row := range byMessage[strings.TrimSpace(ref.MsgID)] {
			if !archiveMediaSourceMatches(ref.SourceIdentity, row.SourceIdentity) {
				continue
			}
			mediaItems = append(mediaItems, archiveMediaPayload(row))
		}
		if len(mediaItems) > 0 {
			content := workMessageMediaContent(*ref.Content)
			content["media"] = mediaItems[0]
			content["mediaItems"] = mediaItems
			*ref.Content = content
		}
	}
	return nil
}

func (s *MySQLStore) ProjectWorkMessageStaffMedia(ctx context.Context, tenantID, corpID int, detail *dashboard.WorkMessageStaffDetail) error {
	if detail == nil {
		return nil
	}
	refs := make([]workMessageMediaProjection, 0, len(detail.Messages))
	for index := range detail.Messages {
		refs = append(refs, workMessageMediaProjection{
			MsgID: archiveMessageMsgID(detail.Messages[index].ID), SourceIdentity: detail.Messages[index].ArchiveSourceID,
			Content: &detail.Messages[index].Content,
		})
	}
	return s.projectWorkMessageMedia(ctx, tenantID, corpID, refs)
}

func (s *MySQLStore) ProjectWorkMessageRoomMedia(ctx context.Context, tenantID, corpID int, page *dashboard.WorkMessageRoomMessages) error {
	if page == nil {
		return nil
	}
	refs := make([]workMessageMediaProjection, 0, len(page.Messages))
	for index := range page.Messages {
		refs = append(refs, workMessageMediaProjection{
			MsgID: archiveMessageMsgID(page.Messages[index].ID), SourceIdentity: page.Messages[index].ArchiveSourceID,
			Content: &page.Messages[index].Content,
		})
	}
	return s.projectWorkMessageMedia(ctx, tenantID, corpID, refs)
}

func (s *MySQLStore) ProjectWorkMessagePageMedia(ctx context.Context, tenantID, corpID int, page *dashboard.WorkMessagePage) error {
	if page == nil {
		return nil
	}
	contents := make([]any, len(page.Items))
	refs := make([]workMessageMediaProjection, 0, len(page.Items))
	for index := range page.Items {
		contents[index] = staffMessageContent(page.Items[index].ContentRaw)
		refs = append(refs, workMessageMediaProjection{
			MsgID: page.Items[index].MsgID, SourceIdentity: page.Items[index].ArchiveSourceID, Content: &contents[index],
		})
	}
	if err := s.projectWorkMessageMedia(ctx, tenantID, corpID, refs); err != nil {
		return err
	}
	for index := range page.Items {
		if content, ok := contents[index].(map[string]any); ok {
			if media, exists := content["media"].(map[string]any); exists {
				page.Items[index].Media = media
			}
			page.Items[index].ContentRaw = workMessageMediaContentJSON(content, page.Items[index].ContentRaw)
		}
	}
	return nil
}

func workMessageMediaContentJSON(content map[string]any, fallback string) string {
	encoded, err := json.Marshal(content)
	if err != nil {
		return fallback
	}
	return string(encoded)
}

func archiveMessageMsgID(id string) string {
	id = strings.TrimSpace(id)
	if strings.HasPrefix(id, "msg:") {
		return strings.TrimPrefix(id, "msg:")
	}
	if strings.HasPrefix(id, "seq:") || strings.HasPrefix(id, "table:") {
		return ""
	}
	return id
}

func archiveMediaSourceMatches(expected, actual string) bool {
	expected, actual = strings.TrimSpace(expected), strings.TrimSpace(actual)
	return expected == actual || (expected == "wecom" && strings.HasPrefix(actual, "wecom:"))
}

func workMessageMediaContent(value any) map[string]any {
	if object, ok := value.(map[string]any); ok {
		return object
	}
	return map[string]any{"value": value}
}

func archiveMediaPayload(row workMessageMediaRow) map[string]any {
	result := map[string]any{
		"id": row.ID, "type": row.MediaType, "name": row.Name, "mimeType": row.MIMEType,
		"size": row.Size, "status": row.Status,
	}
	if row.Status == "ready" {
		result["url"] = "/dashboard/archive/media/" + row.ID + "/content"
	}
	if row.ErrorCode != "" {
		result["errorCode"] = row.ErrorCode
	}
	return result
}

func (s *MySQLStore) ArchiveMediaContent(ctx context.Context, filter dashboard.ArchiveMediaContentFilter) (dashboard.ArchiveMediaContentObject, bool, error) {
	if s == nil || s.db == nil || filter.TenantID <= 0 || filter.CorpID <= 0 || strings.TrimSpace(filter.ID) == "" {
		return dashboard.ArchiveMediaContentObject{}, false, nil
	}
	conversationTypes := uniqueArchiveMediaConversationTypes(filter.AllowedConversationTypes)
	if len(conversationTypes) == 0 {
		return dashboard.ArchiveMediaContentObject{}, false, nil
	}
	conversationScopes, ok := archiveMediaConversationScopes(conversationTypes, filter.ConversationScopes)
	if !ok {
		return dashboard.ArchiveMediaContentObject{}, false, nil
	}
	messageUnion := make([]string, 0, dashboard.WorkMessageArchiveMessageTableCount)
	for tableIndex := 1; tableIndex <= dashboard.WorkMessageArchiveMessageTableCount; tableIndex++ {
		messageUnion = append(messageUnion, fmt.Sprintf("SELECT corp_id,msgid,work_employee_id,to_user_type FROM mc_work_message_%d WHERE deleted_at IS NULL", tableIndex))
	}
	where := []string{"message.to_user_type IN (" + placeholders(len(conversationTypes)) + ")"}
	args := []any{filter.ID, filter.TenantID, filter.CorpID}
	args = append(args, intsToAny(conversationTypes)...)
	scopeWhere := make([]string, 0, len(conversationScopes))
	for _, scope := range conversationScopes {
		if !scope.RestrictEmployeeIDs {
			scopeWhere = append(scopeWhere, "message.to_user_type=?")
			args = append(args, scope.ConversationType)
			continue
		}
		scopeWhere = append(scopeWhere, "(message.to_user_type=? AND message.work_employee_id IN ("+placeholders(len(scope.AllowedEmployeeIDs))+"))")
		args = append(args, scope.ConversationType)
		args = append(args, intsToAny(scope.AllowedEmployeeIDs)...)
	}
	where = append(where, "("+strings.Join(scopeWhere, " OR ")+")")
	var object dashboard.ArchiveMediaContentObject
	err := s.db.QueryRowContext(ctx, `
		SELECT media.id,media.media_type,media.media_name,media.mime_type,media.bytes_received,media.status,media.storage_path,media.sha256
		FROM mochat_go_archive_media_objects media
		INNER JOIN mochat_go_archive_message_sources source
		  ON source.tenant_id=media.tenant_id AND source.corp_id=media.corp_id AND source.msgid=media.msgid
		 AND source.source_id=media.source_identity
		WHERE media.id=? AND media.tenant_id=? AND media.corp_id=? AND media.status='ready'
		  AND EXISTS (
		    SELECT 1 FROM (`+strings.Join(messageUnion, " UNION ALL ")+`) message
		    WHERE message.corp_id=media.corp_id AND message.msgid=media.msgid AND `+strings.Join(where, " AND ")+`
		  )
		LIMIT 1
	`, args...).Scan(&object.ID, &object.MediaType, &object.Name, &object.MIMEType, &object.Size, &object.Status, &object.StoragePath, &object.SHA256)
	if err == sql.ErrNoRows {
		return dashboard.ArchiveMediaContentObject{}, false, nil
	}
	if err != nil {
		return dashboard.ArchiveMediaContentObject{}, false, err
	}
	return object, true, nil
}

func archiveMediaConversationScopes(conversationTypes []int, input []dashboard.ArchiveMediaConversationScope) ([]dashboard.ArchiveMediaConversationScope, bool) {
	allowed := make(map[int]bool, len(conversationTypes))
	for _, conversationType := range conversationTypes {
		allowed[conversationType] = true
	}
	byType := make(map[int]dashboard.ArchiveMediaConversationScope, len(conversationTypes))
	for _, scope := range input {
		if !allowed[scope.ConversationType] {
			continue
		}
		current, exists := byType[scope.ConversationType]
		if !scope.RestrictEmployeeIDs {
			byType[scope.ConversationType] = dashboard.ArchiveMediaConversationScope{ConversationType: scope.ConversationType}
			continue
		}
		if exists && !current.RestrictEmployeeIDs {
			continue
		}
		ids := uniquePositiveInts(append(current.AllowedEmployeeIDs, scope.AllowedEmployeeIDs...))
		if len(ids) == 0 {
			continue
		}
		byType[scope.ConversationType] = dashboard.ArchiveMediaConversationScope{
			ConversationType: scope.ConversationType, RestrictEmployeeIDs: true, AllowedEmployeeIDs: ids,
		}
	}
	result := make([]dashboard.ArchiveMediaConversationScope, 0, len(conversationTypes))
	for _, conversationType := range conversationTypes {
		scope, exists := byType[conversationType]
		if !exists {
			return nil, false
		}
		result = append(result, scope)
	}
	return result, true
}

func uniqueArchiveMediaConversationTypes(values []int) []int {
	seen := map[int]bool{}
	for _, value := range values {
		if value >= 0 && value <= 2 {
			seen[value] = true
		}
	}
	result := make([]int, 0, len(seen))
	for _, value := range values {
		if seen[value] {
			result = append(result, value)
			seen[value] = false
		}
	}
	return result
}
