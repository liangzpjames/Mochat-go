package dashboard

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"jiyi/mochat-go/internal/dashboardprincipal"
)

const archiveMediaTestID = "8ff7bf2d-5604-43bc-a600-3ec91d575085"

type fakeArchiveMediaContentStore struct {
	mu                     sync.Mutex
	beforeLookup           func()
	object                 ArchiveMediaContentObject
	found                  bool
	filter                 ArchiveMediaContentFilter
	calls                  int
	objectConversationType *int
	objectEmployeeID       int
}

type archiveDeadlineRecorder struct {
	*httptest.ResponseRecorder
	deadlines []time.Time
}

func (w *archiveDeadlineRecorder) SetWriteDeadline(deadline time.Time) error {
	w.deadlines = append(w.deadlines, deadline)
	return nil
}

func (store *fakeArchiveMediaContentStore) ArchiveMediaContent(_ context.Context, filter ArchiveMediaContentFilter) (ArchiveMediaContentObject, bool, error) {
	if store.beforeLookup != nil {
		store.beforeLookup()
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	store.calls++
	store.filter = filter
	if store.objectConversationType != nil {
		allowed := false
		for _, scope := range filter.ConversationScopes {
			if scope.ConversationType == *store.objectConversationType && (!scope.RestrictEmployeeIDs || containsArchiveMediaConversationType(scope.AllowedEmployeeIDs, store.objectEmployeeID)) {
				allowed = true
			}
		}
		if !allowed {
			return ArchiveMediaContentObject{}, false, nil
		}
	}
	return store.object, store.found, nil
}

func containsArchiveMediaConversationType(values []int, target int) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func awaitArchiveMediaSignals(t *testing.T, signals <-chan struct{}, count int, label string) {
	t.Helper()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for received := 0; received < count; received++ {
		select {
		case <-signals:
		case <-timer.C:
			t.Fatalf("%s: received %d/%d signals", label, received, count)
		}
	}
}

func TestArchiveMediaContentServesFullHeadAndSingleRanges(t *testing.T) {
	root := t.TempDir()
	path := writeArchiveMediaTestFile(t, root, []byte("0123456789"))
	store := &fakeArchiveMediaContentStore{found: true, object: ArchiveMediaContentObject{
		ID: archiveMediaTestID, MediaType: "voice", Name: "voice.wav", MIMEType: "audio/wav",
		Size: 10, Status: "ready", StoragePath: path, SHA256: archiveMediaTestSHA256([]byte("0123456789")),
	}}
	handler := NewArchiveMediaContentHandler(store, root)
	hashCalls := 0
	handler.snapshotCopy = func(writer io.Writer, reader io.Reader) (int64, error) {
		hashCalls++
		return io.Copy(writer, reader)
	}
	tests := []struct {
		name, method, header, body, length, contentRange string
		status                                           int
	}{
		{"full", "GET", "", "0123456789", "10", "", 200},
		{"head", "HEAD", "", "", "10", "", 200},
		{"range", "GET", "bytes=2-5", "2345", "4", "bytes 2-5/10", 206},
		{"head range", "HEAD", "bytes=2-5", "", "4", "bytes 2-5/10", 206},
		{"open", "GET", "bytes=7-", "789", "3", "bytes 7-9/10", 206},
		{"suffix", "GET", "bytes=-3", "789", "3", "bytes 7-9/10", 206},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := &archiveDeadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
			handler.ServeHTTP(response, archiveMediaRequest(test.method, test.header))
			if response.Code != test.status || response.Body.String() != test.body {
				t.Fatalf("status/body=%d/%q want=%d/%q", response.Code, response.Body.String(), test.status, test.body)
			}
			if response.Header().Get("Content-Length") != test.length || response.Header().Get("Content-Range") != test.contentRange {
				t.Fatalf("length/range=%q/%q", response.Header().Get("Content-Length"), response.Header().Get("Content-Range"))
			}
			if response.Header().Get("Content-Type") != "audio/wav" || response.Header().Get("X-Content-Type-Options") != "nosniff" || response.Header().Get("Cache-Control") != "private, no-store" {
				t.Fatalf("headers=%#v", response.Header())
			}
			if !strings.HasPrefix(response.Header().Get("Content-Disposition"), "inline;") {
				t.Fatalf("inline disposition=%q", response.Header().Get("Content-Disposition"))
			}
			if got, want := len(response.deadlines), map[bool]int{true: 1, false: 0}[test.method == http.MethodGet]; got != want || (got == 1 && !response.deadlines[0].IsZero()) {
				t.Fatalf("write deadlines=%+v method=%s", response.deadlines, test.method)
			}
		})
	}
	downloadRequest := archiveMediaRequest(http.MethodGet, "")
	downloadRequest.URL.RawQuery = "download=1"
	downloadResponse := httptest.NewRecorder()
	handler.ServeHTTP(downloadResponse, downloadRequest)
	if downloadResponse.Code != http.StatusOK || !strings.HasPrefix(downloadResponse.Header().Get("Content-Disposition"), "attachment;") {
		t.Fatalf("download status/disposition=%d/%q", downloadResponse.Code, downloadResponse.Header().Get("Content-Disposition"))
	}
	if store.filter.TenantID != 11 || store.filter.CorpID != 27 || !store.filter.RestrictEmployeeIDs || len(store.filter.AllowedEmployeeIDs) != 2 || !reflect.DeepEqual(store.filter.AllowedConversationTypes, []int{0, 1, 2}) {
		t.Fatalf("scope filter=%+v", store.filter)
	}
	if hashCalls != 7 {
		t.Fatalf("full/head/range integrity checks=%d want=7", hashCalls)
	}
}

func TestArchiveMediaContentScopesEveryConversationPermissionForGETAndHEAD(t *testing.T) {
	root := t.TempDir()
	path := writeArchiveMediaTestFile(t, root, []byte("payload"))
	permissions := []struct {
		code    string
		allowed []int
	}{
		{code: "dashboard.chat.v2_all", allowed: []int{0, 1, 2}},
		{code: "dashboard.chat.v2_staff", allowed: []int{0}},
		{code: "dashboard.chat.v2_customer", allowed: []int{1}},
		{code: "dashboard.chat.v2_group", allowed: []int{2}},
	}
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		for _, permission := range permissions {
			for conversationType := 0; conversationType <= 2; conversationType++ {
				name := method + "/" + permission.code + "/" + strconv.Itoa(conversationType)
				t.Run(name, func(t *testing.T) {
					objectType := conversationType
					store := &fakeArchiveMediaContentStore{found: true, objectConversationType: &objectType, objectEmployeeID: 31, object: ArchiveMediaContentObject{
						ID: archiveMediaTestID, MediaType: "file", Name: "payload.bin", MIMEType: "application/octet-stream",
						Size: 7, Status: "ready", StoragePath: path, SHA256: archiveMediaTestSHA256([]byte("payload")),
					}}
					request := archiveMediaRequest(method, "")
					access, _ := DashboardAccessFromContext(request.Context())
					access.PermissionCode = permission.code
					access.PermissionCodes = []string{permission.code}
					access.PermissionScopes = map[string]DataScope{permission.code: DataScopeDepartment}
					request = request.WithContext(WithDashboardAccessContext(request.Context(), access))
					response := httptest.NewRecorder()
					NewArchiveMediaContentHandler(store, root).ServeHTTP(response, request)
					want := http.StatusNotFound
					if containsArchiveMediaConversationType(permission.allowed, conversationType) {
						want = http.StatusOK
					}
					if response.Code != want {
						t.Fatalf("status=%d want=%d filter=%+v", response.Code, want, store.filter)
					}
					if !reflect.DeepEqual(store.filter.AllowedConversationTypes, permission.allowed) {
						t.Fatalf("allowed conversation types=%v want=%v", store.filter.AllowedConversationTypes, permission.allowed)
					}
				})
			}
		}
	}
}

func TestArchiveMediaContentDoesNotMergeDifferentPermissionDataScopes(t *testing.T) {
	root := t.TempDir()
	path := writeArchiveMediaTestFile(t, root, []byte("payload"))
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		for _, test := range []struct {
			name             string
			conversationType int
			employeeID       int
			want             int
		}{
			{name: "own staff", conversationType: 0, employeeID: 31, want: http.StatusOK},
			{name: "other staff remains hidden", conversationType: 0, employeeID: 999, want: http.StatusNotFound},
			{name: "group tenant scope", conversationType: 2, employeeID: 999, want: http.StatusOK},
			{name: "customer permission absent", conversationType: 1, employeeID: 31, want: http.StatusNotFound},
		} {
			t.Run(method+"/"+test.name, func(t *testing.T) {
				objectType := test.conversationType
				store := &fakeArchiveMediaContentStore{found: true, objectConversationType: &objectType, objectEmployeeID: test.employeeID, object: ArchiveMediaContentObject{
					ID: archiveMediaTestID, MediaType: "file", Name: "payload.bin", MIMEType: "application/octet-stream",
					Size: 7, Status: "ready", StoragePath: path, SHA256: archiveMediaTestSHA256([]byte("payload")),
				}}
				request := archiveMediaRequest(method, "")
				access, _ := DashboardAccessFromContext(request.Context())
				access.Scope = DataScopeTenant
				access.PermissionCode = "dashboard.chat.v2_group"
				access.PermissionCodes = []string{"dashboard.chat.v2_group", "dashboard.chat.v2_staff"}
				access.PermissionScopes = map[string]DataScope{
					"dashboard.chat.v2_staff": DataScopeSelf,
					"dashboard.chat.v2_group": DataScopeTenant,
				}
				request = request.WithContext(WithDashboardAccessContext(request.Context(), access))
				response := httptest.NewRecorder()
				NewArchiveMediaContentHandler(store, root).ServeHTTP(response, request)
				if response.Code != test.want {
					t.Fatalf("status=%d want=%d filter=%+v", response.Code, test.want, store.filter)
				}
			})
		}
	}
}

func TestArchiveMediaContentServesOnlyTheValidatedSnapshot(t *testing.T) {
	root := t.TempDir()
	path := writeArchiveMediaTestFile(t, root, []byte("trusted"))
	store := &fakeArchiveMediaContentStore{found: true, object: ArchiveMediaContentObject{
		ID: archiveMediaTestID, MediaType: "file", Name: "payload.bin", MIMEType: "application/octet-stream",
		Size: 7, Status: "ready", StoragePath: path, SHA256: archiveMediaTestSHA256([]byte("trusted")),
	}}
	handler := NewArchiveMediaContentHandler(store, root)
	handler.snapshotValidated = func(snapshotName string, snapshot os.FileInfo) {
		if strings.Contains(snapshotName, archiveMediaTestID) {
			t.Fatalf("snapshot filename leaked object locator: %q", snapshotName)
		}
		if runtime.GOOS != "windows" && snapshot.Mode().Perm() != 0o600 {
			t.Fatalf("snapshot permissions=%#o want=0600", snapshot.Mode().Perm())
		}
		if err := os.WriteFile(path, []byte("EVIL!!!"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, archiveMediaRequest(http.MethodGet, ""))
	if response.Code != http.StatusOK || response.Body.String() != "trusted" {
		t.Fatalf("status/body=%d/%q, want verified snapshot", response.Code, response.Body.String())
	}
	entries, err := os.ReadDir(filepath.Join(root, "archive-media"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != archiveMediaTestID {
		t.Fatalf("temporary snapshot was not cleaned: %v", entries)
	}
}

func TestArchiveMediaSnapshotConcurrencyCoversCopyAndHash(t *testing.T) {
	const requestCount = archiveMediaHashConcurrency * 2
	root := t.TempDir()
	path := writeArchiveMediaTestFile(t, root, []byte("payload"))
	lookupsReady := make(chan struct{}, requestCount)
	lookupsRelease := make(chan struct{})
	store := &fakeArchiveMediaContentStore{found: true, object: ArchiveMediaContentObject{
		ID: archiveMediaTestID, MediaType: "file", Name: "payload.bin", MIMEType: "application/octet-stream",
		Size: 7, Status: "ready", StoragePath: path, SHA256: archiveMediaTestSHA256([]byte("payload")),
	}}
	store.beforeLookup = func() {
		lookupsReady <- struct{}{}
		<-lookupsRelease
	}
	handler := NewArchiveMediaContentHandler(store, root)
	started, snapshotsRelease := make(chan struct{}, requestCount), make(chan struct{})
	var active, maximum atomic.Int32
	handler.snapshotCopy = func(writer io.Writer, reader io.Reader) (int64, error) {
		current := active.Add(1)
		defer active.Add(-1)
		for {
			observed := maximum.Load()
			if current <= observed || maximum.CompareAndSwap(observed, current) {
				break
			}
		}
		started <- struct{}{}
		<-snapshotsRelease
		return io.Copy(writer, reader)
	}
	var lookupsReleaseOnce, snapshotsReleaseOnce sync.Once
	releaseLookups := func() { lookupsReleaseOnce.Do(func() { close(lookupsRelease) }) }
	releaseSnapshots := func() { snapshotsReleaseOnce.Do(func() { close(snapshotsRelease) }) }
	var wait sync.WaitGroup
	t.Cleanup(func() {
		releaseLookups()
		releaseSnapshots()
		wait.Wait()
	})
	for index := 0; index < requestCount; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, archiveMediaRequest(http.MethodGet, ""))
			if response.Code != http.StatusOK {
				t.Errorf("status=%d", response.Code)
			}
		}()
	}
	awaitArchiveMediaSignals(t, lookupsReady, requestCount, "requests did not reach the archive media lookup barrier")
	releaseLookups()
	awaitArchiveMediaSignals(t, started, archiveMediaHashConcurrency, "copy/hash slots did not fill")
	if got := len(handler.hashSlots); got != archiveMediaHashConcurrency {
		t.Fatalf("occupied copy/hash slots=%d want=%d", got, archiveMediaHashConcurrency)
	}
	select {
	case <-started:
		t.Fatal("more than four snapshot copies entered the integrity boundary")
	default:
	}
	releaseSnapshots()
	wait.Wait()
	if maximum.Load() != archiveMediaHashConcurrency {
		t.Fatalf("maximum concurrent snapshot copies=%d want=%d", maximum.Load(), archiveMediaHashConcurrency)
	}
}

type blockingArchiveMediaResponseWriter struct {
	header  http.Header
	started chan<- struct{}
	release <-chan struct{}
}

func (writer *blockingArchiveMediaResponseWriter) Header() http.Header { return writer.header }
func (writer *blockingArchiveMediaResponseWriter) WriteHeader(int)     {}
func (writer *blockingArchiveMediaResponseWriter) Write(payload []byte) (int, error) {
	writer.started <- struct{}{}
	<-writer.release
	return len(payload), nil
}

func TestArchiveMediaSnapshotConcurrencyRemainsBoundedUntilResponseCleanup(t *testing.T) {
	root := t.TempDir()
	path := writeArchiveMediaTestFile(t, root, []byte("payload"))
	store := &fakeArchiveMediaContentStore{found: true, object: ArchiveMediaContentObject{
		ID: archiveMediaTestID, MediaType: "file", Name: "payload.bin", MIMEType: "application/octet-stream",
		Size: 7, Status: "ready", StoragePath: path, SHA256: archiveMediaTestSHA256([]byte("payload")),
	}}
	handler := NewArchiveMediaContentHandler(store, root)
	started, release := make(chan struct{}, archiveMediaHashConcurrency+1), make(chan struct{})
	var wait sync.WaitGroup
	for index := 0; index < archiveMediaHashConcurrency+1; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			handler.ServeHTTP(&blockingArchiveMediaResponseWriter{header: http.Header{}, started: started, release: release}, archiveMediaRequest(http.MethodGet, ""))
		}()
	}
	for index := 0; index < archiveMediaHashConcurrency; index++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("bounded response did not start")
		}
	}
	fifthStarted := false
	select {
	case <-started:
		fifthStarted = true
	case <-time.After(50 * time.Millisecond):
	}
	entries, err := os.ReadDir(filepath.Join(root, "archive-media"))
	if err != nil {
		close(release)
		wait.Wait()
		t.Fatal(err)
	}
	close(release)
	wait.Wait()
	if fifthStarted {
		t.Fatal("fifth response retained an additional live snapshot before earlier cleanup")
	}
	if len(entries) != archiveMediaHashConcurrency+1 {
		t.Fatalf("live archive files=%d want source+%d snapshots", len(entries), archiveMediaHashConcurrency)
	}
}

func TestArchiveMediaContentRejectsUnsupportedMethod(t *testing.T) {
	response := httptest.NewRecorder()
	NewArchiveMediaContentHandler(&fakeArchiveMediaContentStore{}, t.TempDir()).ServeHTTP(response, archiveMediaRequest(http.MethodPost, ""))
	if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("status/allow=%d/%q", response.Code, response.Header().Get("Allow"))
	}
}

func TestArchiveMediaContentFailsClosedBeforeLookup(t *testing.T) {
	store := &fakeArchiveMediaContentStore{}
	handler := NewArchiveMediaContentHandler(store, t.TempDir())
	principal := dashboardprincipal.DashboardPrincipal{UserID: 5, TenantID: 11, CorpID: 27, CorpStatus: dashboardprincipal.CorpBindingStatusActive, AuthVersion: 1}
	access := DashboardAccessContext{UserID: 5, TenantID: 11, CorpID: 27, PermissionCodes: []string{"dashboard.chat.v2_all"}, PermissionScopes: map[string]DataScope{"dashboard.chat.v2_all": DataScopeDepartment}, Scope: DataScopeDepartment, ScopeRequired: true, DepartmentEmployeeIDs: []int{31}, AllowedEmployeeIDs: []int{31}}
	tests := []struct {
		name string
		p    *dashboardprincipal.DashboardPrincipal
		a    *DashboardAccessContext
		path string
	}{
		{name: "missing principal", a: &access},
		{name: "missing access", p: &principal},
		{name: "tenant mismatch", p: &principal, a: changedArchiveMediaAccess(access, func(value *DashboardAccessContext) { value.TenantID++ })},
		{name: "corp mismatch", p: &principal, a: changedArchiveMediaAccess(access, func(value *DashboardAccessContext) { value.CorpID++ })},
		{name: "user mismatch", p: &principal, a: changedArchiveMediaAccess(access, func(value *DashboardAccessContext) { value.UserID++ })},
		{name: "unscoped resource", p: &principal, a: changedArchiveMediaAccess(access, func(value *DashboardAccessContext) { value.ScopeRequired = false })},
		{name: "empty employee scope", p: &principal, a: changedArchiveMediaAccess(access, func(value *DashboardAccessContext) { value.AllowedEmployeeIDs = nil })},
		{name: "invalid id", p: &principal, a: &access, path: "/dashboard/archive/media/not-a-uuid/content"},
		{name: "path traversal", p: &principal, a: &access, path: "/dashboard/archive/media/../content"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := test.path
			if path == "" {
				path = "/dashboard/archive/media/" + archiveMediaTestID + "/content"
			}
			request := httptest.NewRequest(http.MethodGet, path, nil)
			ctx := request.Context()
			if test.p != nil {
				ctx = dashboardprincipal.WithPrincipal(ctx, *test.p)
			}
			if test.a != nil {
				ctx = WithDashboardAccessContext(ctx, *test.a)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request.WithContext(ctx))
			if response.Code != http.StatusNotFound {
				t.Fatalf("status=%d want=404", response.Code)
			}
		})
	}
	if store.calls != 0 {
		t.Fatalf("unsafe request reached store %d times", store.calls)
	}
}

func TestArchiveMediaContentHidesUnavailableAndInvalidRanges(t *testing.T) {
	root := t.TempDir()
	path := writeArchiveMediaTestFile(t, root, []byte("payload"))
	for _, test := range []struct {
		name, status, storagePath, rangeHeader string
		found, directory                       bool
		want                                   int
	}{
		{name: "unknown", want: 404},
		{name: "pending", found: true, status: "pending", storagePath: path, want: 404},
		{name: "fetching", found: true, status: "fetching", storagePath: path, want: 404},
		{name: "failed", found: true, status: "failed", storagePath: path, want: 404},
		{name: "missing", found: true, status: "missing", storagePath: path, want: 404},
		{name: "corrupt", found: true, status: "corrupt", storagePath: path, want: 404},
		{name: "tampered path", found: true, status: "ready", storagePath: filepath.Join(root, "elsewhere"), want: 404},
		{name: "directory", found: true, status: "ready", storagePath: path, directory: true, want: 404},
		{name: "multiple ranges", found: true, status: "ready", storagePath: path, rangeHeader: "bytes=0-1,3-4", want: 416},
		{name: "unsatisfied", found: true, status: "ready", storagePath: path, rangeHeader: "bytes=99-", want: 416},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.directory {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
				defer func() { _ = os.Remove(path); _ = os.WriteFile(path, []byte("payload"), 0o600) }()
			}
			store := &fakeArchiveMediaContentStore{found: test.found, object: ArchiveMediaContentObject{ID: archiveMediaTestID, MediaType: "file", Size: 7, Status: test.status, StoragePath: test.storagePath, SHA256: archiveMediaTestSHA256([]byte("payload"))}}
			response := httptest.NewRecorder()
			NewArchiveMediaContentHandler(store, root).ServeHTTP(response, archiveMediaRequest(http.MethodGet, test.rangeHeader))
			if response.Code != test.want {
				t.Fatalf("status=%d want=%d", response.Code, test.want)
			}
		})
	}
}

func TestArchiveMediaContentSanitizesMetadataAndRejectsChangedFiles(t *testing.T) {
	root := t.TempDir()
	path := writeArchiveMediaTestFile(t, root, []byte("payload"))
	originalInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	store := &fakeArchiveMediaContentStore{found: true, object: ArchiveMediaContentObject{
		ID: archiveMediaTestID, MediaType: "image", Name: "..\\unsafe\"\r\nX-Evil: yes.svg",
		MIMEType: "image/svg+xml", Size: 7, Status: "ready", StoragePath: path, SHA256: archiveMediaTestSHA256([]byte("payload")),
	}}
	response := httptest.NewRecorder()
	NewArchiveMediaContentHandler(store, root).ServeHTTP(response, archiveMediaRequest(http.MethodGet, ""))
	disposition := response.Header().Get("Content-Disposition")
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "application/octet-stream" || !strings.HasPrefix(disposition, "attachment;") || strings.ContainsAny(disposition, "\r\n") || strings.Contains(disposition, "unsafe\"") {
		t.Fatalf("status/type/disposition=%d/%q/%q", response.Code, response.Header().Get("Content-Type"), disposition)
	}
	if err := os.WriteFile(path, []byte("PAYLOAD"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, originalInfo.ModTime(), originalInfo.ModTime()); err != nil {
		t.Fatal(err)
	}
	response = httptest.NewRecorder()
	NewArchiveMediaContentHandler(store, root).ServeHTTP(response, archiveMediaRequest(http.MethodGet, ""))
	if response.Code != http.StatusNotFound {
		t.Fatalf("same-size tamper status=%d want=404", response.Code)
	}
	if err := os.WriteFile(path, []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	store.object.Size = 8
	response = httptest.NewRecorder()
	NewArchiveMediaContentHandler(store, root).ServeHTTP(response, archiveMediaRequest(http.MethodGet, ""))
	if response.Code != http.StatusNotFound {
		t.Fatalf("changed file status=%d want=404", response.Code)
	}
}

func TestArchiveMediaContentRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "archive-media", archiveMediaTestID)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "outside")
	if err := os.WriteFile(target, []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	store := &fakeArchiveMediaContentStore{found: true, object: ArchiveMediaContentObject{ID: archiveMediaTestID, MediaType: "file", Size: 7, Status: "ready", StoragePath: path, SHA256: archiveMediaTestSHA256([]byte("payload"))}}
	response := httptest.NewRecorder()
	NewArchiveMediaContentHandler(store, root).ServeHTTP(response, archiveMediaRequest(http.MethodGet, ""))
	if response.Code != http.StatusNotFound {
		t.Fatalf("symlink status=%d want=404", response.Code)
	}
}

func TestArchiveMediaContentRejectsSymlinkedArchiveDirectory(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	path := filepath.Join(outside, archiveMediaTestID)
	if err := os.WriteFile(path, []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	archiveRoot := filepath.Join(root, "archive-media")
	if err := os.Symlink(outside, archiveRoot); err != nil {
		t.Skipf("directory symlink unavailable: %v", err)
	}
	store := &fakeArchiveMediaContentStore{found: true, object: ArchiveMediaContentObject{ID: archiveMediaTestID, MediaType: "file", Size: 7, Status: "ready", StoragePath: filepath.Join(archiveRoot, archiveMediaTestID), SHA256: archiveMediaTestSHA256([]byte("payload"))}}
	response := httptest.NewRecorder()
	NewArchiveMediaContentHandler(store, root).ServeHTTP(response, archiveMediaRequest(http.MethodGet, ""))
	if response.Code != http.StatusNotFound {
		t.Fatalf("symlinked archive directory status=%d want=404", response.Code)
	}
}

func TestArchiveMediaPermissionResourceMatchesGETAndHEAD(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		resources := []DashboardPermissionResource{{
			PermissionCode: "dashboard.chat.v2_all", Method: method,
			PathPattern: "/dashboard/archive/media/{id}/content", ScopeRequired: true,
		}}
		matches := matchingDashboardResources(resources, method, "/dashboard/archive/media/"+archiveMediaTestID+"/content")
		if len(matches) != 1 || !matches[0].ScopeRequired {
			t.Fatalf("%s matches=%+v", method, matches)
		}
	}
}

func changedArchiveMediaAccess(value DashboardAccessContext, change func(*DashboardAccessContext)) *DashboardAccessContext {
	change(&value)
	return &value
}

func writeArchiveMediaTestFile(t *testing.T, root string, payload []byte) string {
	t.Helper()
	path := filepath.Join(root, "archive-media", archiveMediaTestID)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func archiveMediaTestSHA256(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func archiveMediaRequest(method, rangeHeader string) *http.Request {
	req := httptest.NewRequest(method, "/dashboard/archive/media/"+archiveMediaTestID+"/content", nil)
	if rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}
	principal := dashboardprincipal.DashboardPrincipal{UserID: 5, TenantID: 11, CorpID: 27, CorpStatus: dashboardprincipal.CorpBindingStatusActive, AuthVersion: 1}
	ctx := dashboardprincipal.WithPrincipal(req.Context(), principal)
	ctx = WithDashboardAccessContext(ctx, DashboardAccessContext{UserID: 5, TenantID: 11, CorpID: 27, WorkEmployeeID: 31, PermissionCode: "dashboard.chat.v2_all", PermissionCodes: []string{"dashboard.chat.v2_all"}, PermissionScopes: map[string]DataScope{"dashboard.chat.v2_all": DataScopeDepartment}, Scope: DataScopeDepartment, ScopeRequired: true, DepartmentEmployeeIDs: []int{31, 32}, AllowedEmployeeIDs: []int{31, 32}})
	return req.WithContext(ctx)
}
