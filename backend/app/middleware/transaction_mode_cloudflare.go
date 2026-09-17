//go:build cloudflare

package middleware

// Only routes whose handlers have been converted to the D1 execution model
// may bypass database/sql transactions. Keeping this registry behind the
// Cloudflare build tag lets the upstream route declarations remain unchanged.
func cloudflareTransactionMode(_ string, path string) transactionMode {
	switch path {
	case "/api/login", "/api/register", "/api/projects/batch":
		return transactionCommand
	case "/api/me/login-bundle", "/api/dashboards", "/api/dashboards/library", "/api/dashboards/:id", "/api/dashboards/starred", "/api/dashboard-templates", "/api/organizations/:organizationId/members/:userId/project-roles", "/api/notification-channels", "/api/notification-rules", "/api/synthetics/checks", "/api/synthetics/checks/:id", "/api/synthetics/open-count", "/api/organizations/:organizationId/status-pages", "/api/organizations/:organizationId/incidents", "/api/organizations/:organizationId/incidents/:incidentId/updates", "/api/organizations/:organizationId/status-pages/:id/incidents", "/api/organizations/:organizationId/overview/counts", "/api/organizations/:organizationId/teams", "/api/escalation-policies", "/api/organizations/:organizationId/escalation-policies", "/api/post-mortems", "/api/post-mortems/:id", "/api/post-mortems/:id/activity", "/api/pages/open-count", "/api/pages/:id", "/api/contact-methods", "/api/user-notification-rules", "/api/ack/:token", "/api/has-organizations":
		return transactionRead
	default:
		return transactionRequired
	}
}
