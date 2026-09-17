//go:build cloudflare

package middleware

import "net/http"

// Only routes with an explicit D1 implementation may bypass database/sql
// transactions. Keeping this registry behind the Cloudflare build tag lets
// the upstream route declarations remain unchanged.
func cloudflareTransactionMode(method string, path string) transactionMode {
	if method == http.MethodGet {
		switch path {
		case "/api/auth/start/:provider", "/api/auth/callback/:provider":
			return transactionRequired
		default:
			return transactionRead
		}
	}

	switch path {
	case "/api/login", "/api/register", "/api/projects/batch":
		return transactionCommand
	case "/api/me/login-bundle", "/api/dashboards", "/api/has-organizations":
		return transactionRead
	default:
		return transactionRequired
	}
}
