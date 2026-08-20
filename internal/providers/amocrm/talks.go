package amocrm

import (
	"Marusia_CRM/internal/domain/models"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/ikermy/air_logger/v2/pkg/logger"
)

// GetTalk получает беседу (talk) из AmoCRM по ID
func (p *AmoCRMProvider) GetTalk(ctx context.Context, talkID string) (*models.Talk, error) {
	if p.subdomain == "" || p.accessToken == "" {
		return nil, fmt.Errorf("провайдер не инициализирован: отсутствует subdomain или access_token")
	}

	apiURL := fmt.Sprintf("https://%s.amocrm.ru/api/v4/talks/%s", p.subdomain, talkID)
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания HTTP запроса: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.DoRequestWithRetry(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения запроса к AmoCRM API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("беседа с ID %s не найдена", talkID)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AmoCRM API ошибка (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		ID         int64  `json:"id"`
		EntityID   int64  `json:"entity_id"`
		EntityType string `json:"entity_type"`
		CreatedBy  int64  `json:"created_by"`
		CreatedAt  int64  `json:"created_at"`
		UpdatedAt  int64  `json:"updated_at"`
		IsDeleted  bool   `json:"is_deleted"`
		Embedded   struct {
			Contact struct {
				ID int64 `json:"id"`
			} `json:"contact"`
			Lead struct {
				ID int64 `json:"id"`
			} `json:"lead"`
		} `json:"_embedded"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("ошибка парсинга ответа: %w", err)
	}

	talk := &models.Talk{
		ID:         fmt.Sprintf("%d", result.ID),
		EntityID:   fmt.Sprintf("%d", result.EntityID),
		EntityType: result.EntityType,
		CreatedBy:  fmt.Sprintf("%d", result.CreatedBy),
		CreatedAt:  result.CreatedAt,
		UpdatedAt:  result.UpdatedAt,
		IsDeleted:  result.IsDeleted,
	}

	logger.Info("AmoCRM: беседа получена, ID=%s, EntityType=%s", talk.ID, talk.EntityType)
	return talk, nil
}
