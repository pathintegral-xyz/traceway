//go:build !cloudflare

package middleware

func cloudflareTransactionMode(string) transactionMode { return transactionRequired }
