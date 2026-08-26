// Package archive contains the source and synchronization boundary for
// conversation archives. Persistence and Dashboard presentation stay outside
// this package so a future real WeCom source can replace simulation without
// changing consumers.
package archive

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"jiyi/mochat-go/internal/modules/providers"
)

const (
	DefaultFetchLimit = 100
	simulationPrefix  = "MOCHAT-SIM:"
)

var (
	ErrInvalidScope = errors.New("archive source scope is invalid")
	ErrInvalidRunID = errors.New("archive simulation run id is invalid")
	runIDPattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,79}$`)
)

// Scope is the only authority accepted by an ArchiveSource. Callers must
// derive it from the authenticated tenant/corp context, never from a message
// body or query parameter.
type Scope struct {
	TenantID int64
	CorpID   int64
}

func (s Scope) valid() bool { return s.TenantID > 0 && s.CorpID > 0 }

// Cursor is source-neutral. Sequence is used by the current WeCom-shaped
// sources; Token remains available for a future opaque external cursor.
type Cursor struct {
	Sequence int64
	Token    string
}

// Message is the normalized source boundary. Source identity is carried with
// every message before persistence, so a simulated row cannot silently become
// an external archive row.
type Message struct {
	Source      providers.Source
	SourceID    string
	Namespace   string
	MsgID       string
	Seq         int64
	Action      string
	From        string
	ToList      []string
	RoomID      string
	MsgType     string
	MsgTime     time.Time
	ContentRaw  string
	ContentText string
	RawJSON     string
	// Media is an internal persistence descriptor. SDKFileID must never be
	// serialized into Dashboard message content, URLs, or logs.
	Media []MediaDescriptor `json:"-"`
}

// MediaDescriptor carries the private bridge locator separately from the
// sanitized message representation. The store encrypts SDKFileID before the
// message transaction commits.
type MediaDescriptor struct {
	Type         string `json:"type"`
	SDKFileID    string `json:"-"`
	FileName     string `json:"fileName,omitempty"`
	MIMEType     string `json:"mimeType,omitempty"`
	ExpectedSize int64  `json:"expectedSize,omitempty"`
	ExpectedMD5  string `json:"expectedMd5,omitempty"`
}

type Page struct {
	Messages   []Message
	NextCursor Cursor
	HasMore    bool
}

// ArchiveSource is replaceable at the fetch boundary. An implementation must
// expose an explicit source kind, source id, and namespace and must scope every
// fetch to the supplied tenant/corp pair.
type ArchiveSource interface {
	Kind() providers.Source
	SourceID() string
	Namespace() string
	Status() providers.Status
	Fetch(context.Context, Scope, Cursor, int) (Page, error)
}

// ExternalSource is deliberately fail-closed. It is a composition placeholder
// for a future paid getchatdata implementation and never reports a successful
// page merely because credentials exist.
type ExternalSource struct {
	sourceID  string
	namespace string
	status    providers.Status
}

var _ ArchiveSource = (*ExternalSource)(nil)

func NewExternalSource(sourceID, namespace string, status providers.Status) (*ExternalSource, error) {
	sourceID = strings.TrimSpace(sourceID)
	namespace = strings.TrimSpace(namespace)
	if sourceID == "" || namespace == "" {
		return nil, fmt.Errorf("external archive source identity is required")
	}
	status.Source = providers.SourceExternal
	status.Capabilities = []string{"archive_sync"}
	if status.State == providers.StateReady || status.State == "" {
		status.State = providers.StateLimited
	}
	if strings.TrimSpace(status.Code) == "" || status.Code == "archive.credentials_missing" {
		if status.Code == "" {
			status.Code = "archive.getchatdata_unimplemented"
		}
	}
	if status.Reason == "" {
		status.Reason = "真实会话存档 getchatdata source 尚未实现"
	}
	if status.Action == "" {
		status.Action = "接入并验证真实会话存档 source 后再启用同步"
	}
	return &ExternalSource{sourceID: sourceID, namespace: namespace, status: status}, nil
}

func (s *ExternalSource) Kind() providers.Source { return providers.SourceExternal }

func (s *ExternalSource) SourceID() string {
	if s == nil {
		return ""
	}
	return s.sourceID
}

func (s *ExternalSource) Namespace() string {
	if s == nil {
		return ""
	}
	return s.namespace
}

func (s *ExternalSource) Status() providers.Status {
	if s == nil {
		return providers.Status{Source: providers.SourceExternal, State: providers.StateUnavailable, Code: "archive.source_unavailable"}
	}
	if s.status.Code == "archive.credentials_missing" {
		return providers.Status{
			Kind: "wecom_archive", Source: providers.SourceExternal, State: providers.StateLimited,
			Code: "archive.credentials_missing", Reason: "external archive credentials are incomplete",
			Action: "configure and verify the external archive credentials", Missing: s.status.Missing,
		}
	}
	return providers.Status{
		Kind: "wecom_archive", Source: providers.SourceExternal, State: providers.StateLimited,
		Code: "archive.getchatdata_unimplemented", Reason: "real getchatdata source is not implemented",
		Action: "connect and verify the real archive source before enabling sync",
	}
}

func (s *ExternalSource) Fetch(ctx context.Context, scope Scope, _ Cursor, _ int) (Page, error) {
	if ctx == nil {
		return Page{}, errors.New("context is required")
	}
	if !scope.valid() {
		return Page{}, ErrInvalidScope
	}
	if s != nil && s.status.Code == "archive.credentials_missing" {
		return Page{}, providers.ErrNotConfigured
	}
	return Page{}, providers.ErrCapabilityUnavailable
}

// SimulationSource is an explicit, deterministic source for local acceptance
// runs. It never calls WeCom and uses a dedicated namespace for every run.
type SimulationSource struct {
	runID string
}

var _ ArchiveSource = (*SimulationSource)(nil)

func NewSimulationSource(runID string) (*SimulationSource, error) {
	runID = strings.TrimSpace(runID)
	if !runIDPattern.MatchString(runID) {
		return nil, ErrInvalidRunID
	}
	return &SimulationSource{runID: runID}, nil
}

func (s *SimulationSource) Kind() providers.Source { return providers.SourceSimulated }

func (s *SimulationSource) SourceID() string {
	if s == nil {
		return ""
	}
	return "simulation:" + s.runID
}

func (s *SimulationSource) Namespace() string {
	if s == nil {
		return ""
	}
	return simulationPrefix + s.runID
}

func (s *SimulationSource) Status() providers.Status {
	return providers.Status{
		Kind:         "wecom_archive",
		Source:       providers.SourceSimulated,
		State:        providers.StateLimited,
		Code:         "archive.simulation_ready",
		Reason:       "具名模拟会话存档 source 已准备，仅用于隔离验收数据",
		Action:       "仅在明确启用的 simulation run 中执行；不代表真实企微存档已接通",
		Capabilities: []string{"archive_sync"},
	}
}

func (s *SimulationSource) Fetch(ctx context.Context, scope Scope, cursor Cursor, limit int) (Page, error) {
	if ctx == nil {
		return Page{}, errors.New("context is required")
	}
	if !scope.valid() {
		return Page{}, ErrInvalidScope
	}
	if s == nil || s.runID == "" {
		return Page{}, ErrInvalidRunID
	}
	if limit <= 0 {
		limit = DefaultFetchLimit
	}
	all := BuildSimulationMessages(s.runID, time.Now().UTC())
	available := make([]Message, 0, len(all))
	for _, message := range all {
		if message.Seq > cursor.Sequence {
			available = append(available, message)
		}
	}
	page := Page{NextCursor: cursor}
	if len(available) == 0 {
		return page, nil
	}
	if len(available) > limit {
		page.Messages = append([]Message(nil), available[:limit]...)
		page.HasMore = true
	} else {
		page.Messages = append([]Message(nil), available...)
	}
	page.NextCursor.Sequence = page.Messages[len(page.Messages)-1].Seq
	return page, nil
}

// BuildSimulationMessages is shared by the source and the retained SQL
// simulator so both paths exercise the same message matrix.
func BuildSimulationMessages(runID string, now time.Time) []Message {
	now = now.UTC()
	type fixture struct {
		kind   string
		text   string
		action string
	}
	fixtures := []fixture{
		{kind: "text", text: "模拟验收关键词：客户咨询产品方案", action: "send"},
		{kind: "text", text: "模拟客户回复：请发送报价", action: "send"},
		{kind: "image", text: "模拟图片：产品截图", action: "send"},
		{kind: "file", text: "模拟文件：产品报价单.pdf", action: "send"},
		{kind: "voice", text: "模拟语音：三十秒需求说明", action: "send"},
		{kind: "video", text: "模拟视频：产品演示", action: "send"},
		{kind: "location", text: "模拟位置：上海市浦东新区", action: "send"},
		{kind: "card", text: "模拟名片：客户联系人", action: "send"},
		{kind: "link", text: "模拟链接：https://example.invalid/mochat-simulation", action: "send"},
		{kind: "text", text: "模拟内部会话：员工协作跟进", action: "send"},
		{kind: "text", text: "模拟群聊：欢迎加入验收群", action: "send"},
		{kind: "emotion", text: "模拟表情消息", action: "send"},
		{kind: "text", text: "模拟撤回消息", action: "revoke"},
	}
	hash := sha256.Sum256([]byte(runID))
	base := int64(4_000_000_000_000_000 + (binary.BigEndian.Uint64(hash[:8]) & ((1 << 50) - 1)))
	start := now.Add(-time.Duration(len(fixtures)) * time.Minute)
	namespace := simulationPrefix + runID
	sourceID := "simulation:" + runID
	messages := make([]Message, 0, len(fixtures))
	for index, fixture := range fixtures {
		from, to := "employee-a", []string{"contact"}
		roomID := ""
		switch index {
		case 1:
			from, to = "contact", []string{"employee-a"}
		case 9:
			from, to = "employee-a", []string{"employee-b"}
		case 10, 11, 12:
			from, to, roomID = "employee-a", []string{"employee-b", "contact"}, "room"
		}
		content, _ := json.Marshal(map[string]any{
			"msgtype": fixture.kind, "content": fixture.text, "simulation": true,
			"source_id": sourceID, "run_id": runID,
		})
		messages = append(messages, Message{
			Source: providers.SourceSimulated, SourceID: sourceID, Namespace: namespace,
			MsgID: fmt.Sprintf("%s:%03d", namespace, index+1), Seq: base + int64(index+1),
			Action: fixture.action, From: from, ToList: to, RoomID: roomID, MsgType: fixture.kind,
			MsgTime: start.Add(time.Duration(index) * time.Minute), ContentRaw: string(content),
			ContentText: fixture.text, RawJSON: string(content),
		})
	}
	return messages
}

// MessageTypes returns the deterministic simulation type set for completion
// tests and local acceptance tooling.
func MessageTypes() []string {
	set := map[string]bool{}
	for _, message := range BuildSimulationMessages("coverage", time.Unix(1, 0)) {
		set[message.MsgType] = true
	}
	result := make([]string, 0, len(set))
	for kind := range set {
		result = append(result, kind)
	}
	sort.Strings(result)
	return result
}
