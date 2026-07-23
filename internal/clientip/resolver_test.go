package clientip

import (
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestResolverIgnoresForwardedHeadersByDefault(t *testing.T) {
	resolver := DirectResolver()
	status := resolver.Status()
	if status.TrustedProxyCIDRs == nil || status.TrustedProxyCIDRCount != 0 {
		t.Fatalf("Status() = %+v", status)
	}
	request := httptest.NewRequest("GET", "http://example.test", nil)
	request.RemoteAddr = "198.51.100.8:4321"
	request.Header.Set("X-Forwarded-For", "203.0.113.9")
	request.Header.Set("X-Real-IP", "203.0.113.10")
	if got := resolver.Resolve(request); got != "198.51.100.8" {
		t.Fatalf("Resolve() = %q", got)
	}
}

func TestResolverOnlyTrustsConfiguredProxyPeer(t *testing.T) {
	resolver, err := NewResolver(Config{TrustProxyHeaders: true, TrustedProxyCIDRs: []string{"10.0.0.0/8"}})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("GET", "http://example.test", nil)
	request.RemoteAddr = "198.51.100.8:4321"
	request.Header.Set("X-Forwarded-For", "203.0.113.9")
	if got := resolver.Resolve(request); got != "198.51.100.8" {
		t.Fatalf("Resolve() = %q", got)
	}
}

func TestResolverWalksForwardedChainFromTrustedEdge(t *testing.T) {
	resolver, err := NewResolver(Config{TrustProxyHeaders: true, TrustedProxyCIDRs: []string{"10.0.0.0/8", "192.0.2.0/24"}})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("GET", "http://example.test", nil)
	request.RemoteAddr = "10.0.0.8:4321"
	request.Header.Set("X-Forwarded-For", "198.51.100.44, 203.0.113.25, 192.0.2.9")
	if got := resolver.Resolve(request); got != "203.0.113.25" {
		t.Fatalf("Resolve() = %q", got)
	}
}

func TestResolverFallsBackToRealIPForTrustedProxy(t *testing.T) {
	resolver, err := NewResolver(Config{TrustProxyHeaders: true, TrustedProxyCIDRs: []string{"127.0.0.1"}})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("GET", "http://example.test", nil)
	request.RemoteAddr = "127.0.0.1:4321"
	request.Header.Set("X-Forwarded-For", "invalid")
	request.Header.Set("X-Real-IP", "2001:db8::7")
	if got := resolver.Resolve(request); got != "2001:db8::7" {
		t.Fatalf("Resolve() = %q", got)
	}
}

func TestNewResolverRequiresTrustedCIDRs(t *testing.T) {
	if _, err := NewResolver(Config{TrustProxyHeaders: true}); err == nil {
		t.Fatal("expected missing trusted proxy CIDR error")
	}
	if _, err := NewResolver(Config{TrustedProxyCIDRs: []string{"not-a-cidr"}}); err == nil {
		t.Fatal("expected invalid trusted proxy CIDR error")
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
}
