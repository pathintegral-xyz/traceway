package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

// Delivery is one intended notification send. Enqueue persists it; the drain
// worker delivers it with retries. AdapterConfig is a snapshot: it is never
// re-derived at send time, so later channel/contact-method edits do not affect
// rows already queued.
type Delivery struct {
	Kind               string
	AdapterType        string
	AdapterConfig      json.RawMessage
	Message            models.NotificationMessage
	NotBefore          *time.Time
	CancelKey          string
	PageNotificationId *int
	RuleId             *int
	ProjectId          *uuid.UUID
	ChannelName        string
}

// Enqueue inserts a pending outbox row. When called with a transaction, its
// commit is the durable promise; a standalone insert is durable on success.
// Callers should Wake() after that point.
func Enqueue(tx lit.Executor, d Delivery) (int, error) {
	messageJSON, err := json.Marshal(d.Message)
	if err != nil {
		return 0, err
	}
	now := time.Now().UTC()
	next := now
	if d.NotBefore != nil {
		next = d.NotBefore.UTC()
	}
	cfg := d.AdapterConfig
	if len(cfg) == 0 {
		cfg = json.RawMessage("{}")
	}
	if db.IsCloudflare() {
		var pageNotificationID, ruleID, projectID any
		if d.PageNotificationId != nil {
			pageNotificationID = *d.PageNotificationId
		}
		if d.RuleId != nil {
			ruleID = *d.RuleId
		}
		if d.ProjectId != nil {
			projectID = d.ProjectId.String()
		}
		result, err := tx.Exec(
			`INSERT INTO notification_outbox (kind, status, adapter_type, adapter_config, message, attempts, next_attempt_at, cancel_key, page_notification_id, rule_id, project_id, channel_name, last_error, created_at) VALUES (?, 'pending', ?, ?, ?, 0, ?, ?, ?, ?, ?, ?, '', ?)`,
			d.Kind, d.AdapterType, string(cfg), string(messageJSON), next, d.CancelKey, pageNotificationID, ruleID, projectID, d.ChannelName, now,
		)
		if err != nil {
			return 0, err
		}
		id, err := result.LastInsertId()
		return int(id), err
	}
	row := &models.OutboxDelivery{
		Kind:               d.Kind,
		Status:             models.OutboxPending,
		AdapterType:        d.AdapterType,
		AdapterConfig:      models.JSONText(cfg),
		Message:            models.JSONText(messageJSON),
		NextAttemptAt:      next,
		CancelKey:          d.CancelKey,
		PageNotificationId: d.PageNotificationId,
		RuleId:             d.RuleId,
		ProjectId:          d.ProjectId,
		ChannelName:        d.ChannelName,
		CreatedAt:          now,
	}
	return transactional.OutboxRepository.Enqueue(tx, row)
}

// CancelByKey flips every pending/sending row under the key to cancelled and
// mirrors their linked page_notifications rows. Runs in the caller's
// transaction. An in-flight send may still deliver once (at-least-once), but
// the row's final state stays cancelled: MarkSent/MarkFailedWithBackoff are
// guarded on status = 'sending'.
func CancelByKey(tx *sql.Tx, cancelKey string) error {
	now := time.Now().UTC()
	rows, err := transactional.OutboxRepository.FindCancellable(tx, cancelKey)
	if err != nil {
		return err
	}
	// notification_outbox first, matching the drain's finalize order; the
	// reverse order can deadlock against a concurrent finalize on Postgres.
	if err := transactional.OutboxRepository.CancelByKey(tx, cancelKey, now); err != nil {
		return err
	}
	for _, row := range rows {
		if row.PageNotificationId != nil {
			if err := transactional.PageNotificationRepository.MarkCancelled(tx, *row.PageNotificationId, now); err != nil {
				return err
			}
		}
	}
	return nil
}

func PageCancelKey(pageId int) string {
	return "page:" + strconv.Itoa(pageId)
}

// CancelForProject cancels everything still queued for a project (which has no
// foreign key from notification_outbox), in the caller's transaction.
func CancelForProject(tx *sql.Tx, projectId uuid.UUID) error {
	pages, err := transactional.PageRepository.FindByProject(tx, projectId, "", projectCancelPageLimit, 0)
	if err != nil {
		return err
	}
	for _, page := range pages {
		if err := CancelByKey(tx, PageCancelKey(page.Id)); err != nil {
			return err
		}
	}
	return transactional.OutboxRepository.CancelByProject(tx, projectId, time.Now().UTC())
}

const projectCancelPageLimit = 5000

// VerificationCancelKey scopes a contact method's verification sends so a new
// code supersedes the one before it.
func VerificationCancelKey(methodId int) string {
	return "verify:" + strconv.Itoa(methodId)
}

var wakeCh = make(chan struct{}, 1)

// Wake nudges the drain worker so a freshly enqueued delivery goes out
// immediately instead of waiting for the next poll. Non-blocking.
func Wake() {
	select {
	case wakeCh <- struct{}{}:
	default:
	}
}

// SendFunc performs one delivery attempt. Registered from cmd/run.go with the
// notifications-package implementation; the indirection exists because this
// package cannot import notifications (notifications imports it to enqueue).
type SendFunc func(ctx context.Context, adapterType string, adapterConfig json.RawMessage, msg models.NotificationMessage) error

var sender SendFunc

func RegisterSender(fn SendFunc) {
	sender = fn
}

// TerminalHook observes terminal outcomes (models.OutboxSent or
// models.OutboxFailed) for audit bookkeeping. Called outside any transaction;
// must not block.
type TerminalHook func(row *models.OutboxDelivery, status string, errorMsg string)

var terminalHook TerminalHook

func RegisterTerminalHook(fn TerminalHook) {
	terminalHook = fn
}
