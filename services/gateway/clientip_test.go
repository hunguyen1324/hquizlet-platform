package main

import (
	"net/http/httptest"
	"testing"
)

func TestClientIPIgnoresHeadersFromUntrustedPeers(t *testing.T) {
	resolver := newIPResolver(defaultTrustedProxies)
	req := httptest.NewRequest("POST", "/v1/auth/login", nil)
	req.RemoteAddr = "203.0.113.9:5555"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	req.Header.Set("X-Real-IP", "5.6.7.8")
	req.Header.Set("CF-Connecting-IP", "9.9.9.9")

	if got := resolver.ClientIP(req); got != "203.0.113.9" {
		t.Fatalf("direct client spoofing headers resolved to %q, want 203.0.113.9", got)
	}
}

func TestClientIPUsesRightmostUntrustedHopBehindProxy(t *testing.T) {
	resolver := newIPResolver(defaultTrustedProxies)
	req := httptest.NewRequest("POST", "/v1/auth/login", nil)
	req.RemoteAddr = "172.18.0.3:444"
	// Left-most value was supplied by the client and must be ignored.
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 198.51.100.7")

	if got := resolver.ClientIP(req); got != "198.51.100.7" {
		t.Fatalf("proxy chain resolved to %q, want 198.51.100.7", got)
	}
}

func TestClientIPFallsBackToProxyAddressWhenChainIsTrusted(t *testing.T) {
	resolver := newIPResolver(defaultTrustedProxies)
	req := httptest.NewRequest("POST", "/v1/auth/login", nil)
	req.RemoteAddr = "172.18.0.3:444"
	req.Header.Set("X-Forwarded-For", "10.1.1.1:")

	if got := resolver.ClientIP(req); got != "10.1.1.1" {
		t.Fatalf("all-trusted chain resolved to %q, want left-most 10.1.1.1", got)
	}
}

func TestClientIPUsesRealIPAndCFHeadersAsFallback(t *testing.T) {
	resolver := newIPResolver(defaultTrustedProxies)

	req := httptest.NewRequest("GET", "/v1/classes", nil)
	req.RemoteAddr = "[::1]:5000"
	req.Header.Set("X-Real-IP", "198.51.100.20")
	if got := resolver.ClientIP(req); got != "198.51.100.20" {
		t.Fatalf("X-Real-IP fallback resolved to %q", got)
	}

	req = httptest.NewRequest("GET", "/v1/classes", nil)
	req.RemoteAddr = "[::1]:5000"
	req.Header.Set("CF-Connecting-IP", "2001:db8::7")
	if got := resolver.ClientIP(req); got != "2001:db8::7" {
		t.Fatalf("CF-Connecting-IP fallback resolved to %q", got)
	}
}

func TestClientIPRespectsNarrowTrustList(t *testing.T) {
	resolver := newIPResolver("172.18.0.0/16")

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "172.19.0.9:5000"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	if got := resolver.ClientIP(req); got != "172.19.0.9" {
		t.Fatalf("peer outside TRUSTED_PROXIES resolved to %q, want 172.19.0.9", got)
	}

	req = httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "172.18.0.3:5000"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	if got := resolver.ClientIP(req); got != "1.2.3.4" {
		t.Fatalf("trusted peer resolved to %q, want 1.2.3.4", got)
	}
}

func TestClientIPHandlesInvalidRemoteAddr(t *testing.T) {
	resolver := newIPResolver(defaultTrustedProxies)
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "not-an-address"
	if got := resolver.ClientIP(req); got != "" {
		t.Fatalf("invalid peer resolved to %q, want empty", got)
	}
}
