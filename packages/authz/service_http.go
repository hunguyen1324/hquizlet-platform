package authz

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"time"
)

// SignServiceRequest binds internal GET requests to their exact path and query.
func SignServiceRequest(r *http.Request, key string) {
	stamp := strconv.FormatInt(time.Now().Unix(), 10)
	r.Header.Set("X-Service-Timestamp", stamp)
	r.Header.Set("X-Service-Signature", serviceSignature(r, key, stamp))
}

func VerifyServiceRequest(r *http.Request, key string) bool {
	stamp := r.Header.Get("X-Service-Timestamp")
	ts, err := strconv.ParseInt(stamp, 10, 64)
	if key == "" || err != nil || time.Now().Unix()-ts > 30 || time.Now().Unix()-ts < -30 {
		return false
	}
	return hmac.Equal([]byte(r.Header.Get("X-Service-Signature")), []byte(serviceSignature(r, key, stamp)))
}

func serviceSignature(r *http.Request, key, stamp string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(r.Method + "|" + r.URL.RequestURI() + "|" + stamp))
	return hex.EncodeToString(mac.Sum(nil))
}
