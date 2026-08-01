package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"jiyi/mochat-go/internal/modules/scrm/ports"
)

func (r *TagRepository) ListTagCatalog(ctx context.Context, filter ports.ListTagCatalogFilter) (ports.TagCatalog, error) {
	groups, err := r.db.QueryContext(ctx, `SELECT g.id,g.name,g.version,COUNT(t.id) FROM mochat_go_scrm_tag_groups g LEFT JOIN mochat_go_scrm_tags t ON t.tenant_id=g.tenant_id AND t.corp_id=g.corp_id AND t.group_id=g.id AND t.deleted_at IS NULL WHERE g.tenant_id=? AND g.corp_id=? AND g.deleted_at IS NULL GROUP BY g.id,g.name,g.version ORDER BY g.name,g.id`, filter.TenantID, filter.CorpID)
	if err != nil {
		return ports.TagCatalog{}, err
	}
	defer groups.Close()
	catalog := ports.TagCatalog{Groups: []ports.TagGroup{}, Tags: []ports.CustomerTag{}}
	for groups.Next() {
		item := ports.TagGroup{TenantID: filter.TenantID, CorpID: filter.CorpID}
		if err := groups.Scan(&item.ID, &item.Name, &item.Version, &item.TagCount); err != nil {
			return ports.TagCatalog{}, err
		}
		catalog.Groups = append(catalog.Groups, item)
	}
	if err := groups.Err(); err != nil {
		return ports.TagCatalog{}, err
	}

	query := `SELECT t.id,t.group_id,t.name,t.version,COUNT(ct.contact_id) FROM mochat_go_scrm_tags t LEFT JOIN mochat_go_scrm_contact_tags ct ON ct.tenant_id=t.tenant_id AND ct.corp_id=t.corp_id AND ct.tag_id=t.id WHERE t.tenant_id=? AND t.corp_id=? AND t.deleted_at IS NULL AND t.group_id IS NOT NULL`
	args := []any{filter.TenantID, filter.CorpID}
	if filter.GroupID != "" {
		query += ` AND t.group_id=?`
		args = append(args, filter.GroupID)
	}
	if filter.Keyword != "" {
		query += ` AND t.name LIKE ? ESCAPE '\\'`
		args = append(args, "%"+escapeLike(filter.Keyword)+"%")
	}
	query += ` GROUP BY t.id,t.group_id,t.name,t.version ORDER BY t.name,t.id`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return ports.TagCatalog{}, err
	}
	defer rows.Close()
	for rows.Next() {
		item := ports.CustomerTag{TenantID: filter.TenantID, CorpID: filter.CorpID}
		if err := rows.Scan(&item.ID, &item.GroupID, &item.Name, &item.Version, &item.UsageCount); err != nil {
			return ports.TagCatalog{}, err
		}
		catalog.Tags = append(catalog.Tags, item)
	}
	return catalog, rows.Err()
}

func (r *TagRepository) CreateGroup(ctx context.Context, command ports.CreateTagGroupCommand) (ports.TagGroup, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ports.TagGroup{}, err
	}
	defer tx.Rollback()
	if err := corpExistsTx(ctx, tx, command.TenantID, command.CorpID); err != nil {
		return ports.TagGroup{}, err
	}
	id, name := uuid.NewString(), strings.TrimSpace(command.Name)
	replayed, resourceID, err := claimIdempotency(ctx, tx, command.TenantID, command.CorpID, "tag.group.create", command.IdempotencyKey, requestFingerprint(struct{ Name string }{name}), id)
	if err != nil {
		return ports.TagGroup{}, err
	}
	if !replayed {
		if err := duplicateGroupNameTx(ctx, tx, command.TenantID, command.CorpID, "", name); err != nil {
			return ports.TagGroup{}, err
		}
		now := time.Now().UTC()
		if _, err := tx.ExecContext(ctx, `INSERT INTO mochat_go_scrm_tag_groups(id,tenant_id,corp_id,name,version,created_at,updated_at) VALUES(?,?,?,?,1,?,?)`, id, command.TenantID, command.CorpID, name, now, now); err != nil {
			return ports.TagGroup{}, err
		}
	}
	item, err := getTagGroupWith(ctx, tx, command.TenantID, command.CorpID, resourceID, false)
	if err != nil {
		return ports.TagGroup{}, err
	}
	if err := tx.Commit(); err != nil {
		return ports.TagGroup{}, err
	}
	return item, nil
}

func (r *TagRepository) RenameGroup(ctx context.Context, command ports.RenameTagGroupCommand) (ports.TagGroup, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ports.TagGroup{}, err
	}
	defer tx.Rollback()
	if err := corpExistsTx(ctx, tx, command.TenantID, command.CorpID); err != nil {
		return ports.TagGroup{}, err
	}
	name := strings.TrimSpace(command.Name)
	fingerprint := requestFingerprint(struct {
		GroupID, Name string
		Version       int64
	}{command.GroupID, name, command.Version})
	replayed, resourceID, err := claimIdempotency(ctx, tx, command.TenantID, command.CorpID, "tag.group.rename", command.IdempotencyKey, fingerprint, command.GroupID)
	if err != nil {
		return ports.TagGroup{}, err
	}
	if !replayed {
		if _, err := getTagGroupWith(ctx, tx, command.TenantID, command.CorpID, command.GroupID, true); err != nil {
			return ports.TagGroup{}, err
		}
		if err := duplicateGroupNameTx(ctx, tx, command.TenantID, command.CorpID, command.GroupID, name); err != nil {
			return ports.TagGroup{}, err
		}
		result, err := tx.ExecContext(ctx, `UPDATE mochat_go_scrm_tag_groups SET name=?,version=version+1,updated_at=? WHERE tenant_id=? AND corp_id=? AND id=? AND version=? AND deleted_at IS NULL`, name, time.Now().UTC(), command.TenantID, command.CorpID, command.GroupID, command.Version)
		if err != nil {
			return ports.TagGroup{}, err
		}
		if n, _ := result.RowsAffected(); n != 1 {
			return ports.TagGroup{}, ports.ErrAssignmentConflict
		}
	}
	item, err := getTagGroupWith(ctx, tx, command.TenantID, command.CorpID, resourceID, false)
	if err != nil {
		return ports.TagGroup{}, err
	}
	if err := tx.Commit(); err != nil {
		return ports.TagGroup{}, err
	}
	return item, nil
}

func (r *TagRepository) CreateCustomerTag(ctx context.Context, command ports.CreateCustomerTagCommand) (ports.CustomerTag, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ports.CustomerTag{}, err
	}
	defer tx.Rollback()
	if err := corpExistsTx(ctx, tx, command.TenantID, command.CorpID); err != nil {
		return ports.CustomerTag{}, err
	}
	if _, err := getTagGroupWith(ctx, tx, command.TenantID, command.CorpID, command.GroupID, true); err != nil {
		return ports.CustomerTag{}, err
	}
	id, name := uuid.NewString(), strings.TrimSpace(command.Name)
	fingerprint := requestFingerprint(struct{ GroupID, Name string }{command.GroupID, name})
	replayed, resourceID, err := claimIdempotency(ctx, tx, command.TenantID, command.CorpID, "tag.create.v2", command.IdempotencyKey, fingerprint, id)
	if err != nil {
		return ports.CustomerTag{}, err
	}
	if !replayed {
		if err := duplicateTagNameTx(ctx, tx, command.TenantID, command.CorpID, command.GroupID, "", name); err != nil {
			return ports.CustomerTag{}, err
		}
		now := time.Now().UTC()
		if _, err := tx.ExecContext(ctx, `INSERT INTO mochat_go_scrm_tags(id,tenant_id,corp_id,group_id,name,version,created_at,updated_at) VALUES(?,?,?,?,?,1,?,?)`, id, command.TenantID, command.CorpID, command.GroupID, name, now, now); err != nil {
			return ports.CustomerTag{}, err
		}
	}
	item, err := getCustomerTagWith(ctx, tx, command.TenantID, command.CorpID, resourceID, false)
	if err != nil {
		return ports.CustomerTag{}, err
	}
	if err := tx.Commit(); err != nil {
		return ports.CustomerTag{}, err
	}
	return item, nil
}

func (r *TagRepository) RenameCustomerTag(ctx context.Context, command ports.RenameCustomerTagCommand) (ports.CustomerTag, error) {
	return r.mutateCustomerTag(ctx, "tag.rename.v2", command.TenantID, command.CorpID, command.TagID, command.Version, command.IdempotencyKey,
		requestFingerprint(struct {
			TagID, Name string
			Version     int64
		}{command.TagID, strings.TrimSpace(command.Name), command.Version}),
		func(ctx context.Context, tx *sql.Tx, current ports.CustomerTag) error {
			name := strings.TrimSpace(command.Name)
			if err := duplicateTagNameTx(ctx, tx, command.TenantID, command.CorpID, current.GroupID, command.TagID, name); err != nil {
				return err
			}
			_, err := tx.ExecContext(ctx, `UPDATE mochat_go_scrm_tags SET name=?,version=version+1,updated_at=? WHERE tenant_id=? AND corp_id=? AND id=?`, name, time.Now().UTC(), command.TenantID, command.CorpID, command.TagID)
			return err
		})
}

func (r *TagRepository) MoveCustomerTag(ctx context.Context, command ports.MoveCustomerTagCommand) (ports.CustomerTag, error) {
	return r.mutateCustomerTag(ctx, "tag.move", command.TenantID, command.CorpID, command.TagID, command.Version, command.IdempotencyKey,
		requestFingerprint(struct {
			TagID, GroupID string
			Version        int64
		}{command.TagID, command.GroupID, command.Version}),
		func(ctx context.Context, tx *sql.Tx, current ports.CustomerTag) error {
			if _, err := getTagGroupWith(ctx, tx, command.TenantID, command.CorpID, command.GroupID, true); err != nil {
				return err
			}
			if err := duplicateTagNameTx(ctx, tx, command.TenantID, command.CorpID, command.GroupID, command.TagID, current.Name); err != nil {
				return err
			}
			_, err := tx.ExecContext(ctx, `UPDATE mochat_go_scrm_tags SET group_id=?,version=version+1,updated_at=? WHERE tenant_id=? AND corp_id=? AND id=?`, command.GroupID, time.Now().UTC(), command.TenantID, command.CorpID, command.TagID)
			return err
		})
}

func (r *TagRepository) MaintainTagContacts(ctx context.Context, command ports.MaintainTagContactsCommand) (ports.CustomerTag, error) {
	add, remove := normalizedContactIDs(command.AddContactIDs), normalizedContactIDs(command.RemoveContactIDs)
	fingerprint := requestFingerprint(struct {
		TagID       string
		Add, Remove []string
		Version     int64
	}{command.TagID, add, remove, command.Version})
	return r.mutateCustomerTag(ctx, "tag.contacts", command.TenantID, command.CorpID, command.TagID, command.Version, command.IdempotencyKey, fingerprint,
		func(ctx context.Context, tx *sql.Tx, _ ports.CustomerTag) error {
			for _, id := range append(append([]string{}, add...), remove...) {
				if err := contactExistsTx(ctx, tx, command.TenantID, command.CorpID, id); err != nil {
					return err
				}
			}
			for _, id := range remove {
				if _, err := tx.ExecContext(ctx, `DELETE FROM mochat_go_scrm_contact_tags WHERE tenant_id=? AND corp_id=? AND tag_id=? AND contact_id=?`, command.TenantID, command.CorpID, command.TagID, id); err != nil {
					return err
				}
			}
			for _, id := range add {
				if _, err := tx.ExecContext(ctx, `INSERT IGNORE INTO mochat_go_scrm_contact_tags(tenant_id,corp_id,contact_id,tag_id,created_at) VALUES(?,?,?,?,?)`, command.TenantID, command.CorpID, id, command.TagID, time.Now().UTC()); err != nil {
					return err
				}
			}
			_, err := tx.ExecContext(ctx, `UPDATE mochat_go_scrm_tags SET version=version+1,updated_at=? WHERE tenant_id=? AND corp_id=? AND id=?`, time.Now().UTC(), command.TenantID, command.CorpID, command.TagID)
			return err
		})
}

func (r *TagRepository) DeleteCustomerTag(ctx context.Context, command ports.DeleteCustomerTagCommand) (ports.DeleteCustomerTagResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ports.DeleteCustomerTagResult{}, err
	}
	defer tx.Rollback()
	if err := corpExistsTx(ctx, tx, command.TenantID, command.CorpID); err != nil {
		return ports.DeleteCustomerTagResult{}, err
	}
	fingerprint := requestFingerprint(struct {
		TagID   string
		Version int64
	}{command.TagID, command.Version})
	replayed, resource, err := claimIdempotency(ctx, tx, command.TenantID, command.CorpID, "tag.delete", command.IdempotencyKey, fingerprint, command.TagID)
	if err != nil {
		return ports.DeleteCustomerTagResult{}, err
	}
	if replayed {
		count := parseAffectedCount(resource)
		if err := tx.Commit(); err != nil {
			return ports.DeleteCustomerTagResult{}, err
		}
		return ports.DeleteCustomerTagResult{AffectedResourceCount: count}, nil
	}
	current, err := getCustomerTagWith(ctx, tx, command.TenantID, command.CorpID, command.TagID, true)
	if err != nil {
		return ports.DeleteCustomerTagResult{}, err
	}
	if current.Version != command.Version {
		return ports.DeleteCustomerTagResult{}, ports.ErrAssignmentConflict
	}
	count := current.UsageCount
	if _, err := tx.ExecContext(ctx, `DELETE FROM mochat_go_scrm_contact_tags WHERE tenant_id=? AND corp_id=? AND tag_id=?`, command.TenantID, command.CorpID, command.TagID); err != nil {
		return ports.DeleteCustomerTagResult{}, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE mochat_go_scrm_tags SET deleted_at=?,version=version+1,updated_at=? WHERE tenant_id=? AND corp_id=? AND id=? AND version=? AND deleted_at IS NULL`, time.Now().UTC(), time.Now().UTC(), command.TenantID, command.CorpID, command.TagID, command.Version)
	if err != nil {
		return ports.DeleteCustomerTagResult{}, err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return ports.DeleteCustomerTagResult{}, ports.ErrAssignmentConflict
	}
	encoded := fmt.Sprintf("%s|%d", command.TagID, count)
	if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_scrm_idempotency_keys SET resource_id=? WHERE tenant_id=? AND corp_id=? AND action='tag.delete' AND idempotency_key=?`, encoded, command.TenantID, command.CorpID, command.IdempotencyKey); err != nil {
		return ports.DeleteCustomerTagResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return ports.DeleteCustomerTagResult{}, err
	}
	return ports.DeleteCustomerTagResult{AffectedResourceCount: count}, nil
}

func (r *TagRepository) mutateCustomerTag(ctx context.Context, action string, tenant, corp int64, tagID string, version int64, key, fingerprint string, mutate func(context.Context, *sql.Tx, ports.CustomerTag) error) (ports.CustomerTag, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ports.CustomerTag{}, err
	}
	defer tx.Rollback()
	if err := corpExistsTx(ctx, tx, tenant, corp); err != nil {
		return ports.CustomerTag{}, err
	}
	replayed, resourceID, err := claimIdempotency(ctx, tx, tenant, corp, action, key, fingerprint, tagID)
	if err != nil {
		return ports.CustomerTag{}, err
	}
	if !replayed {
		current, err := getCustomerTagWith(ctx, tx, tenant, corp, tagID, true)
		if err != nil {
			return ports.CustomerTag{}, err
		}
		if current.Version != version {
			return ports.CustomerTag{}, ports.ErrAssignmentConflict
		}
		if err := mutate(ctx, tx, current); err != nil {
			return ports.CustomerTag{}, err
		}
	}
	item, err := getCustomerTagWith(ctx, tx, tenant, corp, resourceID, false)
	if err != nil {
		return ports.CustomerTag{}, err
	}
	if err := tx.Commit(); err != nil {
		return ports.CustomerTag{}, err
	}
	return item, nil
}

func getTagGroupWith(ctx context.Context, query rowQuerier, tenant, corp int64, id string, lock bool) (ports.TagGroup, error) {
	statement := `SELECT id,name,version FROM mochat_go_scrm_tag_groups WHERE tenant_id=? AND corp_id=? AND id=? AND deleted_at IS NULL`
	if lock {
		statement += ` FOR UPDATE`
	}
	item := ports.TagGroup{TenantID: tenant, CorpID: corp}
	err := query.QueryRowContext(ctx, statement, tenant, corp, id).Scan(&item.ID, &item.Name, &item.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return ports.TagGroup{}, ports.ErrTagGroupNotFound
	}
	return item, err
}

func getCustomerTagWith(ctx context.Context, query rowQuerier, tenant, corp int64, id string, lock bool) (ports.CustomerTag, error) {
	statement := `SELECT t.id,t.group_id,t.name,t.version,(SELECT COUNT(*) FROM mochat_go_scrm_contact_tags ct WHERE ct.tenant_id=t.tenant_id AND ct.corp_id=t.corp_id AND ct.tag_id=t.id) FROM mochat_go_scrm_tags t WHERE t.tenant_id=? AND t.corp_id=? AND t.id=? AND t.deleted_at IS NULL`
	if lock {
		statement += ` FOR UPDATE`
	}
	item := ports.CustomerTag{TenantID: tenant, CorpID: corp}
	err := query.QueryRowContext(ctx, statement, tenant, corp, id).Scan(&item.ID, &item.GroupID, &item.Name, &item.Version, &item.UsageCount)
	if errors.Is(err, sql.ErrNoRows) {
		return ports.CustomerTag{}, ports.ErrTagNotFound
	}
	return item, err
}

func duplicateGroupNameTx(ctx context.Context, tx *sql.Tx, tenant, corp int64, excludeID, name string) error {
	var found string
	err := tx.QueryRowContext(ctx, `SELECT id FROM mochat_go_scrm_tag_groups WHERE tenant_id=? AND corp_id=? AND LOWER(name)=LOWER(?) AND id<>? AND deleted_at IS NULL LIMIT 1`, tenant, corp, name, excludeID).Scan(&found)
	if err == nil {
		return ports.ErrDuplicateTagName
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	return err
}

func duplicateTagNameTx(ctx context.Context, tx *sql.Tx, tenant, corp int64, groupID, excludeID, name string) error {
	var found string
	err := tx.QueryRowContext(ctx, `SELECT id FROM mochat_go_scrm_tags WHERE tenant_id=? AND corp_id=? AND group_id=? AND LOWER(name)=LOWER(?) AND id<>? AND deleted_at IS NULL LIMIT 1`, tenant, corp, groupID, name, excludeID).Scan(&found)
	if err == nil {
		return ports.ErrDuplicateTagName
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	return err
}

func parseAffectedCount(resource string) int64 {
	parts := strings.Split(resource, "|")
	if len(parts) != 2 {
		return 0
	}
	value, _ := strconv.ParseInt(parts[1], 10, 64)
	return value
}

var _ ports.CustomerTagRepository = (*TagRepository)(nil)
