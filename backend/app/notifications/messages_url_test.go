package notifications

import (
	"testing"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/config"
)

func TestProjectDashboardURL(t *testing.T) {
	previous := config.Config.AppBaseURL
	config.Config.AppBaseURL = "https://traceway.example.com/"
	t.Cleanup(func() { config.Config.AppBaseURL = previous })
	projectId := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	for _, tc := range []struct {
		path string
		want string
	}{
		{"/issues/abc", "https://traceway.example.com/issues/abc?projectId=" + projectId.String()},
		{"/issues?preset=1h", "https://traceway.example.com/issues?preset=1h&projectId=" + projectId.String()},
		{"/endpoints?from=2026-10-13T01:00:00Z&to=2026-10-13T01:03:00Z", "https://traceway.example.com/endpoints?from=2026-10-13T01%3A00%3A00Z&projectId=" + projectId.String() + "&to=2026-10-13T01%3A03%3A00Z"},
		{"/ai-traces/conversations?flagged=1&preset=24h", "https://traceway.example.com/ai-traces/conversations?flagged=1&preset=24h&projectId=" + projectId.String()},
		{"/?projectId=wrong#details", "https://traceway.example.com/?projectId=" + projectId.String() + "#details"},
		{"", ""},
		{"https://external.example.com/details", "https://external.example.com/details"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			if got := projectDashboardURL(tc.path, projectId); got != tc.want {
				t.Errorf("projectDashboardURL = %q, want %q", got, tc.want)
			}
		})
	}
}
