package main

import (
	"os"
	"strconv"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/service"
)

// contentPolicyFromEnv reads anti-bulk-collection thresholds.
//
//	CONTENT_MAX_PAGE_SIZE        (default 20)   non-owner cards per request
//	CONTENT_REQUESTS_PER_MIN     (default 60)   content requests / account / minute
//	CONTENT_CARDS_PER_WINDOW     (default 300)  delivered cards / account / window
//	CONTENT_CARD_WINDOW_SECONDS  (default 600)
//	CONTENT_BULK_ENFORCEMENT_ENABLED (default true) false = log-only
//
// The enforcement flag only affects rate budgets; permission checks and the
// page-size cap are never disabled by it.
func contentPolicyFromEnv() service.ContentPolicy {
	p := service.DefaultContentPolicy()
	p.MaxPageSize = envInt("CONTENT_MAX_PAGE_SIZE", p.MaxPageSize)
	p.RequestsPerMin = envInt("CONTENT_REQUESTS_PER_MIN", p.RequestsPerMin)
	p.CardsPerWindow = envInt("CONTENT_CARDS_PER_WINDOW", p.CardsPerWindow)
	p.CardWindow = time.Duration(envInt("CONTENT_CARD_WINDOW_SECONDS", int(p.CardWindow/time.Second))) * time.Second
	if v := os.Getenv("CONTENT_BULK_ENFORCEMENT_ENABLED"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			p.EnforceLimits = b
		}
	}
	return p
}

func envInt(key string, def int) int {
	if n, err := strconv.Atoi(os.Getenv(key)); err == nil && n > 0 {
		return n
	}
	return def
}
