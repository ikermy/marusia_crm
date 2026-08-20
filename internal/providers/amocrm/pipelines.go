package amocrm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/ikermy/air_logger/v2/pkg/logger"
)

// GetPipelines получает список воронок со статусами
func (p *AmoCRMProvider) GetPipelines(ctx context.Context, userID uint32) (any, error) {
	if p.subdomain == "" || p.accessToken == "" {
		return nil, fmt.Errorf("провайдер не инициализирован: отсутствует subdomain или access_token")
	}

	apiURL := fmt.Sprintf("https://%s.amocrm.ru/api/v4/leads/pipelines", p.subdomain)
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

	if resp.StatusCode != http.StatusOK {
		logger.Error("AmoCRM API вернул ошибку при получении воронок (%d): %s", resp.StatusCode, string(body), userID)
		return nil, fmt.Errorf("AmoCRM API ошибка (%d): %s", resp.StatusCode, string(body))
	}

	var result any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("ошибка парсинга ответа: %w", err)
	}

	logger.Info("AmoCRM: получены воронки и статусы", userID)
	return result, nil
}
