// Package netguard provides HTTP clients that refuse to reach internal
// network addresses.
//
// It exists because YtVault fetches user-supplied URLs (poster images,
// image searches). Without a guard, a request can be pointed at a cloud
// metadata endpoint (169.254.169.254) or at a service listening on the
// host's private network, and the response body is written to disk and
// served back.
//
// The guard works in two layers:
//
//  1. Validation resolves the hostname and rejects any address in a
//     private, loopback, link-local, or otherwise reserved range.
//  2. The dialer resolves the hostname itself and connects to the IP it
//     just checked, so a DNS answer that changes between validation and
//     connection (rebinding) cannot slip through.
//
// Every redirect hop is re-validated, because a redirect can move the
// request to a different host.
package netguard

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

// ErrBlocked is returned when a URL resolves to an address the server must
// not reach.
var ErrBlocked = errors.New("url resolves to a private or internal address")

// reservedCIDRs covers ranges that net.IP's Is* helpers do not classify.
var reservedCIDRs = func() []*net.IPNet {
	cidrs := []string{
		"100.64.0.0/10",   // carrier-grade NAT
		"192.0.0.0/24",    // IETF protocol assignments
		"198.18.0.0/15",   // benchmarking
		"192.0.2.0/24",    // TEST-NET-1
		"198.51.100.0/24", // TEST-NET-2
		"203.0.113.0/24",  // TEST-NET-3
		"240.0.0.0/4",     // reserved
		"2001:db8::/32",   // documentation
	}
	out := make([]*net.IPNet, 0, len(cidrs))
	for _, c := range cidrs {
		if _, n, err := net.ParseCIDR(c); err == nil {
			out = append(out, n)
		}
	}
	return out
}()

// IsPublicIP reports whether ip is an address the server may connect to.
func IsPublicIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsInterfaceLocalMulticast() {
		return false
	}
	for _, n := range reservedCIDRs {
		if n.Contains(ip) {
			return false
		}
	}
	return true
}

// ValidateURL parses raw and rejects it unless it is an http(s) URL whose
// host resolves only to public addresses.
func ValidateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("url must be http or https")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("url has no host")
	}
	ips, err := resolve(host)
	if err != nil {
		return err
	}
	for _, ip := range ips {
		if !IsPublicIP(ip) {
			return ErrBlocked
		}
	}
	return nil
}

// resolve returns the addresses host points at. A literal IP is returned
// as-is so that ValidateURL also works for address literals.
func resolve(host string) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}, nil
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, fmt.Errorf("resolve host %q: %w", host, err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("host %q did not resolve", host)
	}
	return ips, nil
}

// Client returns an http.Client that only connects to public addresses.
//
// Redirects are followed (up to maxRedirects) but each hop is re-validated.
// The proxy is deliberately not used: with a proxy in the path the client
// would connect to the proxy rather than to the validated address, which
// would defeat the check.
func Client(timeout time.Duration, maxRedirects int) *http.Client {
	dialer := &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}

	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("split address %q: %w", addr, err)
			}
			ips, err := resolve(host)
			if err != nil {
				return nil, err
			}
			var lastErr error
			for _, ip := range ips {
				if !IsPublicIP(ip) {
					lastErr = ErrBlocked
					continue
				}
				conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
				if err == nil {
					return conn, nil
				}
				lastErr = err
			}
			if lastErr == nil {
				lastErr = ErrBlocked
			}
			return nil, lastErr
		},
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: timeout,
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("too many redirects")
			}
			return ValidateURL(req.URL.String())
		},
	}
}
