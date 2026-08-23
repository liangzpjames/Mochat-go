// Package ports defines the contracts owned by the AI settings module.
package ports

import (
	"context"
	"errors"
	"io"
)

var (
	ErrKnowledgeBaseInvalid      = errors.New("AI settings knowledge base invalid")
	ErrKnowledgeBaseReferenced   = errors.New("AI settings knowledge base referenced")
	ErrKnowledgeBaseHasDocuments = errors.New("AI settings knowledge base has documents")
	ErrNotFound                  = errors.New("AI settings record not found")
	ErrDocumentLimit             = errors.New("AI settings document limit reached")
	ErrDocumentDuplicate         = errors.New("AI settings document duplicate")
	ErrUnsupportedDocumentType   = errors.New("AI settings document type unsupported")
	ErrDocumentTooLarge          = errors.New("AI settings document too large")
	ErrDocumentTextTooLarge      = errors.New("AI settings document text too large")
	ErrDocumentUnreadable        = errors.New("AI settings document unreadable")
	ErrUnsafeObjectKey           = errors.New("AI settings object key unsafe")
)

const (
	DocumentStatusReady           = "ready"
	DocumentStatusFailed          = "failed"
	SessionAnalysisSystemKey      = "session-analysis"
	SessionAnalysisAssistantName  = "会话分析助手"
	DefaultSmartAnalysisSystemKey = "default-smart-analysis"
	DefaultSmartAnalysisRuleName  = "默认智能分析规则"
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

type KnowledgeBaseHasDocumentsError struct {
	Count int
}

func (e *KnowledgeBaseHasDocumentsError) Error() string {
	return ErrKnowledgeBaseHasDocuments.Error()
}

func (e *KnowledgeBaseHasDocumentsError) Is(target error) bool {
	return target == ErrKnowledgeBaseHasDocuments
}

type KnowledgeBase struct {
	ID                 string `json:"id"`
	TenantID           int64  `json:"-"`
	CorpID             int64  `json:"corpId"`
	Name               string `json:"name"`
	Description        string `json:"description"`
	DocumentCount      int    `json:"documentCount"`
	ReadyDocumentCount int    `json:"readyDocumentCount"`
	Status             int    `json:"status"`
	CreatedBy          int64  `json:"-"`
	UpdatedBy          int64  `json:"-"`
	CreatedAt          string `json:"createdAt"`
	UpdatedAt          string `json:"updatedAt"`
}

type Agent struct {
	ID                 string             `json:"id"`
	TenantID           int64              `json:"-"`
	CorpID             int64              `json:"corpId"`
	SystemKey          string             `json:"systemKey,omitempty"`
	Name               string             `json:"name"`
	Description        string             `json:"description"`
	KnowledgeBaseIDs   []string           `json:"knowledgeBaseIds"`
	KnowledgeBaseCount int                `json:"knowledgeBaseCount"`
	ReadyDocumentCount int                `json:"readyDocumentCount"`
	Status             int                `json:"status"`
	CreatedBy          int64              `json:"-"`
	UpdatedBy          int64              `json:"-"`
	CreatedAt          string             `json:"createdAt"`
	UpdatedAt          string             `json:"updatedAt"`
	SmartAnalysisRule  *SmartAnalysisRule `json:"smartAnalysisRule,omitempty"`
}

type SmartAnalysisRule struct {
	ID                int64    `json:"id"`
	Name              string   `json:"name"`
	Objective         string   `json:"objective"`
	ConversationTypes []string `json:"conversationTypes"`
	LookbackDays      int      `json:"lookbackDays"`
	MinimumMessages   int      `json:"minimumMessages"`
	CurrentVersion    int      `json:"currentVersion"`
	UpdatedAt         string   `json:"updatedAt"`
}

type KnowledgeDocument struct {
	ID              string `json:"id"`
	TenantID        int64  `json:"-"`
	CorpID          int64  `json:"corpId"`
	KnowledgeBaseID string `json:"knowledgeBaseId"`
	Filename        string `json:"filename"`
	Extension       string `json:"extension"`
	MIMEType        string `json:"mimeType"`
	ObjectKey       string `json:"-"`
	SizeBytes       int64  `json:"sizeBytes"`
	SHA256          string `json:"sha256"`
	Status          string `json:"status"`
	ErrorSummary    string `json:"errorSummary"`
	CharacterCount  int    `json:"characterCount"`
	ChunkCount      int    `json:"chunkCount"`
	CreatedBy       int64  `json:"-"`
	UpdatedBy       int64  `json:"-"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
}

type KnowledgeChunk struct {
	ID              int64  `json:"-"`
	TenantID        int64  `json:"-"`
	CorpID          int64  `json:"-"`
	KnowledgeBaseID string `json:"-"`
	DocumentID      string `json:"-"`
	DocumentName    string `json:"-"`
	Ordinal         int    `json:"-"`
	Content         string `json:"-"`
	CharacterCount  int    `json:"-"`
}

type DocumentUpload struct {
	Filename  string
	MIMEType  string
	SizeBytes int64
	Reader    io.Reader
}

type ParsedDocument struct {
	Text           string
	SHA256         string
	CharacterCount int
	Extension      string
	MIMEType       string
}

type DocumentStorage interface {
	Stage(io.Reader) (string, int64, string, error)
	Commit(string, string) (string, error)
	Quarantine(string) (string, error)
	Restore(string, string) error
	PurgeQuarantine(string) error
	Delete(string) error
	RemoveStaged(string)
}

type SessionAssistantContext struct {
	AgentID             string
	Name                string
	Instructions        string
	Enabled             bool
	KnowledgeBaseCount  int
	ReadyDocumentCount  int
	KnowledgeChunks     []KnowledgeChunk
	SettingsFingerprint string
	UpdatedAt           string
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

type DocumentRepository interface {
	List(context.Context, int64, int64, string) ([]KnowledgeDocument, error)
	Get(context.Context, int64, int64, string, string) (KnowledgeDocument, error)
	Count(context.Context, int64, int64, string) (int, error)
	Create(context.Context, KnowledgeDocument, []KnowledgeChunk) (KnowledgeDocument, error)
	Delete(context.Context, int64, int64, int64, string, string) (KnowledgeDocument, error)
}

type SessionAssistantRepository interface {
	EnsureSessionAssistant(context.Context, int64, int64, int64, string) (Agent, error)
	GetSessionAssistant(context.Context, int64, int64) (Agent, error)
	UpdateSessionAssistant(context.Context, Agent) (Agent, error)
	LoadSessionAssistantContext(context.Context, int64, int64) (SessionAssistantContext, error)
}

type IDGenerator interface {
	Generate() string
}
