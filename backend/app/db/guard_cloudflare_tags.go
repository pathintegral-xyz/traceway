//go:build cloudflare && (transactional_pg || telemetry_ch || telemetry_duckdb)

package db

// The Cloudflare target currently uses the two SQLite-dialect D1 databases.
// Rejecting another storage tag makes the intended migration surface explicit.
var _ = build_error__cloudflare_requires_sqlite_dialect_d1_databases
