package store

import (
	"context"
	"database/sql"
	"errors"

	"jiyi/mochat-go/internal/dashboardprincipal"
)

type tenantCorpBindingQueryRowFunc func(context.Context, string, ...any) identityRowScanner

type TenantCorpBindingStore struct {
	db       *sql.DB
	queryRow tenantCorpBindingQueryRowFunc
}

func NewTenantCorpBindingStore(db *sql.DB) *TenantCorpBindingStore {
	store := &TenantCorpBindingStore{db: db}
	if db != nil {
		store.queryRow = func(ctx context.Context, query string, args ...any) identityRowScanner {
			return db.QueryRowContext(ctx, query, args...)
		}
	}
	return store
}

func (store *TenantCorpBindingStore) ResolveBinding(ctx context.Context, tenantID int) (dashboardprincipal.Binding, error) {
	if store == nil || tenantID <= 0 {
		return dashboardprincipal.Binding{}, dashboardprincipal.ErrBindingUnavailable
	}
	row, err := store.query(ctx, `
		SELECT COUNT(*) AS binding_count,
		       COALESCE(MIN(b.tenant_id), 0),
		       COALESCE(MIN(b.corp_id), 0),
		       COALESCE(MIN(b.status), 0),
		       COALESCE(MIN(b.version), 0),
		       COALESCE(MIN(c.tenant_id), 0)
		FROM mochat_go_tenant_corp_bindings b
		LEFT JOIN mc_corp c ON c.id = b.corp_id
		WHERE b.tenant_id = ?
	`, tenantID)
	if err != nil {
		return dashboardprincipal.Binding{}, dashboardprincipal.ErrBindingUnavailable
	}

	var count, bindingTenant, corpID, status, corpTenant int64
	var version uint64
	if err := row.Scan(&count, &bindingTenant, &corpID, &status, &version, &corpTenant); err != nil {
		return dashboardprincipal.Binding{}, dashboardprincipal.ErrBindingUnavailable
	}
	if count != 1 || bindingTenant != int64(tenantID) || corpID <= 0 || corpTenant != int64(tenantID) || version == 0 {
		return dashboardprincipal.Binding{}, dashboardprincipal.ErrBindingUnavailable
	}

	var bindingStatus dashboardprincipal.CorpBindingStatus
	switch status {
	case 1:
		bindingStatus = dashboardprincipal.CorpBindingStatusPending
	case 2:
		bindingStatus = dashboardprincipal.CorpBindingStatusActive
	case 3:
		bindingStatus = dashboardprincipal.CorpBindingStatusSuspended
	default:
		return dashboardprincipal.Binding{}, dashboardprincipal.ErrBindingUnavailable
	}
	return dashboardprincipal.Binding{
		TenantID: int(bindingTenant),
		CorpID:   int(corpID),
		Status:   bindingStatus,
		Version:  version,
		Count:    int(count),
	}, nil
}

func (store *TenantCorpBindingStore) query(ctx context.Context, query string, args ...any) (identityRowScanner, error) {
	if store == nil {
		return nil, errors.New("tenant corp binding store is unavailable")
	}
	if store.queryRow != nil {
		return store.queryRow(ctx, query, args...), nil
	}
	if store.db == nil {
		return nil, errors.New("tenant corp binding store is unavailable")
	}
	return store.db.QueryRowContext(ctx, query, args...), nil
}
