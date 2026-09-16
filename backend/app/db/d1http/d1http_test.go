package d1http

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestDB(t *testing.T, handler http.HandlerFunc) *sql.DB {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	db, err := Open(Config{AccountID: "account", DatabaseID: "database", APIToken: "token", Endpoint: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func response(w http.ResponseWriter, result any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "result": []any{result}})
}

func TestQueryUsesRawD1Rows(t *testing.T) {
	db := newTestDB(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer token" {
			t.Fatalf("authorization = %q", got)
		}
		var request queryRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.SQL != "SELECT id, name FROM projects WHERE id = ?" || len(request.Params) != 1 || request.Params[0] != float64(7) {
			t.Fatalf("unexpected request: %#v", request)
		}
		response(w, map[string]any{
			"success": true,
			"meta":    map[string]any{"changes": 0, "last_row_id": "0"},
			"results": map[string]any{"columns": []string{"id", "name"}, "rows": [][]any{{7, "traceway"}}},
		})
	})

	row := db.QueryRowContext(context.Background(), "SELECT id, name FROM projects WHERE id = ?", 7)
	var id int
	var name string
	if err := row.Scan(&id, &name); err != nil {
		t.Fatal(err)
	}
	if id != 7 || name != "traceway" {
		t.Fatalf("got (%d, %q)", id, name)
	}
}

func TestExecExposesD1MetadataAndFormatsTime(t *testing.T) {
	now := time.Date(2026, 9, 16, 8, 0, 0, 123, time.UTC)
	db := newTestDB(t, func(w http.ResponseWriter, r *http.Request) {
		var request queryRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if got := request.Params[0]; got != now.Format(time.RFC3339Nano) {
			t.Fatalf("time parameter = %#v", got)
		}
		response(w, map[string]any{
			"success": true,
			"meta":    map[string]any{"changes": 2, "last_row_id": "41"},
			"results": map[string]any{"columns": []string{}, "rows": [][]any{}},
		})
	})

	result, err := db.Exec("UPDATE projects SET created_at = ?", now)
	if err != nil {
		t.Fatal(err)
	}
	changed, _ := result.RowsAffected()
	id, _ := result.LastInsertId()
	if changed != 2 || id != 41 {
		t.Fatalf("got changes=%d id=%d", changed, id)
	}
}

func TestTransactionIsExplicitlyRejected(t *testing.T) {
	db := newTestDB(t, func(http.ResponseWriter, *http.Request) { t.Fatal("request should not be made") })
	_, err := db.BeginTx(context.Background(), nil)
	if !errors.Is(err, ErrTransactionsUnsupported) {
		t.Fatalf("expected ErrTransactionsUnsupported, got %v", err)
	}
}

func TestPingUsesD1(t *testing.T) {
	db := newTestDB(t, func(w http.ResponseWriter, r *http.Request) {
		var request queryRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.SQL != "SELECT 1" {
			t.Fatalf("ping query = %q", request.SQL)
		}
		response(w, map[string]any{
			"success": true,
			"meta":    map[string]any{"changes": 0, "last_row_id": "0"},
			"results": map[string]any{"columns": []string{"1"}, "rows": [][]any{{1}}},
		})
	})
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatal(err)
	}
}
