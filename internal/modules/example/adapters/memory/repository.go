package memory

import (
	"context"
	"sync"

	"jiyi/mochat-go/internal/modules/example/domain"
)

type Repository struct {
	mu      sync.RWMutex
	modules map[string]domain.Module
}

func NewRepository() *Repository {
	return &Repository{modules: make(map[string]domain.Module)}
}

func (r *Repository) Save(_ context.Context, module domain.Module) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.modules[module.ID] = module
	return nil
}

func (r *Repository) Find(_ context.Context, id string) (domain.Module, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	module, ok := r.modules[id]
	return module, ok, nil
}
