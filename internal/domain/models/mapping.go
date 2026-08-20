package models

import (
	"time"
)

// TODO фактически сейчас это задел на будущее когда будет поддержка нескольких CRM

// CRMMapping представляет маппинг между приложением и CRM
type CRMMapping struct {
	ID                uint32    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID            uint32    `gorm:"not null;index:idx_user_id;index:idx_app_id" json:"user_id"`
	ApplicationID     uint32    `gorm:"not null;index:idx_app_id" json:"application_id"`
	CRMConfigID       uint32    `gorm:"not null" json:"crm_config_id"`
	FieldMapping      string    `gorm:"type:text" json:"field_mapping"`               // JSON маппинг полей
	PipelineID        string    `gorm:"type:varchar(100)" json:"pipeline_id"`         // ID воронки в CRM
	StatusID          string    `gorm:"type:varchar(100)" json:"status_id"`           // ID статуса в воронке
	ResponsibleUserID string    `gorm:"type:varchar(100)" json:"responsible_user_id"` // ID ответственного
	IsActive          bool      `gorm:"default:true" json:"is_active"`
	Priority          int       `gorm:"default:0" json:"priority"` // порядок обработки
	CreatedAt         time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt         time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	// Связи
	CRMConfig CRMConfig `gorm:"foreignKey:CRMConfigID;constraint:OnDelete:CASCADE" json:"-"`
}

// TableName переопределяет имя таблицы
func (CRMMapping) TableName() string {
	return "crm_mappings"
}

// FieldMappingRules структура для JSON маппинга полей
type FieldMappingRules struct {
	LeadFields    map[string]string `json:"lead_fields,omitempty"`    // маппинг полей лида
	ContactFields map[string]string `json:"contact_fields,omitempty"` // маппинг полей контакта
	CustomFields  map[string]string `json:"custom_fields,omitempty"`  // кастомные поля
}
