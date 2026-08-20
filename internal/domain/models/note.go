package models

// Note представляет примечание в CRM
type Note struct {
	ID                string `json:"id"`
	EntityID          string `json:"entity_id"`
	CreatedBy         int64  `json:"created_by,omitempty"`
	NoteType          string `json:"note_type"`
	Text              string `json:"text,omitempty"`
	ResponsibleUserID int64  `json:"responsible_user_id,omitempty"`
	CreatedAt         int64  `json:"created_at"`
	UpdatedAt         int64  `json:"updated_at"`
}

// CreateNoteRequest структура запроса на создание примечания
type CreateNoteRequest struct {
	NoteType          string `json:"note_type" binding:"required"`
	Text              string `json:"text" binding:"required"`
	CreatedBy         int64  `json:"created_by,omitempty"`
	ResponsibleUserID int64  `json:"responsible_user_id,omitempty"`
}
