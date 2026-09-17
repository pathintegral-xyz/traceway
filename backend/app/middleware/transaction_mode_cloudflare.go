//go:build cloudflare

package middleware

// Only routes with an explicit D1 implementation may bypass database/sql
// transactions. Keeping this registry behind the Cloudflare build tag lets
// the upstream route declarations remain unchanged.
func cloudflareTransactionMode(path string) transactionMode {
	switch path {
	case "/api/login", "/api/register", "/api/projects/batch":
		return transactionCommand
	case "/api/me/login-bundle", "/api/dashboards", "/api/has-organizations":
		return transactionRead
	default:
		return transactionRequired
	}
}
