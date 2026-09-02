package store

import (
	"context"
	"database/sql"
	"log"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func Open(dsn string) *sql.DB {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("[payment] db open: %v", err)
	}
	ctx := context.Background()
	for i := 1; i <= 20; i++ {
		if err := db.PingContext(ctx); err == nil {
			return db
		}
		log.Printf("[payment] waiting for postgres (%d/20)…", i)
		time.Sleep(time.Second)
	}
	log.Fatal("[payment] postgres unreachable after 20 attempts")
	return db
}
