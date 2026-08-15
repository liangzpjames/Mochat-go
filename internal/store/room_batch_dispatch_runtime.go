package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"

	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/wecomcapability"
)

type roomBatchDispatchTarget struct {
	BatchID         int
	OwnerEmployeeID int
	ChunkNo         int
}

// RoomBatchDispatchPayload is called by the durable sender after the
// dispatch has been claimed. The payload is reconstructed from the same
// tenant-scoped business rows used at creation; it is never copied into the
// ledger and never logs credentials.
func (s *MySQLStore) RoomBatchDispatchPayload(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, dispatch wecomcapability.Dispatch) (dashboard.RoomMessageBatchSendMessagePayload, error) {
	if s == nil || s.db == nil || principal.TenantID <= 0 || principal.CorpID <= 0 || dispatch.TenantID != principal.TenantID || dispatch.CorpID != principal.CorpID || dispatch.DispatchKind != string(wecomcapability.DispatchKindRoomBatch) {
		return dashboard.RoomMessageBatchSendMessagePayload{}, dashboard.ErrRoomBatchBodyScope
	}
	if err := s.AuthorizeDispatch(ctx, wecomcapability.DispatchAuthorizationRequest{
		Principal: wecomcapability.DispatchPrincipal{
			UserID: principal.UserID, TenantID: principal.TenantID, CorpID: principal.CorpID,
			IsSuperAdmin: principal.IsSuperAdmin, AuthVersion: principal.AuthVersion,
		},
		Capability: wecomcapability.RoomBatchSend, DispatchID: dispatch.ID,
		ExpectedCredentialVersion: dispatch.CredentialVersion, Stage: "payload",
	}); err != nil {
		return dashboard.RoomMessageBatchSendMessagePayload{}, err
	}
	target, ok := parseRoomBatchDispatchTarget(dispatch.TargetID)
	if !ok {
		return dashboard.RoomMessageBatchSendMessagePayload{}, dashboard.ErrRoomBatchBodyScope
	}
	var contentJSON, sender string
	if err := s.db.QueryRowContext(ctx, `
		SELECT b.content, e.wx_user_id
		FROM mc_room_message_batch_send b
		JOIN mc_room_message_batch_send_employee e ON e.batch_id=b.id AND e.employee_id=?
		WHERE b.tenant_id=? AND b.corp_id=? AND b.id=? AND b.deleted_at IS NULL`, target.OwnerEmployeeID, principal.TenantID, principal.CorpID, target.BatchID).Scan(&contentJSON, &sender); err != nil {
		if err == sql.ErrNoRows {
			return dashboard.RoomMessageBatchSendMessagePayload{}, dashboard.ErrRoomBatchTargetNotOwned
		}
		return dashboard.RoomMessageBatchSendMessagePayload{}, err
	}
	var content []dashboard.ContactMessageBatchSendContent
	if err := json.Unmarshal([]byte(contentJSON), &content); err != nil || len(content) == 0 || strings.TrimSpace(sender) == "" {
		return dashboard.RoomMessageBatchSendMessagePayload{}, dashboard.ErrRoomBatchBodyScope
	}
	offset := target.ChunkNo * contactBatchDispatchChunkSize
	rows, err := s.db.QueryContext(ctx, `
		SELECT chat_id
		FROM mc_room_message_batch_send_result
		WHERE batch_id=? AND employee_id=? AND chat_id<>''
		ORDER BY id ASC LIMIT ?,?`, target.BatchID, target.OwnerEmployeeID, offset, contactBatchDispatchChunkSize)
	if err != nil {
		return dashboard.RoomMessageBatchSendMessagePayload{}, err
	}
	defer rows.Close()
	chatIDs := make([]string, 0, contactBatchDispatchChunkSize)
	for rows.Next() {
		var chatID string
		if err := rows.Scan(&chatID); err != nil {
			return dashboard.RoomMessageBatchSendMessagePayload{}, err
		}
		chatIDs = append(chatIDs, strings.TrimSpace(chatID))
	}
	if err := rows.Err(); err != nil {
		return dashboard.RoomMessageBatchSendMessagePayload{}, err
	}
	if len(chatIDs) == 0 {
		return dashboard.RoomMessageBatchSendMessagePayload{}, dashboard.ErrRoomBatchTargetNotOwned
	}
	return dashboard.RoomMessageBatchSendMessagePayload{Content: content, ChatIDs: chatIDs, Sender: strings.TrimSpace(sender)}, nil
}

func (s *MySQLStore) RoomBatchDispatchCredential(ctx context.Context, principal dashboardprincipal.DashboardPrincipal) (dashboard.RoomWelcomeCorpCredential, bool, error) {
	if principal.TenantID <= 0 || principal.CorpID <= 0 {
		return dashboard.RoomWelcomeCorpCredential{}, false, dashboard.ErrRoomBatchBodyScope
	}
	credential, found, err := s.RoomWelcomeCorpCredentialByID(ctx, principal.CorpID)
	if err != nil || !found || credential.CorpID != principal.CorpID {
		return dashboard.RoomWelcomeCorpCredential{}, false, err
	}
	return credential, true, nil
}

func parseRoomBatchDispatchTarget(value string) (roomBatchDispatchTarget, bool) {
	parts := strings.Split(value, ":")
	if len(parts) != 6 || parts[0] != "room_batch" || parts[1] == "" || parts[2] != "owner" || parts[4] != "chunk" {
		return roomBatchDispatchTarget{}, false
	}
	batchID, batchErr := strconv.Atoi(parts[1])
	ownerEmployeeID, ownerErr := strconv.Atoi(parts[3])
	chunkNo, chunkErr := strconv.Atoi(parts[5])
	if batchErr != nil || ownerErr != nil || chunkErr != nil || batchID <= 0 || ownerEmployeeID <= 0 || chunkNo < 0 {
		return roomBatchDispatchTarget{}, false
	}
	return roomBatchDispatchTarget{BatchID: batchID, OwnerEmployeeID: ownerEmployeeID, ChunkNo: chunkNo}, true
}
