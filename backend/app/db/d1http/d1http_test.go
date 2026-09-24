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

func TestExecSendsJSONBytesAsText(t *testing.T) {
	db := newTestDB(t, func(w http.ResponseWriter, r *http.Request) {
		var request queryRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if got := request.Params[0]; got != `{"recipients":["test@example.com"]}` {
			t.Fatalf("JSON parameter = %#v", got)
		}
		response(w, map[string]any{
			"success": true,
			"meta":    map[string]any{"changes": 1, "last_row_id": "1"},
			"results": map[string]any{"columns": []string{}, "rows": [][]any{}},
		})
	})

	if _, err := db.Exec("INSERT INTO notification_channels (config) VALUES (?)", []byte(`{"recipients":["test@example.com"]}`)); err != nil {
		t.Fatal(err)
	}
}

func TestBooleanParametersUseSQLiteIntegers(t *testing.T) {
	db := newTestDB(t, func(w http.ResponseWriter, r *http.Request) {
		var request queryRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if len(request.Params) != 2 || request.Params[0] != float64(1) || request.Params[1] != float64(0) {
			t.Fatalf("boolean parameters = %#v", request.Params)
		}
		response(w, map[string]any{
			"success": true,
			"meta":    map[string]any{"changes": 1, "last_row_id": "1"},
			"results": map[string]any{"columns": []string{}, "rows": [][]any{}},
		})
	})
	if _, err := db.Exec("INSERT INTO notification_rules (enabled, archived) VALUES (?, ?)", true, false); err != nil {
		t.Fatal(err)
	}
}

func TestBatchBooleanParametersUseSQLiteIntegers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request batchRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if len(request.Batch) != 1 || len(request.Batch[0].Params) != 2 ||
			request.Batch[0].Params[0] != float64(1) || request.Batch[0].Params[1] != float64(0) {
			t.Fatalf("batch boolean parameters = %#v", request.Batch)
		}
		response(w, map[string]any{
			"success": true,
			"meta":    map[string]any{"changes": 1, "last_row_id": "1"},
			"results": map[string]any{"columns": []string{}, "rows": [][]any{}},
		})
	}))
	defer server.Close()
	connector, err := NewConnector(Config{AccountID: "account", DatabaseID: "database", APIToken: "token", Endpoint: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := connector.Batch(context.Background(), []Statement{{SQL: "INSERT INTO flags VALUES (?, ?)", Params: []any{true, false}}}); err != nil {
		t.Fatal(err)
	}
}

func TestScanLegacyBase64Config(t *testing.T) {
	db := newTestDB(t, func(w http.ResponseWriter, r *http.Request) {
		response(w, map[string]any{
			"success": true,
			"meta":    map[string]any{"changes": 0, "last_row_id": "0"},
			"results": map[string]any{
				"columns": []string{"config"},
				"rows":    [][]any{{"eyJyZWNpcGllbnRzIjpbInRlc3RAZXhhbXBsZS5jb20iXX0="}},
			},
		})
	})
	var config json.RawMessage
	if err := db.QueryRow("SELECT config FROM notification_channels").Scan(&config); err != nil {
		t.Fatal(err)
	}
	if got, err := json.Marshal(config); err != nil || string(got) != `{"recipients":["test@example.com"]}` {
		t.Fatalf("legacy base64 config marshalled as %s, error %v", got, err)
	}
}

func TestScanJSONConfig(t *testing.T) {
	db := newTestDB(t, func(w http.ResponseWriter, r *http.Request) {
		response(w, map[string]any{
			"success": true,
			"meta":    map[string]any{"changes": 0, "last_row_id": "0"},
			"results": map[string]any{
				"columns": []string{"config"},
				"rows":    [][]any{{`{"recipients":["test@example.com"]}`}},
			},
		})
	})
	var config json.RawMessage
	if err := db.QueryRow("SELECT config FROM notification_channels").Scan(&config); err != nil {
		t.Fatal(err)
	}
	if string(config) != `{"recipients":["test@example.com"]}` {
		t.Fatalf("config = %s", config)
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

func TestBatchSendsFiniteAtomicWriteSet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request batchRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if len(request.Batch) != 2 || request.Batch[0].SQL != "INSERT INTO events (id) VALUES (?)" || request.Batch[1].SQL != "INSERT INTO notification_outbox (event_id) VALUES (?)" {
			t.Fatalf("unexpected batch: %#v", request)
		}
		if request.Batch[0].Params[0] != "event-1" || request.Batch[1].Params[0] != "event-1" {
			t.Fatalf("unexpected batch params: %#v", request.Batch)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "result": []any{
			map[string]any{"success": true, "meta": map[string]any{"changes": 1, "last_row_id": "41"}, "results": map[string]any{"columns": []string{}, "rows": [][]any{}}},
			map[string]any{"success": true, "meta": map[string]any{"changes": 1, "last_row_id": "42"}, "results": map[string]any{"columns": []string{}, "rows": [][]any{}}},
		}})
	}))
	defer server.Close()

	connector, err := NewConnector(Config{AccountID: "account", DatabaseID: "database", APIToken: "token", Endpoint: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	results, err := connector.Batch(context.Background(), []Statement{
		{SQL: "INSERT INTO events (id) VALUES (?)", Params: []any{"event-1"}},
		{SQL: "INSERT INTO notification_outbox (event_id) VALUES (?)", Params: []any{"event-1"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results[0].LastInsertID != 41 || results[1].LastInsertID != 42 {
		t.Fatalf("unexpected results: %#v", results)
	}
}
