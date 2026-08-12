package dashboard

type CorpDetail struct {
	ID             int
	Name           string
	WxCorpID       string
	SocialCode     string
	EmployeeSecret string
	EventCallback  string
	ContactSecret  string
	Token          string
	EncodingAESKey string
	TenantID       int
	CreatedAt      string
	UpdatedAt      string
}

type CorpListPage struct {
	Items     []CorpDetail
	Total     int
	TotalPage int
}

type CorpListFilter struct {
	TenantID   int
	CorpIDs    []int
	CorpName   string
	Page       int
	PerPage    int
	SuperAdmin bool
}

type CorpUpdateValues struct {
	Name           string
	WxCorpID       string
	EmployeeSecret string
	ContactSecret  string
}

type CorpCreateValues struct {
	Name           string
	WxCorpID       string
	EmployeeSecret string
	ContactSecret  string
	EventCallback  string
	Token          string
	EncodingAESKey string
	TenantID       int
}
