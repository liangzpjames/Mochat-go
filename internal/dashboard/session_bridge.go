package dashboard

import (
	"context"
	"net/http"

	"jiyi/mochat-go/internal/session"
)

type LoginCorpInfo = session.LoginCorpInfo

type loginCorpEmployeeStore interface {
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
}

type loginCorpAccessStore interface {
	CorpIDsByTenant(ctx context.Context, tenantID int) ([]int, error)
	CorpIDsByUser(ctx context.Context, userID int) ([]int, error)
}

func ResolveLoginCorpInfoFromStore(ctx context.Context, headers http.Header, userID int, cacheValue string, store Store) LoginCorpInfo {
	return session.ResolveLoginCorpInfo(headers, userID, cacheValue, func(userID int, corpID int) int {
		employeeID, err := store.EmployeeIDByUserCorp(ctx, userID, corpID)
		if err != nil {
			return 0
		}
		return employeeID
	}, func(userID int) (int, int, bool) {
		corpID, employeeID, ok, err := store.FirstEmployeeByUser(ctx, userID)
		if err != nil {
			return 0, 0, false
		}
		return corpID, employeeID, ok
	})
}

func ResolveValidatedLoginCorpInfoFromStore(ctx context.Context, headers http.Header, user User, cacheValue string, store loginCorpEmployeeStore) (LoginCorpInfo, error) {
	info := session.ResolveLoginCorpInfo(headers, user.ID, cacheValue, func(userID int, corpID int) int {
		employeeID, err := store.EmployeeIDByUserCorp(ctx, userID, corpID)
		if err != nil {
			return 0
		}
		return employeeID
	}, func(userID int) (int, int, bool) {
		corpID, employeeID, ok, err := store.FirstEmployeeByUser(ctx, userID)
		if err != nil {
			return 0, 0, false
		}
		return corpID, employeeID, ok
	})

	access, ok := any(store).(loginCorpAccessStore)
	if !ok {
		return info, nil
	}
	if normalized, ok, err := normalizeLoginCorpInfo(ctx, user, info, store, access); err != nil {
		return LoginCorpInfo{}, err
	} else if ok {
		return normalized, nil
	}
	return fallbackLoginCorpInfo(ctx, user, store, access)
}

func normalizeLoginCorpInfo(ctx context.Context, user User, info LoginCorpInfo, employeeStore loginCorpEmployeeStore, accessStore loginCorpAccessStore) (LoginCorpInfo, bool, error) {
	if len(info.CorpIDs) == 0 {
		return info, true, nil
	}
	if len(info.CorpIDs) != 1 {
		return LoginCorpInfo{CorpIDs: []int{}, RequestSource: info.RequestSource}, true, nil
	}

	corpID := info.CorpIDs[0]
	if corpID <= 0 {
		info.WorkEmployeeID = 0
		return info, true, nil
	}

	allowedCorpIDs, err := allowedLoginCorpIDs(ctx, user, accessStore)
	if err != nil {
		return LoginCorpInfo{}, false, err
	}
	if !containsInt(allowedCorpIDs, corpID) {
		return LoginCorpInfo{}, false, nil
	}

	if user.IsSuperAdmin == 1 {
		employeeID, err := employeeStore.EmployeeIDByUserCorp(ctx, user.ID, corpID)
		if err != nil {
			return LoginCorpInfo{}, false, err
		}
		if info.WorkEmployeeID > 0 && info.WorkEmployeeID == employeeID {
			return info, true, nil
		}
		info.WorkEmployeeID = 0
		return info, true, nil
	}
	return info, true, nil
}

func fallbackLoginCorpInfo(ctx context.Context, user User, employeeStore loginCorpEmployeeStore, accessStore loginCorpAccessStore) (LoginCorpInfo, error) {
	corpID, employeeID, ok, err := employeeStore.FirstEmployeeByUser(ctx, user.ID)
	if err != nil {
		return LoginCorpInfo{}, err
	}
	if !ok {
		return LoginCorpInfo{CorpIDs: []int{}, RequestSource: session.RequestSourceDashboard}, nil
	}

	info := LoginCorpInfo{
		CorpIDs:        []int{corpID},
		WorkEmployeeID: employeeID,
		RequestSource:  session.RequestSourceDashboard,
	}
	normalized, ok, err := normalizeLoginCorpInfo(ctx, user, info, employeeStore, accessStore)
	if err != nil {
		return LoginCorpInfo{}, err
	}
	if ok {
		return normalized, nil
	}
	return LoginCorpInfo{CorpIDs: []int{}, RequestSource: session.RequestSourceDashboard}, nil
}

func allowedLoginCorpIDs(ctx context.Context, user User, store loginCorpAccessStore) ([]int, error) {
	if user.IsSuperAdmin == 1 {
		return store.CorpIDsByTenant(ctx, user.TenantID)
	}
	return store.CorpIDsByUser(ctx, user.ID)
}
