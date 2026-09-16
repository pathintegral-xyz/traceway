//go:build cloudflare && !transactional_pg && !telemetry_ch && !telemetry_duckdb

package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/db/d1http"
)

//go:embed sqlite_telemetry/*.sql
var migrationsCloudflareTelemetryFS embed.FS

// Run applies each migration file and its version marker as one D1 batch. A
// failed file therefore cannot leave a partially-applied schema behind.
func Run(_ string) error {
	if err := runD1Migrations(db.DB, db.MainD1, migrationsSqliteFS, "sqlite", "schema_migrations"); err != nil {
		return fmt.Errorf("main D1 migrations: %w", err)
	}
	if err := runD1Migrations(db.TelemetryDB, db.TelemetryD1, migrationsCloudflareTelemetryFS, "sqlite_telemetry", "schema_migrations"); err != nil {
		return fmt.Errorf("telemetry D1 migrations: %w", err)
	}
	return nil
}

type d1BatchExecutor interface {
	Batch(context.Context, []d1http.Statement) ([]d1http.BatchResult, error)
}

func runD1Migrations(target *sql.DB, executor d1BatchExecutor, fsys fs.FS, dir, trackingTable string) error {
	if executor == nil {
		return fmt.Errorf("D1 batch connector is not initialized")
	}
	if _, err := target.Exec(fmt.Sprintf(sqliteTrackingDDL, trackingTable)); err != nil {
		return fmt.Errorf("create %s table: %w", trackingTable, err)
	}
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return fmt.Errorf("read migrations dir %s: %w", dir, err)
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)
	for _, file := range files {
		version := strings.TrimSuffix(file, ".up.sql")
		var count int
		if err := target.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE version = ?", trackingTable), version).Scan(&count); err != nil {
			return fmt.Errorf("check migration version %s: %w", version, err)
		}
		if count > 0 {
			continue
		}
		contents, err := fs.ReadFile(fsys, dir+"/"+file)
		if err != nil {
			return fmt.Errorf("read migration file %s: %w", file, err)
		}
		batch := make([]d1http.Statement, 0)
		for _, statement := range splitStatements(string(contents)) {
			if statement = strings.TrimSpace(statement); statement != "" {
				batch = append(batch, d1http.Statement{SQL: statement})
			}
		}
		batch = append(batch, d1http.Statement{SQL: fmt.Sprintf("INSERT INTO %s (version) VALUES (?)", trackingTable), Params: []any{version}})
		config.Logf("migrations: applying %s/%s", dir, version)
		started := time.Now()
		if _, err := executor.Batch(context.Background(), batch); err != nil {
			return fmt.Errorf("execute migration %s: %w", file, err)
		}
		config.Logf("migrations: applied %s/%s in %s", dir, version, time.Since(started).Round(time.Millisecond))
	}
	return nil
}
