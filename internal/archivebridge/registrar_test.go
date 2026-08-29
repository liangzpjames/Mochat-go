package archivebridge

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"jiyi/mochat-go/internal/archivefixture"
	"jiyi/mochat-go/internal/wecomarchivedemo"
)

type fakeProductionBindingSource struct {
	bindings []Binding
	err      error
}

func (s fakeProductionBindingSource) ProductionBindings(context.Context) ([]Binding, error) {
	return append([]Binding(nil), s.bindings...), s.err
}

type registrarFinanceDriver struct {
	seq      uint64
	limit    uint32
	mediaID  string
	indexBuf string
}

func (d *registrarFinanceDriver) FetchPage(_ context.Context, seq uint64, limit uint32) (wecomarchivedemo.ArchivePage, error) {
	d.seq, d.limit = seq, limit
	return wecomarchivedemo.ArchivePage{StartSeq: seq, NextSeq: seq + 1, Messages: []json.RawMessage{json.RawMessage(`{"msgid":"finance-registered","msgtype":"text"}`)}}, nil
}

func (d *registrarFinanceDriver) FetchMediaWithTimeout(_ context.Context, mediaID, indexBuf string, _ int) (wecomarchivedemo.MediaChunk, error) {
	d.mediaID, d.indexBuf = mediaID, indexBuf
	return wecomarchivedemo.MediaChunk{Data: []byte("registered-media"), Finished: true}, nil
}

type registrarDataZoneDriver struct {
	item archivefixture.DataZoneMessage
}

func (d *registrarDataZoneDriver) Fetch(_ string, _ int64, _ int) ([]archivefixture.DataZoneMessage, error) {
	return []archivefixture.DataZoneMessage{d.item}, nil
}

func (d *registrarDataZoneDriver) DecryptSecretKey(item archivefixture.DataZoneMessage) ([]byte, error) {
	if item.MessageID != d.item.MessageID || item.EncryptedSecretKey != d.item.EncryptedSecretKey {
		return nil, errors.New("component locator mismatch")
	}
	return []byte("controlled-session-key"), nil
}

func (d *registrarDataZoneDriver) Render(_ string, messageID string, secret []byte) (archivefixture.DataZoneContent, error) {
	if messageID != d.item.MessageID || string(secret) != "controlled-session-key" {
		return archivefixture.DataZoneContent{}, errors.New("component session mismatch")
	}
	return archivefixture.DataZoneContent{Type: "file", FileName: "contract.txt", MIMEType: "text/plain", Body: []byte("registered-component")}, nil
}

type recordingCloser struct {
	id     string
	closed *[]string
	err    error
}

func (c *recordingCloser) Close() error {
	*c.closed = append(*c.closed, c.id)
	return c.err
}

type fakeDriverFactory struct {
	finance     FinanceDriver
	dataZone    DataZoneDriver
	closed      *[]string
	financeErr  error
	dataZoneErr error
}

type errorAndCloserFactory struct {
	driver   FinanceDriver
	buildErr error
	closeErr error
	closed   *[]string
}

func (f errorAndCloserFactory) NewFinanceDriver(context.Context, Binding) (FinanceDriver, io.Closer, error) {
	return f.driver, &recordingCloser{id: "factory-finance", closed: f.closed, err: f.closeErr}, f.buildErr
}

func (errorAndCloserFactory) NewDataZoneDriver(context.Context, Binding) (DataZoneDriver, io.Closer, error) {
	return nil, nil, errors.New("not used")
}

func (f fakeDriverFactory) NewFinanceDriver(context.Context, Binding) (FinanceDriver, io.Closer, error) {
	if f.financeErr != nil {
		return nil, nil, f.financeErr
	}
	return f.finance, &recordingCloser{id: "finance", closed: f.closed}, nil
}

func (f fakeDriverFactory) NewDataZoneDriver(context.Context, Binding) (DataZoneDriver, io.Closer, error) {
	if f.dataZoneErr != nil {
		return nil, nil, f.dataZoneErr
	}
	return f.dataZone, &recordingCloser{id: "data-zone", closed: f.closed}, nil
}

func TestDriverRegistrarRegistersBothModesAndServesBridgeContracts(t *testing.T) {
	financeBinding := Binding{TenantID: 11, CorpID: 27, WXCorpID: "ww-self", IntegrationMode: ModeSelfBuilt}
	dataZoneBinding := Binding{TenantID: 21, CorpID: 37, WXCorpID: "ww-delegated", IntegrationMode: ModeThirdPartyDelegated}
	item := archivefixture.DataZoneMessage{Sequence: 8, MessageID: "delegated-registered", Type: "file", Sender: "external", Receivers: []string{"staff"}, SentAt: time.Unix(1, 0), PublicKeyVersion: 3, EncryptedSecretKey: "encrypted-controlled-key"}
	finance := &registrarFinanceDriver{}
	dataZone := &registrarDataZoneDriver{item: item}
	closed := []string{}
	store := NewStore()
	registrar, err := NewDriverRegistrar(fakeProductionBindingSource{bindings: []Binding{financeBinding, dataZoneBinding}}, fakeDriverFactory{finance: finance, dataZone: dataZone, closed: &closed}, store)
	if err != nil {
		t.Fatal(err)
	}
	count, err := registrar.RegisterAll(context.Background())
	if err != nil || count != 2 || store.DriverCount() != 2 {
		t.Fatalf("count=%d store=%d err=%v", count, store.DriverCount(), err)
	}

	handler, err := NewHandler(Config{BearerToken: testBridgeBearer}, store)
	if err != nil {
		t.Fatal(err)
	}
	messages := postBridge(t, handler, "/v1/archive/messages", map[string]any{
		"tenant_id": 11, "corp_id": 27, "wx_corpid": "ww-self", "integration_mode": ModeSelfBuilt, "seq": 41, "limit": 9,
	})
	if messages.Code != http.StatusOK || finance.seq != 41 || finance.limit != 9 || !strings.Contains(messages.Body.String(), "finance-registered") {
		t.Fatalf("messages=%d %s seq=%d limit=%d", messages.Code, messages.Body.String(), finance.seq, finance.limit)
	}
	media := postBridge(t, handler, "/v1/archive/media/chunks", map[string]any{
		"tenant_id": 11, "corp_id": 27, "wx_corpid": "ww-self", "integration_mode": ModeSelfBuilt, "sdkFileId": "media-registered", "indexBuf": "chunk-2", "timeoutSeconds": 5,
	})
	if media.Code != http.StatusOK || finance.mediaID != "media-registered" || finance.indexBuf != "chunk-2" || !strings.Contains(media.Body.String(), "cmVnaXN0ZXJlZC1tZWRpYQ==") {
		t.Fatalf("media=%d %s call=%s/%s", media.Code, media.Body.String(), finance.mediaID, finance.indexBuf)
	}
	component := postBridge(t, handler, "/v1/archive/component/session", map[string]any{
		"tenant_id": 21, "corp_id": 37, "wx_corpid": "ww-delegated", "integration_mode": ModeThirdPartyDelegated,
		"msgid": item.MessageID, "public_key_ver": item.PublicKeyVersion, "encrypted_secret_key": item.EncryptedSecretKey,
	})
	if component.Code != http.StatusOK || !strings.Contains(component.Body.String(), "cmVnaXN0ZXJlZC1jb21wb25lbnQ=") {
		t.Fatalf("component=%d %s", component.Code, component.Body.String())
	}
	mismatch := postBridge(t, handler, "/v1/archive/messages", map[string]any{
		"tenant_id": 11, "corp_id": 27, "wx_corpid": "ww-other", "integration_mode": ModeSelfBuilt, "seq": 0, "limit": 1,
	})
	if mismatch.Code != http.StatusConflict || !strings.Contains(mismatch.Body.String(), "ARCHIVE_BINDING_MISMATCH") {
		t.Fatalf("mismatch=%d %s", mismatch.Code, mismatch.Body.String())
	}

	if err := registrar.Close(); err != nil {
		t.Fatal(err)
	}
	if strings.Join(closed, ",") != "data-zone,finance" || store.DriverCount() != 0 {
		t.Fatalf("close order=%v store=%d", closed, store.DriverCount())
	}
}

func TestDriverRegistrarFailsClosedAndRollsBackInitializedDrivers(t *testing.T) {
	financeBinding := Binding{TenantID: 11, CorpID: 27, WXCorpID: "ww-self", IntegrationMode: ModeSelfBuilt}
	dataZoneBinding := Binding{TenantID: 21, CorpID: 37, WXCorpID: "ww-delegated", IntegrationMode: ModeThirdPartyDelegated}
	closed := []string{}
	store := NewStore()
	registrar, err := NewDriverRegistrar(fakeProductionBindingSource{bindings: []Binding{financeBinding, dataZoneBinding}}, fakeDriverFactory{
		finance: &registrarFinanceDriver{}, closed: &closed, dataZoneErr: errors.New("controlled initialization failure"),
	}, store)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registrar.RegisterAll(context.Background()); ErrorCode(err) != "ARCHIVE_DRIVER_INITIALIZATION_FAILED" {
		t.Fatalf("initialization error=%v code=%s", err, ErrorCode(err))
	}
	if strings.Join(closed, ",") != "finance" || store.DriverCount() != 0 {
		t.Fatalf("rollback close=%v store=%d", closed, store.DriverCount())
	}
}

func TestDriverRegistrarRejectsZeroDriversAndBindingConflicts(t *testing.T) {
	store := NewStore()
	empty, err := NewDriverRegistrar(fakeProductionBindingSource{}, fakeDriverFactory{}, store)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := empty.RegisterAll(context.Background()); ErrorCode(err) != "ARCHIVE_DRIVER_UNAVAILABLE" {
		t.Fatalf("zero-driver error=%v code=%s", err, ErrorCode(err))
	}

	binding := Binding{TenantID: 11, CorpID: 27, WXCorpID: "ww-self", IntegrationMode: ModeSelfBuilt}
	conflict := binding
	conflict.WXCorpID = "ww-conflict"
	closed := []string{}
	duplicate, err := NewDriverRegistrar(fakeProductionBindingSource{bindings: []Binding{binding, conflict}}, fakeDriverFactory{finance: &registrarFinanceDriver{}, closed: &closed}, NewStore())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := duplicate.RegisterAll(context.Background()); ErrorCode(err) != "ARCHIVE_BINDING_CONFLICT" {
		t.Fatalf("conflict error=%v code=%s", err, ErrorCode(err))
	}
	if len(closed) != 0 {
		t.Fatalf("factory ran before conflict validation: closed=%v", closed)
	}
}

func TestDriverRegistrarCloseAttemptsEveryDriverAndReturnsControlledError(t *testing.T) {
	bindings := []Binding{
		{TenantID: 11, CorpID: 27, WXCorpID: "ww-self", IntegrationMode: ModeSelfBuilt},
		{TenantID: 21, CorpID: 37, WXCorpID: "ww-delegated", IntegrationMode: ModeThirdPartyDelegated},
	}
	closed := []string{}
	factory := fakeDriverFactory{finance: &registrarFinanceDriver{}, dataZone: &registrarDataZoneDriver{}, closed: &closed}
	registrar, err := NewDriverRegistrar(fakeProductionBindingSource{bindings: bindings}, factory, NewStore())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registrar.RegisterAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	registrar.registered[1].closer = &recordingCloser{id: "data-zone", closed: &closed, err: errors.New("controlled close failure")}
	if err := registrar.Close(); ErrorCode(err) != "ARCHIVE_DRIVER_CLOSE_FAILED" {
		t.Fatalf("close error=%v code=%s", err, ErrorCode(err))
	}
	if strings.Join(closed, ",") != "data-zone,finance" {
		t.Fatalf("close order=%v", closed)
	}
}

func TestDriverRegistrarPreservesFactoryErrorAndCloserFailure(t *testing.T) {
	binding := Binding{TenantID: 11, CorpID: 27, WXCorpID: "ww-self", IntegrationMode: ModeSelfBuilt}
	buildErr := errors.New("controlled factory initialization failure")
	closeErr := errors.New("controlled factory closer failure")
	closed := []string{}
	registrar, err := NewDriverRegistrar(fakeProductionBindingSource{bindings: []Binding{binding}}, errorAndCloserFactory{
		buildErr: buildErr, closeErr: closeErr, closed: &closed,
	}, NewStore())
	if err != nil {
		t.Fatal(err)
	}
	_, err = registrar.RegisterAll(context.Background())
	if ErrorCode(err) != "ARCHIVE_DRIVER_INITIALIZATION_FAILED" || !errors.Is(err, buildErr) || !errors.Is(err, closeErr) {
		t.Fatalf("error=%v code=%s build=%t close=%t", err, ErrorCode(err), errors.Is(err, buildErr), errors.Is(err, closeErr))
	}
	if strings.Join(closed, ",") != "factory-finance" {
		t.Fatalf("closed=%v", closed)
	}
}

func TestDriverRegistrarPreservesStoreConflictAndCloserFailure(t *testing.T) {
	existing := Binding{TenantID: 11, CorpID: 27, WXCorpID: "ww-existing", IntegrationMode: ModeSelfBuilt}
	incoming := Binding{TenantID: 11, CorpID: 27, WXCorpID: "ww-incoming", IntegrationMode: ModeSelfBuilt}
	store := NewStore()
	if err := store.RegisterFinance(existing, &registrarFinanceDriver{}); err != nil {
		t.Fatal(err)
	}
	closeErr := errors.New("controlled store-conflict closer failure")
	closed := []string{}
	registrar, err := NewDriverRegistrar(fakeProductionBindingSource{bindings: []Binding{incoming}}, errorAndCloserFactory{
		driver: &registrarFinanceDriver{}, closeErr: closeErr, closed: &closed,
	}, store)
	if err != nil {
		t.Fatal(err)
	}
	_, err = registrar.RegisterAll(context.Background())
	if ErrorCode(err) != "ARCHIVE_BINDING_CONFLICT" || !errors.Is(err, closeErr) {
		t.Fatalf("error=%v code=%s close=%t", err, ErrorCode(err), errors.Is(err, closeErr))
	}
	if strings.Join(closed, ",") != "factory-finance" {
		t.Fatalf("closed=%v", closed)
	}
}
