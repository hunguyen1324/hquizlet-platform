package main

import (
	"net"
	"net/http"
	"strconv"
	"strings"
)

// defaultTrustedProxies lists the networks the gateway accepts forwarding
// headers from. It covers loopback and the private ranges Docker/Tailscale
// style deployments use; set TRUSTED_PROXIES to tighten it (e.g. a single
// nginx network or the Cloudflare edge ranges).
const defaultTrustedProxies = "127.0.0.0/8,::1/128,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,169.254.0.0/16,fc00::/7,fe80::/10"

// ipResolver derives the real client address of a request.
//
// Forwarding headers are only honoured when the request arrives from a trusted
// proxy; otherwise a client could pick its own rate-limit key. When several
// proxies are chained, the right-most address that is not trusted wins, which
// is the closest hop the trusted chain could actually observe.
type ipResolver struct {
	trusted []*net.IPNet
}

func newIPResolver(spec string) *ipResolver {
	r := &ipResolver{}
	for _, raw := range strings.Split(spec, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if !strings.Contains(raw, "/") {
			ip := net.ParseIP(raw)
			if ip == nil {
				continue
			}
			bits := 32
			if ip.To4() == nil {
				bits = 128
			}
			raw = raw + "/" + strconv.Itoa(bits)
		}
		if _, network, err := net.ParseCIDR(raw); err == nil {
			r.trusted = append(r.trusted, network)
		}
	}
	return r
}

func (r *ipResolver) trustedIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	for _, network := range r.trusted {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// ClientIP returns the client address without a port, or "" when it cannot be
// determined.
func (r *ipResolver) ClientIP(req *http.Request) string {
	peer := hostOnly(req.RemoteAddr)
	peerIP := net.ParseIP(peer)
	if peerIP == nil {
		return ""
	}
	normalized := peerIP.String()
	if r == nil || len(r.trusted) == 0 || !r.trustedIP(peerIP) {
		// Direct client: forwarding headers are attacker-controlled, ignore them.
		return normalized
	}

	chain := forwardedChain(req.Header.Get("X-Forwarded-For"))
	if len(chain) == 0 {
		if ip := net.ParseIP(hostOnly(req.Header.Get("X-Real-IP"))); ip != nil {
			chain = []string{ip.String()}
		} else if ip := net.ParseIP(hostOnly(req.Header.Get("CF-Connecting-IP"))); ip != nil {
			chain = []string{ip.String()}
		}
	}
	if len(chain) == 0 {
		return normalized
	}
	for i := len(chain) - 1; i >= 0; i-- {
		ip := net.ParseIP(chain[i])
		if ip == nil {
			continue
		}
		if !r.trustedIP(ip) {
			return ip.String()
		}
		if i == 0 {
			return ip.String()
		}
	}
	return normalized
}

func forwardedChain(header string) []string {
	if strings.TrimSpace(header) == "" {
		return nil
	}
	parts := strings.Split(header, ",")
	chain := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		chain = append(chain, hostOnly(part))
	}
	return chain
}

// hostOnly strips an optional port and IPv6 brackets.
func hostOnly(address string) string {
	address = strings.TrimSpace(address)
	if address == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(address); err == nil {
		return strings.Trim(host, "[]")
	}
	return strings.Trim(address, "[]")
}

// Resolvers are initialised from the environment once so proxies and tests get
// the same behaviour without threading configuration through every call.
var (
	trustedProxySpec = env("TRUSTED_PROXIES", defaultTrustedProxies)
	clientIPResolver = newIPResolver(trustedProxySpec)
)

func clientIPFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	return clientIPResolver.ClientIP(r)
}
