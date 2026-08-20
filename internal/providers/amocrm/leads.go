package amocrm

import (
	"Marusia_CRM/internal/domain/models"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/ikermy/air_logger/v2/pkg/logger"
)

// CreateLead создает новый лид в AmoCRM
func (p *AmoCRMProvider) CreateLead(_ context.Context, _ *models.Lead) (*models.Lead, error) {
	// TODO: Реализовать создание лида через прямой HTTP API
	logger.Warn("AmoCRM CreateLead: метод в разработке")
	return nil, fmt.Errorf("метод CreateLead еще не реализован")
}

// UpdateLead обновляет лид в AmoCRM
func (p *AmoCRMProvider) UpdateLead(ctx context.Context, leadID string, lead *models.Lead, userID uint32) (*models.Lead, error) {
	if p.subdomain == "" || p.accessToken == "" {
		return nil, fmt.Errorf("провайдер не инициализирован: отсутствует subdomain или access_token")
	}

	// Преобразуем leadID в int
	var leadIDInt int64
	if _, err := fmt.Sscanf(leadID, "%d", &leadIDInt); err != nil {
		return nil, fmt.Errorf("некорректный leadID: %w", err)
	}

	// Формируем payload для обновления
	updatePayload := map[string]any{
		"id": leadIDInt,
	}

	// Добавляем только те поля, которые нужно обновить
	if lead.Name != "" {
		updatePayload["name"] = lead.Name
	}
	if lead.StatusID != "" {
		var statusIDInt int64
		if _, err := fmt.Sscanf(lead.StatusID, "%d", &statusIDInt); err != nil {
			return nil, fmt.Errorf("некорректный status_id: %w", err)
		}
		updatePayload["status_id"] = statusIDInt
	}
	if lead.PipelineID != "" {
		var pipelineIDInt int64
		if _, err := fmt.Sscanf(lead.PipelineID, "%d", &pipelineIDInt); err != nil {
			return nil, fmt.Errorf("некорректный pipeline_id: %w", err)
		}
		updatePayload["pipeline_id"] = pipelineIDInt
	}
	if lead.ResponsibleUser != "" {
		var responsibleUserInt int64
		if _, err := fmt.Sscanf(lead.ResponsibleUser, "%d", &responsibleUserInt); err != nil {
			return nil, fmt.Errorf("некорректный responsible_user_id: %w", err)
		}
		updatePayload["responsible_user_id"] = responsibleUserInt
	}

	requestBody := []map[string]any{updatePayload}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("ошибка сериализации запроса: %w", err)
	}
	logger.Debug("AmoCRM UpdateLead request body: %s", string(jsonData), userID)

	apiURL := fmt.Sprintf("https://%s.amocrm.ru/api/v4/leads", p.subdomain)
	req, err := http.NewRequestWithContext(ctx, "PATCH", apiURL, bytes.NewReader(jsonData))
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
		logger.Error("AmoCRM API вернул ошибку при обновлении лида (%d): %s", resp.StatusCode, string(body), userID)
		return nil, fmt.Errorf("AmoCRM API ошибка (%d): %s", resp.StatusCode, string(body))
	}

	logger.Debug("AmoCRM UpdateLead response: %s", string(body), userID)

	var result struct {
		Embedded struct {
			Leads []struct {
				ID         int64  `json:"id"`
				Name       string `json:"name"`
				StatusID   int64  `json:"status_id"`
				PipelineID int64  `json:"pipeline_id"`
				UpdatedAt  int64  `json:"updated_at"`
			} `json:"leads"`
		} `json:"_embedded"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("ошибка парсинга ответа: %w", err)
	}

	if len(result.Embedded.Leads) == 0 {
		return nil, fmt.Errorf("пустой ответ при обновлении лида")
	}

	updated := result.Embedded.Leads[0]
	updatedLead := &models.Lead{
		ID:         fmt.Sprintf("%d", updated.ID),
		Name:       updated.Name,
		StatusID:   fmt.Sprintf("%d", updated.StatusID),
		PipelineID: fmt.Sprintf("%d", updated.PipelineID),
	}

	logger.Info("AmoCRM: лид '%s' (ID=%s) успешно обновлен", updatedLead.Name, updatedLead.ID, userID)
	return updatedLead, nil
}

// GetLead получает лид из AmoCRM по ID
func (p *AmoCRMProvider) GetLead(ctx context.Context, leadID string) (*models.Lead, error) {
	if p.subdomain == "" || p.accessToken == "" {
		return nil, fmt.Errorf("провайдер не инициализирован: отсутствует subdomain или access_token")
	}

	apiURL := fmt.Sprintf("https://%s.amocrm.ru/api/v4/leads/%s", p.subdomain, leadID)
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
		return nil, fmt.Errorf("лид с ID %s не найден", leadID)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AmoCRM API ошибка (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		ID         int64  `json:"id"`
		Name       string `json:"name"`
		Price      int64  `json:"price"`
		StatusID   int64  `json:"status_id"`
		PipelineID int64  `json:"pipeline_id"`
		CreatedAt  int64  `json:"created_at"`
		UpdatedAt  int64  `json:"updated_at"`
		ClosedAt   int64  `json:"closed_at"`
		Embedded   struct {
			Contacts []struct {
				ID int64 `json:"id"`
			} `json:"contacts"`
		} `json:"_embedded"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("ошибка парсинга ответа: %w", err)
	}

	lead := &models.Lead{
		ID:         fmt.Sprintf("%d", result.ID),
		Name:       result.Name,
		StatusID:   fmt.Sprintf("%d", result.StatusID),
		PipelineID: fmt.Sprintf("%d", result.PipelineID),
	}

	logger.Info("AmoCRM: лид получен, ID=%s, Name=%s", lead.ID, lead.Name)
	return lead, nil
}

// GetLeadsByContactID получает все лиды связанные с контактом
func (p *AmoCRMProvider) GetLeadsByContactID(ctx context.Context, contactID string, userID uint32) ([]*models.Lead, error) {
	if p.subdomain == "" || p.accessToken == "" {
		return nil, fmt.Errorf("провайдер не инициализирован: отсутствует subdomain или access_token")
	}

	// Получаем контакт с включенными лидами через with=leads
	apiURL := fmt.Sprintf("https://%s.amocrm.ru/api/v4/contacts/%s?with=leads", p.subdomain, contactID)
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
		return nil, fmt.Errorf("контакт с ID %s не найден", contactID)
	}
	if resp.StatusCode != http.StatusOK {
		logger.Error("AmoCRM API вернул ошибку при получении контакта (%d): %s", resp.StatusCode, string(body), userID)
		return nil, fmt.Errorf("AmoCRM API ошибка (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Embedded struct {
			Leads []struct {
				ID         int64  `json:"id"`
				Name       string `json:"name"`
				Price      int64  `json:"price"`
				StatusID   int64  `json:"status_id"`
				PipelineID int64  `json:"pipeline_id"`
				CreatedAt  int64  `json:"created_at"`
				UpdatedAt  int64  `json:"updated_at"`
			} `json:"leads"`
		} `json:"_embedded"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("ошибка парсинга ответа: %w", err)
	}

	leads := make([]*models.Lead, 0, len(result.Embedded.Leads))
	for _, l := range result.Embedded.Leads {
		lead := &models.Lead{
			ID:         fmt.Sprintf("%d", l.ID),
			Name:       l.Name,
			StatusID:   fmt.Sprintf("%d", l.StatusID),
			PipelineID: fmt.Sprintf("%d", l.PipelineID),
		}
		leads = append(leads, lead)
	}

	logger.Info("AmoCRM: получено %d лидов для контакта ID=%s", len(leads), contactID, userID)
	return leads, nil
}

// CreateAIDialogLead создает лид с указанным именем, тегами и привязкой к контакту
func (p *AmoCRMProvider) CreateAIDialogLead(ctx context.Context, contactID string, leadName string, tags []string, userID uint32) (*models.Lead, error) {
	if p.subdomain == "" || p.accessToken == "" {
		return nil, fmt.Errorf("провайдер не инициализирован: отсутствует subdomain или access_token")
	}
	if strings.TrimSpace(leadName) == "" {
		return nil, fmt.Errorf("leadName обязателен")
	}

	// Преобразуем contactID в int
	var contactIDInt int64
	if _, err := fmt.Sscanf(contactID, "%d", &contactIDInt); err != nil {
		return nil, fmt.Errorf("некорректный contactID: %w", err)
	}

	// Формируем _embedded с контактами
	embedded := map[string]any{
		"contacts": []map[string]any{{"id": contactIDInt}},
	}

	// Добавляем теги если указаны
	if len(tags) > 0 {
		tagObjects := make([]map[string]any, 0, len(tags))
		for _, tag := range tags {
			if strings.TrimSpace(tag) != "" {
				tagObjects = append(tagObjects, map[string]any{"name": tag})
			}
		}
		if len(tagObjects) > 0 {
			embedded["tags"] = tagObjects
		}
	}

	leadPayload := map[string]any{
		"name":      leadName,
		"_embedded": embedded,
	}

	requestBody := []map[string]any{leadPayload}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("ошибка сериализации запроса: %w", err)
	}
	logger.Debug("AmoCRM CreateAIDialogLead request body: %s", string(jsonData), userID)

	apiURL := fmt.Sprintf("https://%s.amocrm.ru/api/v4/leads", p.subdomain)
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(jsonData))
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
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		logger.Error("AmoCRM API вернул ошибку при создании лида (%d): %s", resp.StatusCode, string(body), userID)
		return nil, fmt.Errorf("AmoCRM API ошибка (%d): %s", resp.StatusCode, string(body))
	}

	logger.Debug("AmoCRM CreateAIDialogLead response: %s", string(body), userID)

	var result struct {
		Embedded struct {
			Leads []struct {
				ID     int64  `json:"id"`
				Name   string `json:"name"`
				Status int64  `json:"status_id"`
			} `json:"leads"`
		} `json:"_embedded"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("ошибка парсинга ответа: %w", err)
	}
	if len(result.Embedded.Leads) == 0 {
		return nil, fmt.Errorf("пустой ответ при создании лида")
	}
	created := result.Embedded.Leads[0]

	// Используем имя из ответа, если оно есть, иначе из запроса
	name := created.Name
	if name == "" {
		name = leadName
	}

	lead := &models.Lead{
		ID:       fmt.Sprintf("%d", created.ID),
		Name:     name,
		StatusID: fmt.Sprintf("%d", created.Status),
	}
	logger.Debug("AmoCRM: лид '%s' создан, ID=%s", lead.Name, lead.ID, userID)
	return lead, nil
}
