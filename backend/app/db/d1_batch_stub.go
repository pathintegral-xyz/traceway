//go:build !cloudflare || transactional_pg || telemetry_ch || telemetry_duckdb

package db

import (
	"context"
	"fmt"

	"github.com/tracewayapp/traceway/backend/app/db/d1http"
)

func BatchTelemetry(context.Context, []d1http.Statement) error {
	return fmt.Errorf("Cloudflare telemetry D1 batches are unavailable in this build")
}

func BatchMain(context.Context, []d1http.Statement) ([]d1http.BatchResult, error) {
	return nil, fmt.Errorf("Cloudflare main D1 batches are unavailable in this build")
}
