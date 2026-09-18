//go:build cloudflare && !transactional_pg && !telemetry_ch && !telemetry_duckdb

package db

import (
	"database/sql"
	"fmt"

	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db/d1http"
	"github.com/google/uuid"
)

// MainD1 and TelemetryD1 expose D1's explicit batch operation to the small
// Cloudflare-only mutation paths that cannot use database/sql transactions.
var (
	MainD1      *d1http.Connector
	TelemetryD1 *d1http.Connector
)

func initMainDB() error {
	main, connector, err := openD1(config.Config.CloudflareD1MainDatabaseID)
	if err != nil {
		return fmt.Errorf("open main D1: %w", err)
	}
	DB = main
	MainD1 = connector
	Driver = lit.SQLite
	config.Logf("Cloudflare D1 main database opened")
	return nil
}

func initTelemetryDB() error {
	telemetry, connector, err := openD1(config.Config.CloudflareD1TelemetryDatabaseID)
	if err != nil {
		return fmt.Errorf("open telemetry D1: %w", err)
	}
	TelemetryDB = telemetry
	TelemetryD1 = connector
	config.Logf("Cloudflare D1 telemetry database opened")
	return nil
}

func openD1(databaseID string) (*sql.DB, *d1http.Connector, error) {
	if config.Config == nil {
		return nil, nil, fmt.Errorf("configuration is not initialized")
	}
	connector, err := d1http.NewConnector(d1http.Config{
		AccountID:  config.Config.CloudflareAccountID,
		DatabaseID: databaseID,
		APIToken:   config.Config.CloudflareD1APIToken,
	})
	if err != nil {
		return nil, nil, err
	}
	database := sql.OpenDB(connector)
	// D1 serializes work per database. Keep the HTTPS client pool small so a
	// dashboard fan-out does not turn into a burst that D1 rejects as overloaded.
	database.SetMaxOpenConns(4)
	database.SetMaxIdleConns(4)
	if err := database.Ping(); err != nil {
		database.Close()
		return nil, nil, err
	}
	return database, connector, nil
}

// Like the SQLite deployment, this only preserves the local cache contract.
// Cross-instance cache invalidation remains a separate Cloudflare migration.
func NotifyProjectCacheChanged(lit.Executor, uuid.UUID) error {
	return nil
}
