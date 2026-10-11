//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package outbox

import (
	"github.com/tracewayapp/traceway/backend/app/db"
	"sync"
	"testing"
)

func TestIncidentEnqueueIsAtomicAndPersistsAcrossDelivery(t *testing.T) {
	setupOutboxDB(t)
	d := Delivery{Kind: "rule", AdapterType: "feishu", Message: testMessage("SQL failure"), IncidentKey: "project:channel:trace"}
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := Enqueue(db.DB, d); err != nil {
				t.Errorf("enqueue: %v", err)
			}
		}()
	}
	wg.Wait()
	var count int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM notification_outbox").Scan(&count); err != nil || count != 1 {
		t.Fatalf("expected one queued incident, got %d: %v", count, err)
	}
	if _, err := db.DB.Exec("UPDATE notification_outbox SET status='sent'"); err != nil {
		t.Fatal(err)
	}
	d.Message = testMessage("HTTP 500 from the same failure")
	if id, err := Enqueue(db.DB, d); err != nil || id != 0 {
		t.Fatalf("delayed signal must be deduped: id=%d err=%v", id, err)
	}
	d.IncidentKey = "project:channel:another-trace"
	if id, err := Enqueue(db.DB, d); err != nil || id == 0 {
		t.Fatalf("independent trace must enqueue: id=%d err=%v", id, err)
	}
	d.IncidentKey = "project:another-channel:trace"
	if id, err := Enqueue(db.DB, d); err != nil || id == 0 {
		t.Fatalf("independent channel must enqueue: id=%d err=%v", id, err)
	}
}
