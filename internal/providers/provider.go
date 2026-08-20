package providers

import (
	"Marusia_CRM/internal/domain/models"
	"context"
)

// CRMProvider представляет общий интерфейс для всех CRM систем
type CRMProvider interface {
	// GetProviderType возвращает тип провайдера ('amocrm', 'bitrix24', etc.)
	GetProviderType() string

	// Initialize инициализирует провайдер с учетными данными
	Initialize(ctx context.Context, userID uint32, config *models.CRMConfig, httpClient any) error

	// ValidateCredentials проверяет корректность учетных данных
	ValidateCredentials(ctx context.Context) error

	// TestConnection проверяет подключение к CRM
	TestConnection(ctx context.Context) error

	// RefreshToken обновляет токен доступа (если требуется)
	RefreshToken(ctx context.Context) error

	// Leads
	CreateLead(ctx context.Context, lead *models.Lead) (*models.Lead, error)
	UpdateLead(ctx context.Context, leadID string, lead *models.Lead, userID uint32) (*models.Lead, error)
	GetLead(ctx context.Context, leadID string) (*models.Lead, error)
	GetLeadsByContactID(ctx context.Context, contactID string, userID uint32) ([]*models.Lead, error)
	CreateAIDialogLead(ctx context.Context, contactID string, leadName string, tags []string, userID uint32) (*models.Lead, error)

	// Contacts
	CreateContact(ctx context.Context, userID uint32, contact *models.Contact) (*models.Contact, error)
	UpdateContact(ctx context.Context, userID uint32, contactID string, contact *models.Contact) (*models.Contact, error)
	GetContact(ctx context.Context, userID uint32, contactID string) (*models.Contact, error)
	FindContactByPhone(ctx context.Context, userID uint32, phoneNumber string) (*models.Contact, error)
	FindContactByAltContact(ctx context.Context, userID uint32, altContact string) (*models.Contact, error)
	GetCustomFields(ctx context.Context, userID uint32, entityType string) (*models.CustomFieldsMetadataResponse, error)
	SetMarusiaSource(ctx context.Context, userID uint32, contactID string) error

	// Companies
	CreateCompany(ctx context.Context, company *models.Company) (*models.Company, error)
	UpdateCompany(ctx context.Context, companyID string, company *models.Company) (*models.Company, error)
	GetCompany(ctx context.Context, companyID string) (*models.Company, error)

	// Talks (Беседы)
	GetTalk(ctx context.Context, talkID string) (*models.Talk, error)

	// Notes (Примечания)
	CreateLeadNote(ctx context.Context, leadID string, note *models.CreateNoteRequest, userID uint32) (*models.Note, error)

	// Pipelines (Воронки)
	GetPipelines(ctx context.Context, userID uint32) (any, error)

	// Webhooks
	ParseWebhook(ctx context.Context, payload []byte) (any, error)
}
