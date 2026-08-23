// Package ports defines the contracts owned by the AI settings module.
package ports

import (
	"context"
	"errors"
)

var (
	ErrKnowledgeBaseInvalid    = errors.New("AI settings knowledge base invalid")
	ErrKnowledgeBaseReferenced = errors.New("AI settings knowledge base referenced")
	ErrNotFound                = errors.New("AI settings record not found")
)

type KnowledgeBaseReferencedError struct {
	Count int
}

func (e *KnowledgeBaseReferencedError) Error() string {
	return ErrKnowledgeBaseReferenced.Error()
}

func (e *KnowledgeBaseReferencedError) Is(target error) bool {
	return target == ErrKnowledgeBaseReferenced
}

type KnowledgeBase struct {
	ID            string `json:"id"`
	TenantID      int64  `json:"-"`
	CorpID        int64  `json:"corpId"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	DocumentCount int    `json:"documentCount"`
	Status        int    `json:"status"`
	CreatedBy     int64  `json:"-"`
	UpdatedBy     int64  `json:"-"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}

type Agent struct {
	ID               string   `json:"id"`
	TenantID         int64    `json:"-"`
	CorpID           int64    `json:"corpId"`
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	KnowledgeBaseIDs []string `json:"knowledgeBaseIds"`
	Status           int      `json:"status"`
	CreatedBy        int64    `json:"-"`
	UpdatedBy        int64    `json:"-"`
	CreatedAt        string   `json:"createdAt"`
	UpdatedAt        string   `json:"updatedAt"`
}

type KnowledgeBaseRepository interface {
	List(context.Context, int64, int64) ([]KnowledgeBase, error)
	GetByIDs(context.Context, int64, int64, []string) ([]KnowledgeBase, error)
	Create(context.Context, KnowledgeBase) (KnowledgeBase, error)
	Update(context.Context, KnowledgeBase) (KnowledgeBase, error)
	Delete(context.Context, int64, int64, int64, string) error
}

type AgentRepository interface {
	List(context.Context, int64, int64) ([]Agent, error)
	ListReferencingKnowledgeBase(context.Context, int64, int64, string) ([]Agent, error)
	Create(context.Context, Agent) (Agent, error)
	Update(context.Context, Agent) (Agent, error)
	Delete(context.Context, int64, int64, int64, string) error
}

type IDGenerator interface {
	Generate() string
}
