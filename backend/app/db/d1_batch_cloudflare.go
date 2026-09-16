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

func BatchMain(ctx context.Context, statements []d1http.Statement) ([]d1http.BatchResult, error) {
	if MainD1 == nil {
		return nil, fmt.Errorf("Cloudflare main D1 batch connector is not initialized")
	}
	return MainD1.Batch(ctx, statements)
}
