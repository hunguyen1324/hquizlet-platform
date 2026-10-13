package http

import (
	"net/http/httptest"
	"testing"
)

func TestClientIPProducesValidAuditAddress(t *testing.T) {
	for _, tt := range []struct{ name, forwarded, remote, want string }{
		{"gateway IPv4 with port", "172.19.0.1:50132", "", "172.19.0.1"},
		{"gateway IPv6 with port", "[2001:db8::1]:50132", "", "2001:db8::1"},
		{"proxy chain", " 192.0.2.1, 192.0.2.2", "", "192.0.2.1"},
		{"plain IPv6", "2001:db8::1", "", "2001:db8::1"},
		{"direct peer", "", "192.0.2.2:80", "192.0.2.2"},
		{"invalid value", "unknown", "192.0.2.2:80", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = tt.remote
			r.Header.Set("X-Forwarded-For", tt.forwarded)
			if got := clientIP(r); got != tt.want {
				t.Fatalf("IP = %q, want %q", got, tt.want)
			}
		})
	}
}
