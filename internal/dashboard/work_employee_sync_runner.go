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

// employeeApplyStore is the binding-scoped contract used by the controlled
// company sync job. It deliberately has no password/hash operation and no
// client-selected tenant or corp argument.
type employeeApplyStore interface {
	TenantIDByBindingID(ctx context.Context, bindingID int) (int, error)
	CompanyEmployeeSyncCredentials(ctx context.Context, bindingID int) ([]WorkEmployeeSyncCredential, error)
	BeginCompanyEmployeeSyncAtVersion(ctx context.Context, bindingID int, credentialVersion uint64, queueTicket string) error
	SyncCompanyEmployeesAtVersion(ctx context.Context, bindingID int, credentialVersion uint64, queueTicket string, departments []WorkEmployeeSyncDepartment, employees []WorkEmployeeSyncEmployee) (WorkEmployeeSyncResult, error)
	MarkCompanyEmployeeSyncQueuedAtVersion(ctx context.Context, bindingID int, credentialVersion uint64, queueTicket string, errorCode string) error
	RecordCompanyEmployeeSyncFailureAtVersion(ctx context.Context, bindingID int, credentialVersion uint64, queueTicket string) error
}

type employeeSyncVersionError struct {
	Version uint64
	Err     error
}

func (e *employeeSyncVersionError) Error() string { return e.Err.Error() }
func (e *employeeSyncVersionError) Unwrap() error { return e.Err }

func wrapEmployeeSyncVersion(version uint64, err error) error {
	if err == nil {
		return nil
	}
	return &employeeSyncVersionError{Version: version, Err: err}
}

func syncCompanyEmployeesForBinding(ctx context.Context, store employeeApplyStore, client WorkEmployeeSyncClient, bindingID int, queueTicket string) error {
	if bindingID <= 0 {
		return fmt.Errorf("missing binding id")
	}
	credentials, err := store.CompanyEmployeeSyncCredentials(ctx, bindingID)
	if err != nil {
		return err
	}
	if len(credentials) != 1 {
		return fmt.Errorf("company binding credential unavailable")
	}
	credential := credentials[0]
	tenantID, err := store.TenantIDByBindingID(ctx, bindingID)
	if err != nil {
		return err
	}
	if tenantID <= 0 || credential.TenantID != tenantID {
		return fmt.Errorf("company binding tenant scope mismatch")
	}
	if credential.CorpID <= 0 || strings.TrimSpace(credential.WXCorpID) == "" || strings.TrimSpace(credential.EmployeeSecret) == "" {
		return fmt.Errorf("company binding credential invalid")
	}
	if credential.CredentialVersion == 0 {
		return fmt.Errorf("company binding credential version unavailable")
	}
	if strings.TrimSpace(queueTicket) == "" {
		return fmt.Errorf("company queue ticket unavailable")
	}
	if err := store.BeginCompanyEmployeeSyncAtVersion(ctx, bindingID, credential.CredentialVersion, queueTicket); err != nil {
		return wrapEmployeeSyncVersion(credential.CredentialVersion, err)
	}
	departments, err := client.Departments(ctx, credential)
	if err != nil {
		return wrapEmployeeSyncVersion(credential.CredentialVersion, err)
	}
	employees, err := workEmployeeSyncUsersWithClient(ctx, client, credential, departments)
	if err != nil {
		return wrapEmployeeSyncVersion(credential.CredentialVersion, err)
	}
	_, err = store.SyncCompanyEmployeesAtVersion(ctx, bindingID, credential.CredentialVersion, queueTicket, departments, employees)
	return wrapEmployeeSyncVersion(credential.CredentialVersion, err)
}

// WorkEmployeeSyncEmployees is the shared provider adapter for the company
// profile service. It only fetches business employee data; persistence remains
// in the binding-scoped Store transaction.
func WorkEmployeeSyncEmployees(ctx context.Context, client WorkEmployeeSyncClient, credential WorkEmployeeSyncCredential, departments []WorkEmployeeSyncDepartment) ([]WorkEmployeeSyncEmployee, error) {
	return workEmployeeSyncUsersWithClient(ctx, client, credential, departments)
}

func uniqueEmployeeApplyCorpIDs(values []int) []int {
	seen := map[int]struct{}{}
	result := make([]int, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Ints(result)
	return result
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
	_, err = store.SyncWorkEmployees(ctx, credential, departments, employees, followUserIDs, "")
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
