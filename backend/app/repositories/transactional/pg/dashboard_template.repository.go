//go:build transactional_pg

package pg

import (

	"github.com/tracewayapp/traceway/backend/app/models"

	"github.com/tracewayapp/lit/v2"
)

type dashboardTemplateRepository struct{}

const dashboardTemplateColumns = "id, key, name, description, category, definition, created_at, updated_at"

func (r *dashboardTemplateRepository) FindAll(tx lit.Executor) ([]*models.DashboardTemplate, error) {
	return lit.SelectNamed[models.DashboardTemplate](
		tx,
		"SELECT "+dashboardTemplateColumns+" FROM dashboard_templates ORDER BY category ASC, name ASC",
		lit.P{},
	)
}

func (r *dashboardTemplateRepository) FindByKey(tx lit.Executor, key string) (*models.DashboardTemplate, error) {
	return lit.SelectSingleNamed[models.DashboardTemplate](
		tx,
		"SELECT "+dashboardTemplateColumns+" FROM dashboard_templates WHERE key = :key",
		lit.P{"key": key},
	)
}

func (r *dashboardTemplateRepository) Create(tx lit.Executor, template *models.DashboardTemplate) (int, error) {
	return lit.Insert[models.DashboardTemplate](tx, template)
}

var DashboardTemplateRepository = dashboardTemplateRepository{}
