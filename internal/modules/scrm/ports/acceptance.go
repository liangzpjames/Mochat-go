package ports

import "context"

const AcceptancePrefix = "P35-ACCEPT-"

type AcceptanceScope struct {
	TenantID, CorpID, ActorID int64
	EnvironmentID             string
}

type AcceptanceResult struct {
	Prefix        string   `json:"prefix"`
	EnvironmentID string   `json:"environmentId,omitempty"`
	ResourceIDs   []string `json:"resourceIds,omitempty"`
	Count         int      `json:"count"`
}

type AcceptanceStore interface {
	Create(context.Context, AcceptanceScope) (AcceptanceResult, error)
	Verify(context.Context, AcceptanceScope) (AcceptanceResult, error)
	Cleanup(context.Context, AcceptanceScope) (AcceptanceResult, error)
}
