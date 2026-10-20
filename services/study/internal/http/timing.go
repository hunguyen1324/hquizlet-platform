package http

import (
	"fmt"
	"net/http"
	"time"
)

// writeTiming reports how long the server spent on one quiz request. It lands in
// the Server-Timing header, so DevTools → Network shows the cost per endpoint
// without deploying tracing infrastructure.
func writeTiming(w http.ResponseWriter, name string, start time.Time) {
	w.Header().Set("Server-Timing", fmt.Sprintf("%s;dur=%.1f", name, float64(time.Since(start).Microseconds())/1000))
}
