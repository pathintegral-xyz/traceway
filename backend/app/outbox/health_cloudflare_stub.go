//go:build !cloudflare

package outbox

import "errors"

func healthSnapshotD1() (*HealthStats, error) {
	return nil, errors.New("Cloudflare D1 outbox health is unavailable in this build")
}
