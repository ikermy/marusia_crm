package models

import "time"

// OAuthState представляет временное хранилище для OAuth state
type OAuthState struct {
	State        string    `json:"state"`
	UserID       uint32    `json:"user_id"`
	ClientID     string    `json:"client_id"`
	ClientSecret string    `json:"client_secret"`
	RedirectURL  string    `json:"redirect_url"`
	Subdomain    string    `json:"subdomain"`
	CRMType      string    `json:"crm_type"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}
