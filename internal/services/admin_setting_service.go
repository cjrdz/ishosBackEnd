package services

import (
	"context"
	"fmt"

	"github.com/cjrdz/ishosBackEnd/internal/repository"
)

const AdminTabOrderSettingKey = "admin.tabs.order"

var defaultAdminTabOrder = []string{"ordenes", "categorias", "productos", "empleados", "usuarios", "herramientas", "configuracion"}

type AdminSettingService struct {
	repo *repository.AdminSettingRepository
}

func NewAdminSettingService(repo *repository.AdminSettingRepository) *AdminSettingService {
	return &AdminSettingService{repo: repo}
}

func (s *AdminSettingService) GetTabOrder(ctx context.Context) ([]string, error) {
	record, err := s.repo.GetByKey(ctx, AdminTabOrderSettingKey)
	if err != nil {
		return append([]string(nil), defaultAdminTabOrder...), nil
	}

	rawOrder, ok := record.Value["tab_order"]
	if !ok {
		return append([]string(nil), defaultAdminTabOrder...), nil
	}

	list, ok := rawOrder.([]any)
	if !ok {
		if typed, castOK := rawOrder.([]string); castOK {
			return sanitizeTabOrder(typed), nil
		}
		return append([]string(nil), defaultAdminTabOrder...), nil
	}

	order := make([]string, 0, len(list))
	for _, item := range list {
		text, ok := item.(string)
		if !ok {
			continue
		}
		order = append(order, text)
	}

	return sanitizeTabOrder(order), nil
}

func (s *AdminSettingService) UpdateTabOrder(ctx context.Context, tabOrder []string) ([]string, error) {
	sanitized := sanitizeTabOrder(tabOrder)
	if len(sanitized) == 0 {
		return nil, fmt.Errorf("tab_order cannot be empty")
	}

	_, err := s.repo.UpsertByKey(ctx, AdminTabOrderSettingKey, map[string]any{"tab_order": sanitized})
	if err != nil {
		return nil, err
	}

	return sanitized, nil
}

func DefaultAdminTabOrder() []string {
	return append([]string(nil), defaultAdminTabOrder...)
}

func sanitizeTabOrder(order []string) []string {
	allowed := map[string]bool{
		"ordenes":       true,
		"categorias":    true,
		"productos":     true,
		"empleados":     true,
		"usuarios":      true,
		"herramientas":  true,
		"configuracion": true,
	}

	seen := make(map[string]bool, len(order))
	sanitized := make([]string, 0, len(defaultAdminTabOrder))

	for _, item := range order {
		if !allowed[item] || seen[item] {
			continue
		}
		seen[item] = true
		sanitized = append(sanitized, item)
	}

	for _, fallback := range defaultAdminTabOrder {
		if !seen[fallback] {
			sanitized = append(sanitized, fallback)
		}
	}

	return sanitized
}
