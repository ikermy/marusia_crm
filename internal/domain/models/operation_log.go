package models

import (
	"time"
)

// OperationLog представляет лог операций с CRM
type OperationLog struct {
	ID            uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID        uint32    `gorm:"not null;index:idx_user_created" json:"user_id"`
	CRMConfigID   uint32    `json:"crm_config_id,omitempty"`
	OperationType string    `gorm:"type:varchar(50)" json:"operation_type"` // 'create_lead', 'update_contact', etc.
	RequestData   string    `gorm:"type:text" json:"request_data"`          // JSON
	ResponseData  string    `gorm:"type:text" json:"response_data"`         // JSON
	Status        string    `gorm:"type:varchar(20)" json:"status"`         // 'success', 'error'
	ErrorMessage  string    `gorm:"type:text" json:"error_message,omitempty"`
	DurationMs    int       `json:"duration_ms,omitempty"`
	CreatedAt     time.Time `gorm:"autoCreateTime;index:idx_user_created" json:"created_at"`
}

// TableName переопределяет имя таблицы
func (OperationLog) TableName() string {
	return "crm_operation_logs"
}
