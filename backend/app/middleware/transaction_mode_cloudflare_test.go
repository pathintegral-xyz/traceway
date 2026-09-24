//go:build cloudflare

package middleware

import "testing"

func TestCloudflareNotificationTransactionModes(t *testing.T) {
	tests := []struct {
		method string
		path   string
		want   transactionMode
	}{
		{"GET", "/api/notification-channels", transactionRead},
		{"POST", "/api/notification-channels", transactionCommand},
		{"PUT", "/api/notification-channels/:id", transactionCommand},
		{"DELETE", "/api/notification-channels/:id", transactionCommand},
		{"GET", "/api/notification-rules", transactionRead},
		{"POST", "/api/notification-rules", transactionCommand},
		{"PUT", "/api/notification-rules/:id", transactionCommand},
		{"DELETE", "/api/notification-rules/:id", transactionCommand},
		{"POST", "/api/notification-rules/:id/toggle", transactionCommand},
		{"POST", "/api/notification-rules/:id/snooze", transactionCommand},
		{"POST", "/api/notification-channels/:id/test", transactionRequired},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			if got := cloudflareTransactionMode(tt.method, tt.path); got != tt.want {
				t.Fatalf("cloudflareTransactionMode(%q, %q) = %d, want %d", tt.method, tt.path, got, tt.want)
			}
		})
	}
}
