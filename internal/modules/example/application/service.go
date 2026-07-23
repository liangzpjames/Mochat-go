package application

import (
	"context"
	"errors"
	"strings"

	"jiyi/mochat-go/internal/modules/example/domain"
	"jiyi/mochat-go/internal/modules/example/ports"
)

type Service struct {
	repository ports.Repository
}

func NewService(repository ports.Repository) Service {
	return Service{repository: repository}
}

func (s Service) Create(ctx context.Context, id, rawName string) (domain.Module, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.Module{}, errors.New("module id is required")
	}
	name, err := domain.NewModuleName(rawName)
	if err != nil {
		return domain.Module{}, err
	}
	module := domain.Module{ID: id, Name: name}
	if err := s.repository.Save(ctx, module); err != nil {
		return domain.Module{}, err
	}
	return module, nil
}
