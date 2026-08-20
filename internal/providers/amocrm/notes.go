package amocrm

import (
	"Marusia_CRM/internal/domain/models"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/ikermy/air_logger/v2/pkg/logger"
)

// CreateLeadNote создает примечание для лида в AmoCRM
func (p *AmoCRMProvider) CreateLeadNote(ctx context.Context, leadID string, note *models.CreateNoteRequest, userID uint32) (*models.Note, error) {
	if p.subdomain == "" || p.accessToken == "" {
		return nil, fmt.Errorf("провайдер не инициализирован: отсутствует subdomain или access_token")
	}

	// Преобразуем leadID в int
	var leadIDInt int64
	if _, err := fmt.Sscanf(leadID, "%d", &leadIDInt); err != nil {
		return nil, fmt.Errorf("некорректный leadID: %w", err)
	}

	// Формируем тело запроса согласно AmoCRM Notes API
	params := map[string]any{
		"text": note.Text,
	}

	// Для extended_service_message обязательно указываем service
	if note.NoteType == "extended_service_message" || note.NoteType == "service_message" {
		params["service"] = "MarusiaAI"
	}

	requestBody := []map[string]any{
		{
			"entity_id": leadIDInt,
			"note_type": note.NoteType,
			"params":    params,
		},
	}

	// Добавляем опциональные поля если указаны
	if note.CreatedBy > 0 {
		requestBody[0]["created_by"] = note.CreatedBy
	}
	if note.ResponsibleUserID > 0 {
		requestBody[0]["responsible_user_id"] = note.ResponsibleUserID
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("ошибка сериализации запроса: %w", err)
	}

	logger.Debug("AmoCRM CreateLeadNote request body: %s", string(jsonData), userID)

	apiURL := fmt.Sprintf("https://%s.amocrm.ru/api/v4/leads/%s/notes", p.subdomain, leadID)
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
		logger.Error("AmoCRM API вернул ошибку при создании примечания (%d): %s", resp.StatusCode, string(body), userID)
		return nil, fmt.Errorf("AmoCRM API ошибка (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Embedded struct {
			Notes []struct {
				ID        int64  `json:"id"`
				EntityID  int64  `json:"entity_id"`
				CreatedBy int64  `json:"created_by"`
				CreatedAt int64  `json:"created_at"`
				UpdatedAt int64  `json:"updated_at"`
				NoteType  string `json:"note_type"`
			} `json:"notes"`
		} `json:"_embedded"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("ошибка парсинга ответа: %w", err)
	}
	if len(result.Embedded.Notes) == 0 {
		return nil, fmt.Errorf("пустой ответ при создании примечания")
	}

	created := result.Embedded.Notes[0]
	noteResult := &models.Note{
		ID:        fmt.Sprintf("%d", created.ID),
		EntityID:  fmt.Sprintf("%d", created.EntityID),
		CreatedBy: created.CreatedBy,
		NoteType:  created.NoteType,
		Text:      note.Text,
		CreatedAt: created.CreatedAt,
		UpdatedAt: created.UpdatedAt,
	}

	logger.Info("AmoCRM: примечание создано для лида ID=%s, note_id=%s, type=%s", leadID, noteResult.ID, noteResult.NoteType, userID)
	return noteResult, nil
}
