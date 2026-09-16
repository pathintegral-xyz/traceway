//go:build cloudflare && !transactional_pg && !telemetry_ch && !telemetry_duckdb

package db

import (
	"context"
	"fmt"

	"github.com/tracewayapp/traceway/backend/app/db/d1http"
)

func BatchTelemetry(ctx context.Context, statements []d1http.Statement) error {
	if TelemetryD1 == nil {
		return fmt.Errorf("Cloudflare telemetry D1 batch connector is not initialized")
	}
	_, err := TelemetryD1.Batch(ctx, statements)
	return err
}
