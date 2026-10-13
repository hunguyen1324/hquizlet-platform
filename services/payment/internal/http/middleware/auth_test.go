package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSupportCannotCreditWallet(t *testing.T) {
	for _, tt := range []struct {
		method, path string
		want         int
	}{{"GET", "/v1/admin/payments/orders", 204}, {"POST", "/v1/admin/wallet/credit", 403}} {
		h := Auth(RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })))
		r := httptest.NewRequest(tt.method, tt.path, nil)
		r.Header.Set("X-User-ID", "9")
		r.Header.Set("X-User-Roles", "PlatformSupport")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tt.want {
			t.Fatalf("%s %s got %d want %d", tt.method, tt.path, w.Code, tt.want)
		}
	}
}
