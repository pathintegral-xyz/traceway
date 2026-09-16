//go:build !cloudflare

package outbox

import (
	"context"
	"time"

	"github.com/tracewayapp/traceway/backend/app/models"
)

func drainOnceD1(context.Context, time.Time)            {}
func finalizeRowD1(*models.OutboxDelivery, error)       {}
func finalizeTerminalD1(*models.OutboxDelivery, string) {}
