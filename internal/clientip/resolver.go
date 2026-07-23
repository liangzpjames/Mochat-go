package clientip

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"sort"
	"strings"
)

type Config struct {
	TrustProxyHeaders bool
	TrustedProxyCIDRs []string
}

type Status struct {
	TrustProxyHeaders     bool     `json:"trustProxyHeaders"`
	TrustedProxyCIDRs     []string `json:"trustedProxyCidrs"`
	TrustedProxyCIDRCount int      `json:"trustedProxyCidrCount"`
}

type Resolver struct {
	trustProxyHeaders bool
	trustedProxyCIDRs []string
	trustedProxies    []netip.Prefix
}

func NewResolver(config Config) (*Resolver, error) {
	normalized, err := NormalizeCIDRs(config.TrustedProxyCIDRs)
	if err != nil {
		return nil, err
	}
	if config.TrustProxyHeaders && len(normalized) == 0 {
		return nil, errors.New("trusted proxy CIDRs are required when proxy headers are enabled")
	}
	prefixes := make([]netip.Prefix, 0, len(normalized))
	for _, value := range normalized {
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			return nil, fmt.Errorf("parse trusted proxy CIDR %q: %w", value, err)
		}
		prefixes = append(prefixes, prefix)
	}
	return &Resolver{
		trustProxyHeaders: config.TrustProxyHeaders,
		trustedProxyCIDRs: normalized,
		trustedProxies:    prefixes,
	}, nil
}

func DirectResolver() *Resolver {
	resolver, _ := NewResolver(Config{})
	return resolver
}

func NormalizeCIDRs(values []string) ([]string, error) {
	seen := map[string]struct{}{}
	for _, value := range values {
		for _, raw := range strings.FieldsFunc(value, func(r rune) bool {
			return r == ',' || r == '\n' || r == '\r' || r == ';'
		}) {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			prefix, err := parsePrefix(raw)
			if err != nil {
				return nil, fmt.Errorf("invalid trusted proxy CIDR %q", raw)
			}
			seen[prefix.String()] = struct{}{}
		}
	}
	result := make([]string, 0, len(seen))
	for value := range seen {
		result = append(result, value)
	}
	sort.Strings(result)
	return result, nil
}

func (r *Resolver) Resolve(request *http.Request) string {
	if request == nil {
		return ""
	}
	peer, ok := parseAddr(request.RemoteAddr)
	if !ok {
		return ""
	}
	if r == nil || !r.trustProxyHeaders || !r.isTrustedProxy(peer) {
		return peer.String()
	}
	if forwarded := r.resolveForwardedFor(request.Header.Get("X-Forwarded-For")); forwarded.IsValid() {
		return forwarded.String()
	}
	if realIP, ok := parseAddr(request.Header.Get("X-Real-IP")); ok {
		return realIP.String()
	}
	return peer.String()
}

func (r *Resolver) Status() Status {
	if r == nil {
		return Status{}
	}
	return Status{
		TrustProxyHeaders:     r.trustProxyHeaders,
		TrustedProxyCIDRs:     append([]string{}, r.trustedProxyCIDRs...),
		TrustedProxyCIDRCount: len(r.trustedProxyCIDRs),
	}
}

func (r *Resolver) resolveForwardedFor(value string) netip.Addr {
	parts := strings.Split(value, ",")
	chain := make([]netip.Addr, 0, len(parts))
	for _, part := range parts {
		if address, ok := parseAddr(part); ok {
			chain = append(chain, address)
		}
	}
	for index := len(chain) - 1; index >= 0; index-- {
		if !r.isTrustedProxy(chain[index]) {
			return chain[index]
		}
	}
	if len(chain) > 0 {
		return chain[0]
	}
	return netip.Addr{}
}

func (r *Resolver) isTrustedProxy(address netip.Addr) bool {
	address = address.Unmap()
	for _, prefix := range r.trustedProxies {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}

func parseAddr(value string) (netip.Addr, bool) {
	value = strings.Trim(strings.TrimSpace(value), "\"")
	if value == "" {
		return netip.Addr{}, false
	}
	if addressPort, err := netip.ParseAddrPort(value); err == nil {
		return addressPort.Addr().Unmap(), true
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}
	address, err := netip.ParseAddr(strings.Trim(value, "[]"))
	if err != nil {
		return netip.Addr{}, false
	}
	return address.Unmap(), true
}

func parsePrefix(value string) (netip.Prefix, error) {
	if !strings.Contains(value, "/") {
		address, err := netip.ParseAddr(value)
		if err != nil {
			return netip.Prefix{}, err
		}
		address = address.Unmap()
		return netip.PrefixFrom(address, address.BitLen()), nil
	}
	prefix, err := netip.ParsePrefix(value)
	if err != nil {
		return netip.Prefix{}, err
	}
	if prefix.Addr().Is4In6() {
		bits := prefix.Bits() - 96
		if bits < 0 || bits > 32 {
			return netip.Prefix{}, errors.New("invalid IPv4-mapped prefix length")
		}
		prefix = netip.PrefixFrom(prefix.Addr().Unmap(), bits)
	}
	return prefix.Masked(), nil
}
