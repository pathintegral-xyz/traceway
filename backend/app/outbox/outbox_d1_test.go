//go:build cloudflare && !transactional_pg && !telemetry_ch && !telemetry_duckdb

package outbox

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/db/d1http"
	"github.com/tracewayapp/traceway/backend/app/models"
)

func TestEnqueueStandaloneD1(t *testing.T) {
	var request struct {
		SQL    string `json:"sql"`
		Params []any  `json:"params"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode D1 request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"result": []any{map[string]any{
				"success": true,
				"meta": map[string]any{"changes": 1, "last_row_id": "17"},
				"results": map[string]any{"columns": []string{}, "rows": [][]any{}},
			}},
		})
	}))
	defer server.Close()

	conn, err := d1http.Open(d1http.Config{AccountID: "account", DatabaseID: "database", APIToken: "token", Endpoint: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	previousDB, previousDriver := db.DB, db.Driver
	db.DB, db.Driver = conn, lit.SQLite
	defer func() { db.DB, db.Driver = previousDB, previousDriver }()
	models.Init(db.Driver)

	projectID := uuid.New()
	ruleID := 42
	id, err := Enqueue(db.DB, Delivery{
		Kind:          models.OutboxKindRule,
		AdapterType:   "feishu",
		AdapterConfig: json.RawMessage(`{"webhookUrl":"https://example.com/bot"}`),
		Message:       models.NotificationMessage{Subject: "test", Body: "body"},
		RuleId:        &ruleID,
		ProjectId:     &projectID,
		ChannelName:   "Feishu errors",
	})
	if err != nil {
		t.Fatalf("enqueue on D1: %v", err)
	}
	if id != 17 {
		t.Fatalf("insert id = %d, want 17", id)
	}
	if !strings.Contains(request.SQL, "notification_outbox") {
		t.Fatalf("unexpected D1 insert: %q", request.SQL)
	}
	configFound, messageFound := false, false
	for _, param := range request.Params {
		if param == `{"webhookUrl":"https://example.com/bot"}` {
			configFound = true
		}
		if text, ok := param.(string); ok && strings.Contains(text, `"Subject":"test"`) {
			messageFound = true
		}
	}
	if !configFound {
		t.Fatal("D1 insert did not receive JSON adapter config as text")
	}
	if !messageFound {
		t.Fatal("D1 insert did not receive JSON message as text")
	}
}
