package dashboard

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

const weComCredentialInvalidError = "WECOM_CREDENTIAL_INVALID"

type WeComCompanyVerificationResult struct {
	WXCorpID string
	CorpName string
}

// VerifyCompany validates both company credentials and the provider-returned
// company name. The caller must persist only the returned name after the
// binding/version transaction succeeds.
//
// The employee secret is probed through the department-list capability. This
// avoids inventing a member userid, and the name is accepted only from the
// provider's root department (id=1). The contact secret is probed through the
// follow-user-list capability, which likewise does not require an arbitrary
// external_userid.
func (c *RoomWelcomeWeComClient) VerifyCompany(ctx context.Context, wxCorpID string, employeeSecret string, contactSecret string) (WeComCompanyVerificationResult, error) {
	name, err := c.verifyEmployeeSecret(ctx, wxCorpID, employeeSecret)
	if err != nil {
		return WeComCompanyVerificationResult{}, err
	}
	if err := c.verifyContactSecret(ctx, wxCorpID, contactSecret); err != nil {
		return WeComCompanyVerificationResult{}, err
	}
	return WeComCompanyVerificationResult{WXCorpID: strings.TrimSpace(wxCorpID), CorpName: name}, nil
}

func (c *RoomWelcomeWeComClient) ValidateCorpSecrets(ctx context.Context, wxCorpID string, employeeSecret string, contactSecret string) error {
	if _, err := c.verifyEmployeeSecret(ctx, wxCorpID, employeeSecret); err != nil {
		return err
	}
	return c.verifyContactSecret(ctx, wxCorpID, contactSecret)
}

func (c *RoomWelcomeWeComClient) verifyEmployeeSecret(ctx context.Context, wxCorpID string, employeeSecret string) (string, error) {
	if strings.TrimSpace(wxCorpID) == "" || strings.TrimSpace(employeeSecret) == "" {
		return "", errors.New(weComCredentialInvalidError)
	}
	departments, err := c.Departments(ctx, WorkEmployeeSyncCredential{
		WXCorpID: wxCorpID, EmployeeSecret: employeeSecret,
	})
	if err != nil {
		return "", errors.New(weComCredentialInvalidError)
	}
	for _, department := range departments {
		if department.WXDepartmentID == 1 {
			name := strings.TrimSpace(department.Name)
			if name != "" {
				return name, nil
			}
			break
		}
	}
	return "", fmt.Errorf("%s: provider root department name unavailable", weComCredentialInvalidError)
}

func (c *RoomWelcomeWeComClient) verifyContactSecret(ctx context.Context, wxCorpID string, contactSecret string) error {
	if strings.TrimSpace(wxCorpID) == "" || strings.TrimSpace(contactSecret) == "" {
		return errors.New(weComCredentialInvalidError)
	}
	if _, err := c.FollowUsers(ctx, WorkEmployeeSyncCredential{
		WXCorpID: wxCorpID, ContactSecret: contactSecret,
	}); err != nil {
		return errors.New(weComCredentialInvalidError)
	}
	return nil
}
