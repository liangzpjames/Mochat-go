package runtimegroup

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewHTTPServerUsesBoundedTimeouts(t *testing.T) {
	server := NewHTTPServer(context.Background(), http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	if server.ReadHeaderTimeout != 5*time.Second || server.ReadTimeout != 15*time.Second || server.WriteTimeout != 30*time.Second || server.IdleTimeout != 60*time.Second {
		t.Fatalf("timeouts = header=%s read=%s write=%s idle=%s", server.ReadHeaderTimeout, server.ReadTimeout, server.WriteTimeout, server.IdleTimeout)
	}
}

func TestHTTPServerRejectsSlowHeaders(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	var handled atomic.Bool
	server := NewHTTPServer(context.Background(), http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		handled.Store(true)
	}))
	server.ReadHeaderTimeout = 40 * time.Millisecond
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(listener) }()

	connection, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if _, err := connection.Write([]byte("GET / HTTP/1.1\r\nHost: slow")); err != nil {
		t.Fatal(err)
	}
	if err := connection.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	line, readErr := bufio.NewReader(connection).ReadString('\n')
	if readErr != nil && !errors.Is(readErr, net.ErrClosed) {
		if networkError, ok := readErr.(net.Error); ok && networkError.Timeout() {
			t.Fatalf("slow header connection was not closed before deadline")
		}
	}
	if handled.Load() {
		t.Fatal("handler ran before a complete request header was received")
	}
	if line != "" && !strings.Contains(line, "408") && !strings.Contains(line, "400") {
		t.Fatalf("unexpected response line %q", line)
	}
	_ = server.Close()
	if err := <-serveDone; err != nil && !errors.Is(err, http.ErrServerClosed) {
		t.Fatal(err)
	}
}

func TestGroupDrainsInflightRequestWithoutCancellingBaseContext(t *testing.T) {
	rootCtx, cancelRoot := context.WithCancel(context.Background())
	defer cancelRoot()
	shutdownCtx, triggerShutdown := context.WithCancel(context.Background())
	defer triggerShutdown()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	release := make(chan struct{})
	requestCancelled := make(chan struct{}, 1)
	server := NewHTTPServer(rootCtx, http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		close(started)
		select {
		case <-request.Context().Done():
			requestCancelled <- struct{}{}
		case <-release:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	drainer := &recordingDrainer{begun: make(chan struct{})}
	group := New(drainer, cancelRoot, time.Second)
	if err := group.Add("main", listener, server); err != nil {
		t.Fatal(err)
	}
	runDone := make(chan error, 1)
	go func() { runDone <- group.Run(shutdownCtx) }()

	responseDone := make(chan int, 1)
	go func() {
		response, requestErr := http.Get("http://" + listener.Addr().String() + "/slow")
		if requestErr != nil {
			responseDone <- 0
			return
		}
		defer response.Body.Close()
		responseDone <- response.StatusCode
	}()
	<-started
	triggerShutdown()
	<-drainer.begun
	close(release)
	if status := <-responseDone; status != http.StatusNoContent {
		t.Fatalf("in-flight response status = %d", status)
	}
	select {
	case <-requestCancelled:
		t.Fatal("signal/root cancellation reached the in-flight request")
	default:
	}
	if err := <-runDone; err != nil {
		t.Fatal(err)
	}
}

func TestGroupReturnsListenerFailureAfterDrainHooks(t *testing.T) {
	drainer := &recordingDrainer{begun: make(chan struct{})}
	var stopped atomic.Bool
	group := New(drainer, func() { stopped.Store(true) }, time.Second)
	server := NewHTTPServer(context.Background(), http.NotFoundHandler())
	listener := &failingListener{err: errors.New("accept failed")}
	if err := group.Add("broken", listener, server); err != nil {
		t.Fatal(err)
	}
	err := group.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "accept failed") {
		t.Fatalf("run error = %v", err)
	}
	if drainer.calls.Load() != 1 || !stopped.Load() {
		t.Fatalf("drain calls=%d stopped=%t", drainer.calls.Load(), stopped.Load())
	}
}

type recordingDrainer struct {
	once  sync.Once
	begun chan struct{}
	calls atomic.Int32
}

func (d *recordingDrainer) BeginDrain() {
	d.calls.Add(1)
	d.once.Do(func() { close(d.begun) })
}

type failingListener struct {
	err error
}

func (l *failingListener) Accept() (net.Conn, error) { return nil, l.err }
func (l *failingListener) Close() error              { return nil }
func (l *failingListener) Addr() net.Addr            { return testAddr("broken") }

type testAddr string

func (a testAddr) Network() string { return "test" }
func (a testAddr) String() string  { return string(a) }
