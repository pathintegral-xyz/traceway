package migrations

import (
	"fmt"
	"strings"
	"testing"
)

func TestNormalizeLegacyD1BooleanValues(t *testing.T) {
	target := openMemorySQLite(t)
	tables := map[string][]string{
		"notification_channels":  {"enabled"},
		"notification_rules":     {"enabled"},
		"projects":               {"drop_healthy_healthchecks"},
		"widget_groups":          {"is_default"},
		"widget_group_widgets":   {"is_starred"},
		"refresh_tokens":         {"revoked", "used"},
		"personal_access_tokens": {"revoked"},
		"user_contact_methods":   {"enabled", "verified"},
		"synthetic_checks":       {"enabled"},
		"status_pages":           {"is_public"},
	}
	for table, columns := range tables {
		definitions := make([]string, len(columns))
		for i, column := range columns {
			definitions[i] = column + " INTEGER NOT NULL"
		}
		if _, err := target.Exec(fmt.Sprintf("CREATE TABLE %s (%s)", table, strings.Join(definitions, ", "))); err != nil {
			t.Fatal(err)
		}
		for _, value := range []string{"true", "false"} {
			placeholders := strings.TrimSuffix(strings.Repeat("?,", len(columns)), ",")
			args := make([]any, len(columns))
			for i := range args {
				args[i] = value
			}
			if _, err := target.Exec(fmt.Sprintf("INSERT INTO %s VALUES (%s)", table, placeholders), args...); err != nil {
				t.Fatal(err)
			}
		}
	}
	contents, err := migrationsSqliteFS.ReadFile("sqlite/0077_normalize_d1_boolean_values.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range splitStatements(string(contents)) {
		if strings.TrimSpace(statement) == "" {
			continue
		}
		if _, err := target.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	for table, columns := range tables {
		for _, column := range columns {
			rows, err := target.Query(fmt.Sprintf("SELECT %s, typeof(%s) FROM %s ORDER BY rowid", column, column, table))
			if err != nil {
				t.Fatal(err)
			}
			for i, want := range []int{1, 0} {
				if !rows.Next() {
					t.Fatalf("%s.%s: missing row %d", table, column, i)
				}
				var got int
				var kind string
				if err := rows.Scan(&got, &kind); err != nil {
					t.Fatal(err)
				}
				if got != want || kind != "integer" {
					t.Errorf("%s.%s row %d = %d (%s), want %d (integer)", table, column, i, got, kind, want)
				}
			}
			if err := rows.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}
}
