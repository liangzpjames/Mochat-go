package ports

import (
	"context"
	"time"
)

type AudioObject struct {
	ID              int64      `json:"id"`
	TenantID        int64      `json:"tenantId"`
	UserID          int64      `json:"userId"`
	EmployeeID      int64      `json:"employeeId"`
	CorpID          int64      `json:"corpId"`
	OriginalName    string     `json:"originalName"`
	Source          string     `json:"source"`
	MessageID       string     `json:"messageId"`
	SenderName      string     `json:"senderName"`
	ReceiverName    string     `json:"receiverName"`
	RelativePath    string     `json:"-"`
	ContentType     string     `json:"contentType"`
	SizeBytes       int64      `json:"sizeBytes"`
	DurationSeconds int64      `json:"durationSeconds"`
	SHA256          string     `json:"sha256"`
	CreatedAt       time.Time  `json:"createdAt"`
	SyncedAt        *time.Time `json:"syncedAt,omitempty"`
	PlayURL         string     `json:"playUrl"`
	DeletedAt       *time.Time `json:"-"`
	DeletedBy       int64      `json:"-"`
}

type ListResult struct {
	List    []AudioObject `json:"list"`
	Total   int64         `json:"total"`
	Page    int           `json:"page"`
	PerPage int           `json:"perPage"`
}
type MediaListFilter struct {
	CorpID     int64
	Page       int
	PerPage    int
	Sender     string
	Receiver   string
	SyncedFrom string
	SyncedTo   string
}
type MediaStore interface {
	GetByID(context.Context, int64) (*AudioObject, error)
	List(context.Context, MediaListFilter) (ListResult, error)
}
type DurationUpdater interface {
	UpdateDuration(context.Context, int64, int64) error
}
