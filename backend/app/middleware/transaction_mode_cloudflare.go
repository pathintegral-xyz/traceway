//go:build cloudflare

package middleware

// Only routes whose handlers have been converted to the D1 execution model
// may bypass database/sql transactions. Keeping this registry behind the
// Cloudflare build tag lets the upstream route declarations remain unchanged.
func cloudflareTransactionMode(method string, path string) transactionMode {
	if method == "POST" && (path == "/api/notification-channels" || path == "/api/notification-rules" || path == "/api/notification-rules/:id/toggle" || path == "/api/notification-rules/:id/snooze") {
		return transactionCommand
	}
	if (method == "PUT" || method == "DELETE") && (path == "/api/notification-channels/:id" || path == "/api/notification-rules/:id") {
		return transactionCommand
	}
	switch path {
	case "/api/login", "/api/register", "/api/projects/batch":
		return transactionCommand
	}

	readRequest := method == "GET"
	if method == "POST" {
		switch path {
		case "/api/pages", "/api/pages/for-issues", "/api/organizations/:organizationId/overview/pages", "/api/organizations/:organizationId/overview/incidents", "/api/synthetics/overview":
			readRequest = true
		}
	}
	if !readRequest {
		return transactionRequired
	}

	switch path {
	case "/api/me/login-bundle", "/api/setup/session", "/api/setup/plan", "/api/setup/drafts", "/api/dashboards", "/api/dashboards/library", "/api/dashboards/:id", "/api/dashboards/starred", "/api/dashboard-templates", "/api/organizations/:organizationId/members/:userId/project-roles", "/api/notification-channels", "/api/notification-rules", "/api/synthetics/checks", "/api/synthetics/overview", "/api/synthetics/checks/:id", "/api/synthetics/open-count", "/api/organizations/:organizationId/status-pages", "/api/organizations/:organizationId/incidents", "/api/organizations/:organizationId/incidents/:incidentId/updates", "/api/organizations/:organizationId/status-pages/:id/incidents", "/api/organizations/:organizationId/overview/servers", "/api/organizations/:organizationId/overview/pages", "/api/organizations/:organizationId/overview/counts", "/api/organizations/:organizationId/overview/incidents", "/api/organizations/:organizationId/overview/monitors", "/api/organizations/:organizationId/teams", "/api/organizations/:organizationId/schedules", "/api/organizations/:organizationId/schedules/:scheduleId", "/api/organizations/:organizationId/schedules/:scheduleId/timeline", "/api/organizations/:organizationId/oncall/now", "/api/oncall/current", "/api/escalation-policies", "/api/organizations/:organizationId/escalation-policies", "/api/post-mortems", "/api/post-mortems/:id", "/api/post-mortems/:id/activity", "/api/pages", "/api/pages/for-issues", "/api/pages/open-count", "/api/pages/:id", "/api/contact-methods", "/api/user-notification-rules", "/api/ack/:token", "/api/has-organizations":
		return transactionRead
	default:
		return transactionRequired
	}
}
