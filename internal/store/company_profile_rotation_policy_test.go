package store

import (
	"testing"

	"jiyi/mochat-go/internal/companyprofile"
)

func TestWeComRotationPolicyInvalidatesOnlyStandardSecrets(t *testing.T) {
	employeeSecret := "employee-secret"
	contactSecret := "contact-secret"
	callbackToken := "callback-token"
	chatSecret := "chat-secret"
	cases := []struct {
		name   string
		input  companyprofile.WeComCredentialsInput
		policy companyCredentialRotationPolicy
	}{
		{name: "employee secret", input: companyprofile.WeComCredentialsInput{EmployeeSecret: &employeeSecret}, policy: companyCredentialRotationInvalidatesVerification},
		{name: "contact secret", input: companyprofile.WeComCredentialsInput{ContactSecret: &contactSecret}, policy: companyCredentialRotationInvalidatesVerification},
		{name: "callback only", input: companyprofile.WeComCredentialsInput{CallbackToken: &callbackToken}, policy: companyCredentialRotationPreservesVerification},
		{name: "chat only", input: companyprofile.WeComCredentialsInput{ChatSecret: &chatSecret}, policy: companyCredentialRotationPreservesVerification},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := companyCredentialRotationPolicyForWeComInput(test.input); got != test.policy {
				t.Fatalf("policy=%d, want %d", got, test.policy)
			}
		})
	}
}

func TestWeComRotationGroupsAreIndependentFromBindingVersion(t *testing.T) {
	employeeSecret := "employee-secret"
	callbackToken := "callback-token"
	chatSecret := "chat-secret"
	checks := []struct {
		name  string
		input companyprofile.WeComCredentialsInput
		want  companyCredentialRotationGroups
	}{
		{name: "employee", input: companyprofile.WeComCredentialsInput{EmployeeSecret: &employeeSecret}, want: companyCredentialRotationGroups{Employee: true}},
		{name: "callback", input: companyprofile.WeComCredentialsInput{CallbackToken: &callbackToken}, want: companyCredentialRotationGroups{Callback: true}},
		{name: "archive", input: companyprofile.WeComCredentialsInput{ChatSecret: &chatSecret}, want: companyCredentialRotationGroups{}},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if got := credentialRotationGroupsForWeComInput(check.input); got != check.want {
				t.Fatalf("groups=%+v want %+v", got, check.want)
			}
		})
	}
}
