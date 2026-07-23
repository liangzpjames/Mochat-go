package dashboard

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

type workEmployeeSyncStore interface {
	WorkEmployeeSyncCredentials(ctx context.Context, corpIDs []int) ([]WorkEmployeeSyncCredential, error)
	SyncWorkEmployees(ctx context.Context, credential WorkEmployeeSyncCredential, departments []WorkEmployeeSyncDepartment, employees []WorkEmployeeSyncEmployee, followUserIDs []string, defaultPasswordHash string) (WorkEmployeeSyncResult, error)
}

func syncWorkEmployeesForCorp(ctx context.Context, store workEmployeeSyncStore, client WorkEmployeeSyncClient, passwordKey string, corpID int) error {
	credentials, err := store.WorkEmployeeSyncCredentials(ctx, []int{corpID})
	if err != nil {
		return err
	}
	if len(credentials) == 0 {
		return fmt.Errorf("corp credential not found")
	}
	credential := credentials[0]
	if strings.TrimSpace(credential.WXCorpID) == "" || strings.TrimSpace(credential.EmployeeSecret) == "" {
		return fmt.Errorf("corp employee credential is incomplete")
	}
	departments, err := client.Departments(ctx, credential)
	if err != nil {
		return err
	}
	employees, err := workEmployeeSyncUsersWithClient(ctx, client, credential, departments)
	if err != nil {
		return err
	}
	followUserIDs, _ := client.FollowUsers(ctx, credential)
	defaultPasswordHash, err := randomWorkEmployeePasswordHash(passwordKey)
	if err != nil {
		return err
	}
	_, err = store.SyncWorkEmployees(ctx, credential, departments, employees, followUserIDs, defaultPasswordHash)
	return err
}

func workEmployeeSyncUsersWithClient(ctx context.Context, client WorkEmployeeSyncClient, credential WorkEmployeeSyncCredential, departments []WorkEmployeeSyncDepartment) ([]WorkEmployeeSyncEmployee, error) {
	merged := map[string]WorkEmployeeSyncEmployee{}
	for _, department := range departments {
		if department.WXDepartmentID <= 0 {
			continue
		}
		users, err := client.DepartmentUsers(ctx, credential, department.WXDepartmentID)
		if err != nil {
			return nil, err
		}
		for _, user := range users {
			wxUserID := strings.TrimSpace(user.WXUserID)
			if wxUserID == "" {
				continue
			}
			user.WXUserID = wxUserID
			if current, ok := merged[wxUserID]; ok {
				merged[wxUserID] = mergeWorkEmployeeSyncEmployee(current, user)
				continue
			}
			merged[wxUserID] = user
		}
	}
	keys := make([]string, 0, len(merged))
	for wxUserID := range merged {
		keys = append(keys, wxUserID)
	}
	sort.Strings(keys)
	result := make([]WorkEmployeeSyncEmployee, 0, len(keys))
	for _, wxUserID := range keys {
		result = append(result, merged[wxUserID])
	}
	return result, nil
}
