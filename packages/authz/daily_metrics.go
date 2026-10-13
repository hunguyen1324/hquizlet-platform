package authz

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// StartDailyMetrics backfills 180 days (90 days plus its comparison period), then refreshes once a minute.
// sourceSQL is a service-owned constant returning date, name, value; no user input enters SQL.
func StartDailyMetrics(db *sql.DB, service, sourceSQL string) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS metrics_daily(service TEXT NOT NULL,day DATE NOT NULL,name TEXT NOT NULL,value BIGINT NOT NULL,updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),PRIMARY KEY(service,day,name))`); err != nil {
		return err
	}
	refresh := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := db.ExecContext(ctx, `INSERT INTO metrics_daily(service,day,name,value) SELECT $1,day,name,value FROM (`+sourceSQL+`) source ON CONFLICT(service,day,name) DO UPDATE SET value=excluded.value,updated_at=now()`, service)
		if err != nil {
			log.Printf("[%s] daily metrics refresh failed: %v", service, err)
		}
	}
	refresh()
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			refresh()
		}
	}()
	return nil
}

func DailyMetricsHandler(db *sql.DB, service string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseInt(r.Header.Get("X-User-ID"), 10, 64)
		p := Principal{UserID: id, LegacyRole: r.Header.Get("X-User-Role"), PlatformRoles: ParsePlatformRoles(r.Header.Get("X-User-Roles"))}
		if !Can(p, ActionDashboardViewAdmin, Resource{Type: "platform"}) {
			http.Error(w, "forbidden", 403)
			return
		}
		days, _ := strconv.Atoi(strings.TrimSuffix(r.URL.Query().Get("range"), "d"))
		if days != 7 && days != 28 && days != 90 {
			days = 28
		}
		rows, err := db.QueryContext(r.Context(), `SELECT to_char(day,'YYYY-MM-DD'),name,value,day>=current_date-($2::integer-1) FROM metrics_daily WHERE service=$1 AND day>=current_date-($2::integer*2-1) ORDER BY day,name`, service, days)
		if err != nil {
			http.Error(w, "metrics unavailable", 503)
			return
		}
		defer rows.Close()
		type Point struct {
			Date  string `json:"date"`
			Name  string `json:"name"`
			Value int64  `json:"value"`
		}
		points := []Point{}
		totals, previous := map[string]int64{}, map[string]int64{}
		for rows.Next() {
			var p Point
			var current bool
			if rows.Scan(&p.Date, &p.Name, &p.Value, &current) != nil {
				http.Error(w, "metrics unavailable", 503)
				return
			}
			if current {
				points = append(points, p)
				totals[p.Name] += p.Value
			} else {
				previous[p.Name] += p.Value
			}
		}
		if rows.Err() != nil {
			http.Error(w, "metrics unavailable", 503)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "private,max-age=60")
		json.NewEncoder(w).Encode(map[string]any{"rangeDays": days, "totals": totals, "previous": previous, "series": points})
	}
}
