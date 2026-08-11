package companyprofile

func PlaintextSecretRead(credentials Credentials) string {
	return credentials.EmployeeSecret
}

type Credentials struct {
	EmployeeSecret string
}
