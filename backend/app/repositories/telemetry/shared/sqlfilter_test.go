package shared

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestRootFilterAcceptsNumericAndLegacyTextBooleans(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE traces (is_root INTEGER NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	for _, value := range []any{int64(0), int64(1), "false", "true"} {
		if _, err := db.Exec("INSERT INTO traces (is_root) VALUES (?)", value); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		filter string
		want   int
	}{
		{"root", 2},
		{"non_root", 2},
	} {
		var got int
		if err := db.QueryRow("SELECT COUNT(*) FROM traces WHERE 1=1" + RootFilterClause("is_root", tc.filter)).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != tc.want {
			t.Errorf("%s: got %d rows, want %d", tc.filter, got, tc.want)
		}
	}
}
