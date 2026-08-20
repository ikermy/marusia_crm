package repository

import (
	"Marusia_CRM/internal/domain/models"

	"github.com/ikermy/air_common/pkg/comdb"
)

// CRMRepository определяет интерфейс для работы с данными CRM
type CRMRepository interface {
	// CRM Configs (для API - возвращают данные без токенов)
	CreateCRMConfig(config *models.CRMConfig) error
	GetCRMConfig(id uint32) (*models.CRMConfig, error)
	GetCRMConfigByType(userID uint32, crmType string) (*models.CRMConfig, error)
	GetUserCRMConfigs(userID uint32) ([]models.CRMConfig, error)
	UpsertCRMConfig(config *models.CRMConfig) error
	UpdateCRMConfig(config *models.CRMConfig) error
	DeleteCRMConfig(id uint32) error

	// CRM Configs Internal (для внутреннего использования - возвращают полные credentials с токенами)
	GetCRMConfigInternal(id uint32) (*models.CRMConfig, error)
	GetCRMConfigByTypeInternal(userID uint32, crmType string) (*models.CRMConfig, error)

	// Mappings
	CreateMapping(mapping *models.CRMMapping) error
	GetMapping(id uint32) (*models.CRMMapping, error)
	GetUserMappings(userID uint32) ([]models.CRMMapping, error)
	GetActiveMappings(userID uint32, appID uint32) ([]models.CRMMapping, error)
	UpdateMapping(mapping *models.CRMMapping) error
	DeleteMapping(id uint32) error

	// OAuth States
	SaveOAuthState(state *models.OAuthState) error
	GetOAuthState(state string) (*models.OAuthState, error)
	DeleteOAuthState(state string) error
	CleanupExpiredOAuthStates() error
}

// ExternalDBRepository интерфейс для внешних методов БД (из AiR_Common)
type ExternalDBRepository interface {
	comdb.Exterior
}

// Repository объединяет все репозитории
type Repository struct {
	Internal CRMRepository
	External ExternalDBRepository
}
