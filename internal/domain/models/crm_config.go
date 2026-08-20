package models

import (
	"time"
)

// CRMConfig представляет конфигурацию CRM для пользователя
type CRMConfig struct {
	ID          uint32    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID      uint32    `gorm:"not null;index:idx_user_id" json:"user_id"`
	CRMType     string    `gorm:"type:varchar(50);not null" json:"crm_type"` // 'amocrm', 'bitrix24', etc.
	Name        string    `gorm:"type:varchar(255);not null" json:"name"`    // название для пользователя
	Subdomain   string    `gorm:"type:varchar(255)" json:"subdomain"`        // для AmoCRM
	Credentials string    `gorm:"type:text;not null" json:"credentials"`     // JSON с токенами, ключами
	Options     string    `gorm:"type:text;not null" json:"options"`         // JSON с дополнительными настройками (field_id источника и т.д.)
	Chanells    string    `gorm:"type:text" json:"chanells"`                 // JSON с настройками каналов (telegram, whatsapp, etc.)
	IsActive    bool      `gorm:"default:true" json:"is_active"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// CRMConfigOptions структура для хранения дополнительных настроек CRM
type CRMConfigOptions struct {
	MarusiaSourceFieldID  int64 `json:"marusia_source_field_id,omitempty"`  // ID поля "Источник перехода" для автоматического заполнения
	DefaultPipelineID     int64 `json:"default_pipeline_id,omitempty"`      // ID воронки по умолчанию для создания лидов (deprecated, используйте DefaultLeadPipelineID)
	DefaultLeadPipelineID int64 `json:"default_lead_pipeline_id,omitempty"` // ID воронки по умолчанию для UpdateLead
	DefaultLeadStatusID   int64 `json:"default_lead_status_id,omitempty"`   // ID статуса по умолчанию для UpdateLead
}

// CRMChannelSettings структура для настройки каналов (telegram, whatsapp, instagram, widget и т.д.)
type CRMChannelSettings struct {
	Assist           string   `json:"Assist"`           // Префикс для сообщений ассистента, например: "🤖 Ассистент"
	User             string   `json:"User"`             // Префикс для сообщений пользователя, например: "👤 Пользователь"
	Meta             string   `json:"Meta"`             // Текст метаданных, например: "Цель в диалоге достигнута"
	Voice            string   `json:"Voice"`            // Текст для голосовых сообщений, например: "голосовое сообщение"
	File             string   `json:"File"`             // Текст для отправленных файлов, например: "Отправлен файл"
	LeadName         string   `json:"LeadName"`         // Название лида по умолчанию, например: "AI диалог"
	Tags             []string `json:"Tags"`             // Теги для лида, например: ["MarusiaAI", "Новый клиент"]
	CreateNewContact bool     `json:"CreateNewContact"` // Создавать новый контакт при первом обращении
	CreateNewLead    bool     `json:"CreateNewLead"`    // Создавать новый лид
	ChatMessages     bool     `json:"ChatMessages"`     // Добавлять сообщения чата в CRM
	MetaExist        bool     `json:"MetaExist"`        // Добавлять метаданные в CRM
	AltContact       bool     `json:"AltContact"`       // Создавать контакты без номера телефона
	Telegram         int64    `json:"Telegram"`         // ID канала Telegram
	Instagram        int64    `json:"Instagram"`        // ID канала Instagram
	Widget           int64    `json:"Widget"`           // ID канала Widget
}

// CRMChannels структура для хранения настроек всех каналов
type CRMChannels struct {
	Telegram  *CRMChannelSettings `json:"telegram,omitempty"`
	WhatsApp  *CRMChannelSettings `json:"whatsapp,omitempty"`
	Instagram *CRMChannelSettings `json:"instagram,omitempty"`
	Widget    *CRMChannelSettings `json:"widget,omitempty"`
}

// UpdateChannelSettingsRequest структура для частичного обновления настроек канала (с указателями)
type UpdateChannelSettingsRequest struct {
	Assist           *string   `json:"Assist,omitempty"`
	User             *string   `json:"User,omitempty"`
	Meta             *string   `json:"Meta,omitempty"`
	Voice            *string   `json:"Voice,omitempty"`
	File             *string   `json:"File,omitempty"`
	LeadName         *string   `json:"LeadName,omitempty"`
	Tags             *[]string `json:"Tags,omitempty"`
	CreateNewContact *bool     `json:"CreateNewContact,omitempty"`
	CreateNewLead    *bool     `json:"CreateNewLead,omitempty"`
	ChatMessages     *bool     `json:"ChatMessages,omitempty"`
	MetaExist        *bool     `json:"MetaExist,omitempty"`
	AltContact       *bool     `json:"AltContact,omitempty"`
	Telegram         *int64    `json:"Telegram,omitempty"`
	Instagram        *int64    `json:"Instagram,omitempty"`
	Widget           *int64    `json:"Widget,omitempty"`
}

// TableName переопределяет имя таблицы
func (CRMConfig) TableName() string {
	return "crm_configs"
}

// CRMCredentials структура для хранения учетных данных различных CRM
type CRMCredentials struct {
	// AmoCRM
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RedirectURL  string `json:"redirect_url"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`

	// Bitrix24 (для будущего)
	WebhookURL string `json:"webhook_url"`

	// Другие поля по необходимости
}
