package models

// Talk представляет беседу в CRM
type Talk struct {
	ID         string `json:"id"`
	EntityID   string `json:"entity_id"`
	EntityType string `json:"entity_type"` // contacts, leads, companies, customers
	CreatedBy  string `json:"created_by"`
	CreatedAt  int64  `json:"created_at"`
	UpdatedAt  int64  `json:"updated_at"`
	IsDeleted  bool   `json:"is_deleted"`
}
