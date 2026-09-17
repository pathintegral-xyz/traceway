//go:build !cloudflare

package middleware

func cloudflareTransactionMode(string, string) transactionMode { return transactionRequired }
