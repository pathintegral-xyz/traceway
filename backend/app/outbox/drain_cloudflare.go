//go:build cloudflare

package outbox

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/db/d1http"
	"github.com/tracewayapp/traceway/backend/app/models"
	traceway "go.tracewayapp.com"
)

const d1OutboxColumns = "id, kind, status, adapter_type, adapter_config, message, attempts, next_attempt_at, claimed_at, cancel_key, page_notification_id, rule_id, project_id, channel_name, last_error, created_at, sent_at"

// drainOnceD1 keeps the same claim-before-send protocol as the SQL transaction
// implementation. The status guard is the ownership primitive: across any
// number of instances, only the batch whose pending -> sending update affects
// a row is allowed to deliver it.
func drainOnceD1(ctx context.Context, now time.Time) {
	if db.MainD1 == nil {
		traceway.CaptureException(fmt.Errorf("outbox D1 connector is not initialized"))
		return
	}
	if _, err := db.MainD1.Batch(ctx, []d1http.Statement{{
		SQL:    "UPDATE notification_outbox SET status = 'pending', next_attempt_at = ?, claimed_at = NULL WHERE status = 'sending' AND claimed_at < ?",
		Params: []any{now.UTC(), now.Add(-staleSendingAge).UTC()},
	}}); err != nil {
		traceway.CaptureException(fmt.Errorf("outbox reclaim on D1 failed: %w", err))
		return
	}

	due, err := lit.SelectNamed[models.OutboxDelivery](db.DB,
		"SELECT "+d1OutboxColumns+" FROM notification_outbox WHERE status = 'pending' AND next_attempt_at <= :now ORDER BY CASE WHEN kind = 'page' THEN 0 ELSE 1 END, next_attempt_at ASC, id ASC LIMIT :limit",
		lit.P{"now": now.UTC(), "limit": drainBatchSize})
	if err != nil {
		traceway.CaptureException(fmt.Errorf("outbox due query on D1 failed: %w", err))
		return
	}
	claimed, err := claimDueD1(ctx, due, now)
	if err != nil {
		traceway.CaptureException(fmt.Errorf("outbox claim on D1 failed: %w", err))
		return
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, drainConcurrency)
	for _, row := range claimed {
		wg.Add(1)
		sem <- struct{}{}
		go func(row *models.OutboxDelivery) {
			defer traceway.Recover()
			defer wg.Done()
			defer func() { <-sem }()
			sendRow(ctx, row)
		}(row)
	}
	wg.Wait()
	if len(due) == drainBatchSize {
		Wake()
	}
}

func claimDueD1(ctx context.Context, due []*models.OutboxDelivery, now time.Time) ([]*models.OutboxDelivery, error) {
	if len(due) == 0 {
		return nil, nil
	}
	statements := make([]d1http.Statement, len(due))
	for i, row := range due {
		statements[i] = d1http.Statement{
			SQL:    "UPDATE notification_outbox SET status = 'sending', attempts = attempts + 1, claimed_at = ? WHERE id = ? AND status = 'pending'",
			Params: []any{now.UTC(), row.Id},
		}
	}
	results, err := db.MainD1.Batch(ctx, statements)
	if err != nil {
		return nil, err
	}
	claimed := make([]*models.OutboxDelivery, 0, len(due))
	for i, result := range results {
		if result.Changes == 0 {
			continue
		}
		due[i].Attempts++
		claimed = append(claimed, due[i])
	}
	return claimed, nil
}

func finalizeRowD1(row *models.OutboxDelivery, sendErr error) {
	if sendErr != nil && row.Attempts >= maxAttempts {
		finalizeTerminalD1(row, sendErr.Error())
		return
	}
	now := time.Now().UTC()
	var statements []d1http.Statement
	if sendErr == nil {
		statements = append(statements, d1http.Statement{
			SQL:    "UPDATE notification_outbox SET status = 'sent', sent_at = ?, last_error = '' WHERE id = ? AND status = 'sending'",
			Params: []any{now, row.Id},
		})
		if row.PageNotificationId != nil {
			statements = append(statements, d1http.Statement{
				// This follows the guarded outbox update above in the same D1
				// batch. The EXISTS check makes cancellation win when the send
				// landed after a concurrent acknowledge or resolve.
				SQL:    "UPDATE page_notifications SET status = 'sent', sent_at = ? WHERE id = ? AND status = 'pending' AND EXISTS (SELECT 1 FROM notification_outbox WHERE id = ? AND status = 'sent')",
				Params: []any{now, *row.PageNotificationId, row.Id},
			})
		}
	} else {
		next := now.Add(backoffSchedule[backoffIndex(row.Attempts)])
		statements = append(statements, d1http.Statement{
			SQL:    "UPDATE notification_outbox SET status = 'pending', last_error = ?, next_attempt_at = ?, claimed_at = NULL WHERE id = ? AND status = 'sending'",
			Params: []any{sendErr.Error(), next, row.Id},
		})
	}
	results, err := db.MainD1.Batch(context.Background(), statements)
	if err != nil {
		traceway.CaptureException(fmt.Errorf("failed to finalize D1 outbox row %d: %w", row.Id, err))
		return
	}
	if sendErr != nil {
		traceway.CaptureException(fmt.Errorf("outbox delivery %d (%s via %s) attempt %d failed, will retry: %w", row.Id, row.Kind, row.AdapterType, row.Attempts, sendErr))
		return
	}
	if len(results) > 0 && results[0].Changes > 0 {
		atomic.AddUint64(&sentTotal, 1)
		if terminalHook != nil {
			terminalHook(row, models.OutboxSent, "")
		}
	}
}

func finalizeTerminalD1(row *models.OutboxDelivery, errorMsg string) {
	now := time.Now().UTC()
	statements := []d1http.Statement{{
		SQL:    "UPDATE notification_outbox SET status = 'failed', last_error = ?, sent_at = ? WHERE id = ? AND status = 'sending'",
		Params: []any{errorMsg, now, row.Id},
	}}
	if row.PageNotificationId != nil {
		statements = append(statements, d1http.Statement{
			SQL:    "UPDATE page_notifications SET status = 'failed', error_msg = ?, sent_at = ? WHERE id = ? AND status = 'pending' AND EXISTS (SELECT 1 FROM notification_outbox WHERE id = ? AND status = 'failed')",
			Params: []any{errorMsg, now, *row.PageNotificationId, row.Id},
		})
	}
	results, err := db.MainD1.Batch(context.Background(), statements)
	if err != nil {
		traceway.CaptureException(fmt.Errorf("failed to terminalize D1 outbox row %d: %w", row.Id, err))
		return
	}
	if len(results) == 0 || results[0].Changes == 0 {
		return
	}
	atomic.AddUint64(&terminalFailuresTotal, 1)
	if terminalHook != nil {
		terminalHook(row, models.OutboxFailed, errorMsg)
	}
}
