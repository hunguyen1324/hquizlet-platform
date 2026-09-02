// Package migrations applies the Quiz service SQL migrations.
package migrations

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

//go:embed *.up.sql
var migrationFiles embed.FS

// Run applies every up migration in filename order. The SQL files are
// idempotent, so this is safe on fresh and existing development databases.
func Run(db *sql.DB) error {
	entries, err := fs.ReadDir(migrationFiles, ".")
	if err != nil {
		return fmt.Errorf("read quiz migrations: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		body, err := migrationFiles.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read quiz migration %s: %w", name, err)
		}
		if _, err := db.Exec(string(body)); err != nil {
			return fmt.Errorf("apply quiz migration %s: %w", name, err)
		}
	}
	return nil
}
