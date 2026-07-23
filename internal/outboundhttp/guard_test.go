package outboundhttp

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

func TestGuardValidatesURLSecurityContract(t *testing.T) {
	guard, err := NewGuard(Config{RequireHTTPS: true})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"https public hostname", "https://hooks.example.com/events", ""},
		{"http rejected", "http://hooks.example.com/events", "HTTPS"},
		{"credentials rejected", "https://user:pass@hooks.example.com/events", "credentials"},
		{"fragment rejected", "https://hooks.example.com/events#token", "fragments"},
		{"localhost rejected", "https://service.localhost/events", "localhost"},
		{"private literal rejected", "https://10.0.0.8/events", "blocked network"},
		{"metadata rejected", "https://169.254.169.254/latest/meta-data", "metadata endpoint"},
		{"IPv4 mapped private rejected", "https://[::ffff:127.0.0.1]/events", "globally routable"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := guard.ValidateURL(test.url)
			if test.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ValidateURL() error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestGuardAllowsExplicitCIDRButNeverMetadata(t *testing.T) {
	guard, err := NewGuard(Config{AllowedCIDRs: []string{"127.0.0.0/8", "169.254.0.0/16"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := guard.ValidateURL("http://127.0.0.1/hook"); err != nil {
		t.Fatal(err)
	}
	if err := guard.ValidateURL("http://169.254.169.254/latest/meta-data"); err == nil || !strings.Contains(err.Error(), "metadata endpoint") {
		t.Fatalf("metadata validation error = %v", err)
	}
	status := guard.Status()
	if status.RequireHTTPS || status.AllowedCIDRCount != 2 || !status.DNSPinningEnabled || !status.EnvironmentProxyDisabled {
		t.Fatalf("Status() = %+v", status)
	}
}

func TestGuardPinsResolvedAddressForDial(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host == "hooks.example.test" {
			t.Fatalf("expected request host to preserve explicit port, got %q", r.Host)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	serverURL, _ := url.Parse(server.URL)
	resolver := staticResolver{"hooks.example.test": {netip.MustParseAddr("127.0.0.1")}}
	var dialed string
	dialer := &net.Dialer{}
	guard, err := NewGuard(Config{
		AllowedCIDRs: []string{"127.0.0.0/8"},
		Resolver:     resolver,
		DialContext: func(ctx context.Context, network string, address string) (net.Conn, error) {
			dialed = address
			return dialer.DialContext(ctx, network, address)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	requestURL := "http://hooks.example.test:" + serverURL.Port() + "/hook"
	response, err := guard.NewClient().Get(requestURL)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if !strings.HasPrefix(dialed, "127.0.0.1:") || strings.Contains(dialed, "hooks.example.test") {
		t.Fatalf("dialed address = %q", dialed)
	}
}

func TestGuardRejectsMixedDNSAnswersBeforeDial(t *testing.T) {
	var dialCalls atomic.Int32
	guard, err := NewGuard(Config{
		Resolver: staticResolver{"hooks.example.test": {
			netip.MustParseAddr("8.8.8.8"),
			netip.MustParseAddr("10.0.0.8"),
		}},
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			dialCalls.Add(1)
			return nil, errors.New("unexpected dial")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = guard.NewClient().Get("http://hooks.example.test/hook")
	if err == nil || !strings.Contains(err.Error(), "blocked network") {
		t.Fatalf("request error = %v", err)
	}
	if dialCalls.Load() != 0 {
		t.Fatalf("dial calls = %d", dialCalls.Load())
	}
}

func TestGuardRejectsRedirectToDifferentOrigin(t *testing.T) {
	var targetCalls atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		targetCalls.Add(1)
	}))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/internal", http.StatusFound)
	}))
	defer source.Close()
	guard, err := NewGuard(Config{AllowedCIDRs: []string{"127.0.0.0/8"}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = guard.NewClient().Get(source.URL + "/hook")
	if err == nil || !strings.Contains(err.Error(), "original scheme, host, and port") {
		t.Fatalf("redirect error = %v", err)
	}
	if targetCalls.Load() != 0 {
		t.Fatalf("redirect target calls = %d", targetCalls.Load())
	}
}

func TestNormalizeCIDRs(t *testing.T) {
	got, err := NormalizeCIDRs([]string{"127.0.0.1, 10.0.0.7/8", "127.0.0.1/32", "2001:db8::1"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"10.0.0.0/8", "127.0.0.1/32", "2001:db8::1/128"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeCIDRs() = %v, want %v", got, want)
	}
	if _, err := NormalizeCIDRs([]string{"0.0.0.0/0"}); err == nil {
		t.Fatal("expected entire address space rejection")
	}
	if _, err := NormalizeCIDRs([]string{"128.0.0.0/1"}); err == nil {
		t.Fatal("expected broad CIDR rejection")
	}
}

type staticResolver map[string][]netip.Addr

func (r staticResolver) LookupNetIP(_ context.Context, _ string, host string) ([]netip.Addr, error) {
	addresses, found := r[host]
	if !found {
		return nil, errors.New("host not found")
	}
	return append([]netip.Addr{}, addresses...), nil
}
