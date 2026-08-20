package amocrm

import (
	"Marusia_CRM/internal/domain/models"
	"context"
	"fmt"

	"github.com/ikermy/air_logger/v2/pkg/logger"
)

// CreateCompany создает новую компанию в AmoCRM
func (p *AmoCRMProvider) CreateCompany(_ context.Context, _ *models.Company) (*models.Company, error) {
	// TODO: Реализовать создание компании через прямой HTTP API
	logger.Warn("AmoCRM CreateCompany: метод в разработке")
	return nil, fmt.Errorf("метод CreateCompany еще не реализован")
}

// UpdateCompany обновляет компанию в AmoCRM
func (p *AmoCRMProvider) UpdateCompany(_ context.Context, companyID string, _ *models.Company) (*models.Company, error) {
	// TODO: Реализовать обновление компании через прямой HTTP API
	logger.Warn("AmoCRM UpdateCompany: метод в разработке для company ID: %s", companyID)
	return nil, fmt.Errorf("метод UpdateCompany еще не реализован")
}

// GetCompany получает компанию из AmoCRM
func (p *AmoCRMProvider) GetCompany(_ context.Context, companyID string) (*models.Company, error) {
	// TODO: Реализовать получение компании через прямой HTTP API
	logger.Warn("AmoCRM GetCompany: метод в разработке для company ID: %s", companyID)
	return nil, fmt.Errorf("метод GetCompany еще не реализован")
}
