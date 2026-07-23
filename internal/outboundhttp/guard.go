package outboundhttp

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"sort"
	"strings"
	"time"
)

const maxRedirects = 10

var (
	restrictedPrefixes = mustPrefixes(
		"0.0.0.0/8",
		"10.0.0.0/8",
		"100.64.0.0/10",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"172.16.0.0/12",
		"192.0.0.0/24",
		"192.0.2.0/24",
		"192.168.0.0/16",
		"198.18.0.0/15",
		"198.51.100.0/24",
		"203.0.113.0/24",
		"224.0.0.0/4",
		"240.0.0.0/4",
		"100::/64",
		"2001:db8::/32",
		"fc00::/7",
		"fe80::/10",
		"ff00::/8",
	)
	metadataAddresses = mustAddresses(
		"100.100.100.200",
		"168.63.129.16",
		"169.254.169.253",
		"169.254.169.254",
		"169.254.170.2",
		"fd00:ec2::254",
	)
)

type Resolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}

type DialContextFunc func(context.Context, string, string) (net.Conn, error)

type Config struct {
	RequireHTTPS bool
	AllowedCIDRs []string
	Resolver     Resolver
	DialContext  DialContextFunc
	TLSRootCAs   *x509.CertPool
}

type Status struct {
	RequireHTTPS             bool     `json:"requireHttps"`
	PrivateNetworksBlocked   bool     `json:"privateNetworksBlocked"`
	MetadataAddressesBlocked bool     `json:"metadataAddressesBlocked"`
	DNSPinningEnabled        bool     `json:"dnsPinningEnabled"`
	SameOriginRedirectsOnly  bool     `json:"sameOriginRedirectsOnly"`
	EnvironmentProxyDisabled bool     `json:"environmentProxyDisabled"`
	CustomRootCAsConfigured  bool     `json:"customRootCasConfigured"`
	AllowedCIDRs             []string `json:"allowedCidrs"`
	AllowedCIDRCount         int      `json:"allowedCidrCount"`
}

type Guard struct {
	requireHTTPS  bool
	allowedCIDRs  []string
	allowed       []netip.Prefix
	resolver      Resolver
	dialContext   DialContextFunc
	transport     *http.Transport
	customRootCAs bool
}

func NewGuard(config Config) (*Guard, error) {
	normalized, prefixes, err := normalizeCIDRs(config.AllowedCIDRs)
	if err != nil {
		return nil, err
	}
	resolver := config.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	dialContext := config.DialContext
	if dialContext == nil {
		dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
		dialContext = dialer.DialContext
	}
	guard := &Guard{
		requireHTTPS: config.RequireHTTPS,
		allowedCIDRs: normalized,
		allowed:      prefixes,
		resolver:     resolver,
		dialContext:  dialContext,
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = guard.dial
	if config.TLSRootCAs != nil {
		tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: config.TLSRootCAs}
		if transport.TLSClientConfig != nil {
			tlsConfig = transport.TLSClientConfig.Clone()
			tlsConfig.RootCAs = config.TLSRootCAs
			if tlsConfig.MinVersion < tls.VersionTLS12 {
				tlsConfig.MinVersion = tls.VersionTLS12
			}
		}
		transport.TLSClientConfig = tlsConfig
		guard.customRootCAs = true
	}
	guard.transport = transport
	return guard, nil
}

func MustDefaultGuard() *Guard {
	guard, err := NewGuard(Config{RequireHTTPS: true})
	if err != nil {
		panic(err)
	}
	return guard
}

func NormalizeCIDRs(values []string) ([]string, error) {
	normalized, _, err := normalizeCIDRs(values)
	return normalized, err
}

func (g *Guard) ValidateURL(raw string) error {
	parsed, err := parseURL(raw)
	if err != nil {
		return err
	}
	if g == nil {
		g = MustDefaultGuard()
	}
	if g.requireHTTPS && parsed.Scheme != "https" {
		return errors.New("URL must use HTTPS")
	}
	host := normalizedHostname(parsed)
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return errors.New("URL host must not be localhost")
	}
	if address, err := netip.ParseAddr(strings.Trim(host, "[]")); err == nil {
		if err := g.validateAddress(address); err != nil {
			return err
		}
	}
	return nil
}

func (g *Guard) NewClient() *http.Client {
	if g == nil {
		g = MustDefaultGuard()
	}
	return &http.Client{
		Transport:     g.transport,
		CheckRedirect: g.checkRedirect,
	}
}

func (g *Guard) Status() Status {
	if g == nil {
		g = MustDefaultGuard()
	}
	return Status{
		RequireHTTPS:             g.requireHTTPS,
		PrivateNetworksBlocked:   true,
		MetadataAddressesBlocked: true,
		DNSPinningEnabled:        true,
		SameOriginRedirectsOnly:  true,
		EnvironmentProxyDisabled: true,
		CustomRootCAsConfigured:  g.customRootCAs,
		AllowedCIDRs:             append([]string{}, g.allowedCIDRs...),
		AllowedCIDRCount:         len(g.allowedCIDRs),
	}
}

func (g *Guard) dial(ctx context.Context, network string, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("split outbound address: %w", err)
	}
	addresses, err := g.resolve(ctx, host)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for _, candidate := range addresses {
		if network == "tcp4" && !candidate.Is4() {
			continue
		}
		if network == "tcp6" && candidate.Is4() {
			continue
		}
		connection, err := g.dialContext(ctx, network, net.JoinHostPort(candidate.String(), port))
		if err == nil {
			return connection, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	if lastErr != nil {
		return nil, fmt.Errorf("dial outbound host %q: %w", host, lastErr)
	}
	return nil, fmt.Errorf("outbound host %q has no address for network %s", host, network)
}

func (g *Guard) resolve(ctx context.Context, host string) ([]netip.Addr, error) {
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if address, err := netip.ParseAddr(host); err == nil {
		address = address.Unmap()
		if err := g.validateAddress(address); err != nil {
			return nil, err
		}
		return []netip.Addr{address}, nil
	}
	lookupHost := strings.TrimSuffix(strings.ToLower(host), ".")
	if lookupHost == "" || lookupHost == "localhost" || strings.HasSuffix(lookupHost, ".localhost") {
		return nil, errors.New("outbound host must not be localhost")
	}
	addresses, err := g.resolver.LookupNetIP(ctx, "ip", lookupHost)
	if err != nil {
		return nil, fmt.Errorf("resolve outbound host %q: %w", lookupHost, err)
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("resolve outbound host %q: no addresses", lookupHost)
	}
	result := make([]netip.Addr, 0, len(addresses))
	seen := map[netip.Addr]struct{}{}
	for _, address := range addresses {
		address = address.Unmap()
		if err := g.validateAddress(address); err != nil {
			return nil, fmt.Errorf("outbound host %q: %w", lookupHost, err)
		}
		if _, found := seen[address]; found {
			continue
		}
		seen[address] = struct{}{}
		result = append(result, address)
	}
	return result, nil
}

func (g *Guard) validateAddress(address netip.Addr) error {
	address = address.Unmap()
	if !address.IsValid() || address.IsUnspecified() || address.IsMulticast() {
		return fmt.Errorf("network address %q is not a valid outbound destination", address)
	}
	for _, metadata := range metadataAddresses {
		if address == metadata {
			return fmt.Errorf("network address %q is a blocked metadata endpoint", address)
		}
	}
	for _, prefix := range g.allowed {
		if prefix.Contains(address) {
			return nil
		}
	}
	if !address.IsGlobalUnicast() {
		return fmt.Errorf("network address %q is not globally routable", address)
	}
	for _, prefix := range restrictedPrefixes {
		if prefix.Contains(address) {
			return fmt.Errorf("network address %q belongs to a blocked network", address)
		}
	}
	return nil
}

func (g *Guard) checkRedirect(request *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return errors.New("stopped after 10 redirects")
	}
	if err := g.ValidateURL(request.URL.String()); err != nil {
		return fmt.Errorf("redirect target rejected: %w", err)
	}
	if len(via) == 0 {
		return nil
	}
	previous := via[len(via)-1].URL
	if !sameOrigin(previous, request.URL) {
		return errors.New("redirect target must keep the original scheme, host, and port")
	}
	return nil
}

func parseURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed == nil || parsed.IsAbs() == false || parsed.Host == "" || parsed.Opaque != "" {
		return nil, errors.New("URL must be an absolute HTTP or HTTPS URL")
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("URL only supports HTTP or HTTPS")
	}
	if parsed.User != nil {
		return nil, errors.New("URL credentials are not allowed")
	}
	if parsed.Fragment != "" {
		return nil, errors.New("URL fragments are not allowed")
	}
	host := normalizedHostname(parsed)
	if host == "" || strings.Contains(host, "%") {
		return nil, errors.New("URL host is invalid")
	}
	return parsed, nil
}

func normalizedHostname(parsed *url.URL) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(parsed.Hostname()), "."))
}

func sameOrigin(left *url.URL, right *url.URL) bool {
	if left == nil || right == nil {
		return false
	}
	return strings.EqualFold(left.Scheme, right.Scheme) &&
		strings.EqualFold(left.Hostname(), right.Hostname()) &&
		effectivePort(left) == effectivePort(right)
}

func effectivePort(value *url.URL) string {
	if port := value.Port(); port != "" {
		return port
	}
	if strings.EqualFold(value.Scheme, "https") {
		return "443"
	}
	return "80"
}

func normalizeCIDRs(values []string) ([]string, []netip.Prefix, error) {
	seen := map[string]netip.Prefix{}
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
				return nil, nil, fmt.Errorf("invalid allowed outbound CIDR %q", raw)
			}
			if prefix.Bits() == 0 {
				return nil, nil, fmt.Errorf("allowed outbound CIDR %q must not cover the entire address space", raw)
			}
			minimumBits := 8
			if prefix.Addr().Is6() {
				minimumBits = 7
			}
			if prefix.Bits() < minimumBits {
				return nil, nil, fmt.Errorf("allowed outbound CIDR %q is too broad", raw)
			}
			seen[prefix.String()] = prefix
		}
	}
	normalized := make([]string, 0, len(seen))
	for value := range seen {
		normalized = append(normalized, value)
	}
	sort.Strings(normalized)
	prefixes := make([]netip.Prefix, 0, len(normalized))
	for _, value := range normalized {
		prefixes = append(prefixes, seen[value])
	}
	return normalized, prefixes, nil
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

func mustPrefixes(values ...string) []netip.Prefix {
	result := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		result = append(result, netip.MustParsePrefix(value))
	}
	return result
}

func mustAddresses(values ...string) []netip.Addr {
	result := make([]netip.Addr, 0, len(values))
	for _, value := range values {
		result = append(result, netip.MustParseAddr(value).Unmap())
	}
	return result
}
