package models

import (
	"time"
)

// Lead представляет универсальную модель лида для всех CRM
type Lead struct {
	ID              string         `json:"id,omitempty"`
	Name            string         `json:"name"`
	Price           int64          `json:"price,omitempty"`
	ResponsibleUser string         `json:"responsible_user,omitempty"`
	PipelineID      string         `json:"pipeline_id,omitempty"`
	StatusID        string         `json:"status_id,omitempty"`
	CreatedAt       time.Time      `json:"created_at,omitempty"`
	UpdatedAt       time.Time      `json:"updated_at,omitempty"`
	ClosedAt        time.Time      `json:"closed_at,omitempty"`
	CustomFields    map[string]any `json:"custom_fields,omitempty"`
	Tags            []string       `json:"tags,omitempty"`

	// Связанные данные
	Contacts []Contact `json:"contacts,omitempty"`
}

// Contact представляет универсальную модель контакта для всех CRM
type Contact struct {
	ID           string        `json:"id,omitempty"`
	Name         string        `json:"name"`
	FirstName    string        `json:"first_name,omitempty"`
	LastName     string        `json:"last_name,omitempty"`
	Phone        string        `json:"phone,omitempty"`
	Email        string        `json:"email,omitempty"`
	AltContact   string        `json:"alt_contact,omitempty"`
	CreatedAt    time.Time     `json:"created_at,omitempty"`
	UpdatedAt    time.Time     `json:"updated_at,omitempty"`
	CustomFields []CustomField `json:"custom_fields,omitempty"`
	Tags         []string      `json:"tags,omitempty"`
}

type CustomField struct {
	ID     int64              `json:"id,omitempty"`
	Name   string             `json:"name,omitempty"`
	Code   string             `json:"code,omitempty"`
	Values []CustomFieldValue `json:"values"`
}

type CustomFieldValue struct {
	Value  string `json:"value"`
	EnumID int64  `json:"enum_id,omitempty"`
}

// Company представляет универсальную модель компании для всех CRM
type Company struct {
	ID           string         `json:"id,omitempty"`
	Name         string         `json:"name"`
	CreatedAt    time.Time      `json:"created_at,omitempty"`
	UpdatedAt    time.Time      `json:"updated_at,omitempty"`
	CustomFields map[string]any `json:"custom_fields,omitempty"`
	Tags         []string       `json:"tags,omitempty"`
}

// CustomFieldEnum представляет значение enum для кастомного поля
type CustomFieldEnum struct {
	ID    int64  `json:"id"`
	Value string `json:"value"`
	Sort  int    `json:"sort"`
}

// CustomFieldMetadata представляет метаданные пользовательского поля в CRM
type CustomFieldMetadata struct {
	ID          int64             `json:"id"`
	Name        string            `json:"name"`
	Code        string            `json:"code"`
	FieldType   string            `json:"field_type"`
	Sort        int               `json:"sort"`
	IsApiOnly   bool              `json:"is_api_only"`
	IsMultiple  bool              `json:"is_multiple"`
	IsSystem    bool              `json:"is_system"`
	IsEditable  bool              `json:"is_editable"`
	IsRequired  bool              `json:"is_required"`
	IsVisible   bool              `json:"is_visible"`
	IsDeletable bool              `json:"is_deletable"`
	Enums       []CustomFieldEnum `json:"enums,omitempty"`
}

// CustomFieldsMetadataResponse представляет ответ API со списком метаданных кастомных полей
type CustomFieldsMetadataResponse struct {
	Fields []CustomFieldMetadata `json:"fields"`
}
