//go:build cloudflare

package outbox

import (
	"sync/atomic"
	"time"

	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
)

func healthSnapshotD1() (*HealthStats, error) {
	counts, err := lit.SelectSingleNamed[models.OutboxHealthCounts](db.DB,
		"SELECT COALESCE(SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END), 0) AS pending_count, COALESCE(SUM(CASE WHEN status = 'sending' THEN 1 ELSE 0 END), 0) AS sending_count, COALESCE(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END), 0) AS failed_count FROM notification_outbox", lit.P{})
	if err != nil {
		return nil, err
	}
	oldest, err := lit.SelectSingleNamed[models.OutboxDelivery](db.DB,
		"SELECT "+d1OutboxColumns+" FROM notification_outbox WHERE status = 'pending' ORDER BY next_attempt_at ASC, id ASC LIMIT 1", lit.P{})
	if err != nil {
		return nil, err
	}
	stats := &HealthStats{SentTotal: atomic.LoadUint64(&sentTotal), TerminalFailuresTotal: atomic.LoadUint64(&terminalFailuresTotal)}
	if counts != nil {
		stats.Pending = counts.PendingCount
		stats.Sending = counts.SendingCount
		stats.FailedRows = counts.FailedCount
	}
	if oldest != nil {
		if age := time.Since(oldest.NextAttemptAt); age > 0 {
			stats.OldestPendingAgeSec = int64(age.Seconds())
		}
	}
	return stats, nil
}
