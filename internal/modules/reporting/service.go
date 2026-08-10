package reporting

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

func NewSQLService(db *sql.DB) *Service {
	return NewService(map[ReportKind]Source{
		CustomerReport:   NewSQLRepository(db, CustomerReport),
		EmployeeReport:   NewSQLRepository(db, EmployeeReport),
		ConversionReport: NewSQLRepository(db, ConversionReport),
		BehaviorReport:   NewSQLRepository(db, BehaviorReport),
		DetailReport:     NewSQLRepository(db, DetailReport),
		OverviewReport:   NewSQLRepository(db, OverviewReport),
	})
}

var ErrInvalidQuery = errors.New("invalid report query")

type Source interface {
	Query(context.Context, ReportQuery) (ReportResult, error)
}

type Service struct {
	sources map[ReportKind]Source
}

func NewService(sources map[ReportKind]Source) *Service {
	copyOfSources := make(map[ReportKind]Source, len(sources))
	for kind, source := range sources {
		copyOfSources[kind] = source
	}
	return &Service{sources: copyOfSources}
}

func (s *Service) Query(ctx context.Context, kind ReportKind, query ReportQuery) (ReportResult, error) {
	if query.TenantID <= 0 || query.CorpID <= 0 || query.Page <= 0 || query.PageSize <= 0 || query.PageSize > 200 {
		return ReportResult{}, ErrInvalidQuery
	}
	if _, err := time.LoadLocation(query.Timezone); err != nil || !query.StartAt.Before(query.EndAt) {
		return ReportResult{}, ErrInvalidQuery
	}
	if kind == ConversionReport && query.Stage != "" {
		switch query.Stage {
		case "lead", "contact", "opportunity", "won", "order":
		default:
			return ReportResult{}, ErrInvalidQuery
		}
	}
	source, ok := s.sources[kind]
	if !ok || source == nil {
		return ReportResult{}, fmt.Errorf("%w: unsupported report kind", ErrInvalidQuery)
	}
	query.EmployeeIDs = intersectIDs(query.EmployeeIDs, query.AllowedEmployeeIDs, query.EmployeeScopeRestricted)
	return source.Query(ctx, query)
}

func intersectIDs(requested, allowed []int64, restricted bool) []int64 {
	if len(allowed) == 0 {
		if restricted {
			return []int64{0}
		}
		return requested
	}
	set := make(map[int64]struct{}, len(allowed))
	for _, id := range allowed {
		set[id] = struct{}{}
	}
	result := make([]int64, 0, len(requested))
	for _, id := range requested {
		if _, ok := set[id]; ok {
			result = append(result, id)
		}
	}
	return result
}

type unavailableSource struct{ limitation Limitation }

func UnavailableSource(provider, message string) Source {
	return unavailableSource{limitation: Limitation{Provider: provider, Code: "provider_unavailable", Message: message}}
}

func (source unavailableSource) Query(_ context.Context, query ReportQuery) (ReportResult, error) {
	return ReportResult{
		Summary: map[string]*float64{}, Series: []SeriesPoint{}, Dimensions: []Dimension{}, Items: []map[string]any{},
		Pagination:  Pagination{Page: query.Page, PageSize: query.PageSize},
		Freshness:   Freshness{Provider: source.limitation.Provider, Status: "unavailable"},
		Limitations: []Limitation{source.limitation},
	}, nil
}
