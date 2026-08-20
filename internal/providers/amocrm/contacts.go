package amocrm

import (
	"Marusia_CRM/internal/domain/models"
	"Marusia_CRM/internal/pkg/phone"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/ikermy/air_logger/v2/pkg/logger"
)

// CreateContact создает новый контакт в AmoCRM
// Использует API: POST /api/v4/contacts
func (p *AmoCRMProvider) CreateContact(ctx context.Context, userID uint32, contact *models.Contact) (*models.Contact, error) {
	if p.subdomain == "" || p.accessToken == "" {
		return nil, fmt.Errorf("провайдер не инициализирован: отсутствует subdomain или access_token")
	}

	// Подготавливаем структуру запроса согласно документации AmoCRM
	requestBody := []map[string]any{
		{
			"name":       contact.Name,
			"first_name": contact.FirstName,
			"last_name":  contact.LastName,
		},
	}

	// Добавляем custom_fields_values для телефона и email
	var customFields []map[string]any

	if contact.Phone != "" {
		customFields = append(customFields, map[string]any{
			"field_code": "PHONE",
			"values": []map[string]any{
				{
					"value":     contact.Phone,
					"enum_code": "WORK",
				},
			},
		})
	}

	if contact.Email != "" {
		customFields = append(customFields, map[string]any{
			"field_code": "EMAIL",
			"values": []map[string]any{
				{
					"value":     contact.Email,
					"enum_code": "WORK",
				},
			},
		})
	}

	// Обработка AltContact: если Phone пустой, но AltContact есть
	// Берем ID кастомного поля из contact.CustomFields[0].ID
	if contact.Phone == "" && contact.AltContact != "" && len(contact.CustomFields) > 0 {
		altContactFieldID := contact.CustomFields[0].ID
		if altContactFieldID > 0 {
			logger.Debug("Создание контакта с AltContact: %s в поле ID=%d", contact.AltContact, altContactFieldID)
			customFields = append(customFields, map[string]any{
				"field_id": altContactFieldID,
				"values": []map[string]any{
					{
						"value": contact.AltContact,
					},
				},
			})
		} else {
			logger.Warn("AltContact указан, но CustomFields[0].ID пустой", userID)
		}
	}

	// Добавляем пользовательские custom_fields из запроса
	for _, cf := range contact.CustomFields {
		field := map[string]any{}

		// Используем ID если есть, иначе code
		if cf.ID > 0 {
			field["field_id"] = cf.ID
		} else if cf.Code != "" {
			field["field_code"] = cf.Code
		} else {
			continue // Пропускаем поле, если нет ни ID ни Code
		}

		// Конвертируем значения
		values := make([]map[string]any, 0, len(cf.Values))
		for _, v := range cf.Values {
			val := map[string]any{
				"value": v.Value,
			}
			if v.EnumID > 0 {
				val["enum_id"] = v.EnumID
			}
			values = append(values, val)
		}

		if len(values) > 0 {
			field["values"] = values
			customFields = append(customFields, field)
		}
	}

	if len(customFields) > 0 {
		requestBody[0]["custom_fields_values"] = customFields
	}

	// Добавляем теги если есть
	if len(contact.Tags) > 0 {
		tags := make([]map[string]any, 0, len(contact.Tags))
		for _, tag := range contact.Tags {
			tags = append(tags, map[string]any{
				"name": tag,
			})
		}
		requestBody[0]["_embedded"] = map[string]any{
			"tags": tags,
		}
	}

	// Сериализуем в JSON
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("ошибка сериализации запроса: %w", err)
	}

	// Логируем тело запроса для отладки
	logger.Debug("AmoCRM CreateContact request body: %s", string(jsonData), userID)

	// Формируем URL
	apiURL := fmt.Sprintf("https://%s.amocrm.ru/api/v4/contacts", p.subdomain)

	// Создаем HTTP запрос
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.accessToken)
	req.Header.Set("Content-Type", "application/json")

	// Выполняем запрос с механизмом повторных попыток
	resp, err := p.DoRequestWithRetry(ctx, req)
	if err != nil {
		logger.Error("Ошибка при выполнении HTTP запроса к AmoCRM API (URL: %s): %v", apiURL, err, userID)
		return nil, fmt.Errorf("ошибка выполнения запроса к AmoCRM API (проверьте интернет-соединение и subdomain '%s'): %w", p.subdomain, err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Читаем ответ
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа: %w", err)
	}

	// Проверяем статус
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		logger.Error("AmoCRM API вернул ошибку (%d): %s", resp.StatusCode, string(body), userID)

		// Пытаемся распарсить ошибку валидации для более понятного сообщения
		if resp.StatusCode == http.StatusBadRequest {
			var validationError struct {
				ValidationErrors []struct {
					Errors []struct {
						Code   string `json:"code"`
						Path   string `json:"path"`
						Detail string `json:"detail"`
					} `json:"errors"`
				} `json:"validation-errors"`
			}
			if err := json.Unmarshal(body, &validationError); err == nil && len(validationError.ValidationErrors) > 0 {
				for _, ve := range validationError.ValidationErrors {
					for _, e := range ve.Errors {
						logger.Error("AmoCRM validation error: path=%s, code=%s, detail=%s", e.Path, e.Code, e.Detail, userID)
					}
				}
			}
		}

		return nil, fmt.Errorf("AmoCRM API вернул ошибку (%d): %s", resp.StatusCode, string(body))
	}

	// Парсим ответ
	var result struct {
		Embedded struct {
			Contacts []struct {
				ID        int    `json:"id"`
				Name      string `json:"name"`
				FirstName string `json:"first_name"`
				LastName  string `json:"last_name"`
			} `json:"contacts"`
		} `json:"_embedded"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("ошибка парсинга ответа: %w", err)
	}

	if len(result.Embedded.Contacts) == 0 {
		return nil, fmt.Errorf("контакт не создан: пустой ответ")
	}

	// Возвращаем созданный контакт
	created := result.Embedded.Contacts[0]
	contact.ID = fmt.Sprintf("%d", created.ID)

	logger.Info("AmoCRM: контакт успешно создан, ID=%s", contact.ID, userID)
	return contact, nil
}

// UpdateContact обновляет контакт в AmoCRM
// Использует API: PATCH /api/v4/contacts
func (p *AmoCRMProvider) UpdateContact(ctx context.Context, userID uint32, contactID string, contact *models.Contact) (*models.Contact, error) {
	if p.subdomain == "" || p.accessToken == "" {
		return nil, fmt.Errorf("провайдер не инициализирован: отсутствует subdomain или access_token")
	}

	// Подготавливаем структуру запроса
	updateData := map[string]any{
		"id": contactID,
	}

	// Добавляем поля только если они заполнены
	if contact.Name != "" {
		updateData["name"] = contact.Name
	}
	if contact.FirstName != "" {
		updateData["first_name"] = contact.FirstName
	}
	if contact.LastName != "" {
		updateData["last_name"] = contact.LastName
	}

	// Добавляем custom_fields_values для телефона и email
	var customFields []map[string]any

	if contact.Phone != "" {
		customFields = append(customFields, map[string]any{
			"field_code": "PHONE",
			"values": []map[string]any{
				{
					"value":     contact.Phone,
					"enum_code": "WORK",
				},
			},
		})
	}

	if contact.Email != "" {
		customFields = append(customFields, map[string]any{
			"field_code": "EMAIL",
			"values": []map[string]any{
				{
					"value":     contact.Email,
					"enum_code": "WORK",
				},
			},
		})
	}

	// Обработка AltContact: если Phone пустой, но AltContact есть
	// Берем ID кастомного поля из contact.CustomFields[0].ID
	if contact.Phone == "" && contact.AltContact != "" && len(contact.CustomFields) > 0 {
		altContactFieldID := contact.CustomFields[0].ID
		if altContactFieldID > 0 {
			logger.Debug("Обновление контакта с AltContact: %s в поле ID=%d", contact.AltContact, altContactFieldID)
			customFields = append(customFields, map[string]any{
				"field_id": altContactFieldID,
				"values": []map[string]any{
					{
						"value": contact.AltContact,
					},
				},
			})
		} else {
			logger.Warn("AltContact указан, но CustomFields[0].ID пустой", userID)
		}
	}

	// Добавляем пользовательские custom_fields из запроса
	for _, cf := range contact.CustomFields {
		field := map[string]any{}

		// Используем ID если есть, иначе code
		if cf.ID > 0 {
			field["field_id"] = cf.ID
		} else if cf.Code != "" {
			field["field_code"] = cf.Code
		} else {
			continue // Пропускаем поле, если нет ни ID ни Code
		}

		// Конвертируем значения
		values := make([]map[string]any, 0, len(cf.Values))
		for _, v := range cf.Values {
			val := map[string]any{
				"value": v.Value,
			}
			if v.EnumID > 0 {
				val["enum_id"] = v.EnumID
			}
			values = append(values, val)
		}

		if len(values) > 0 {
			field["values"] = values
			customFields = append(customFields, field)
		}
	}

	if len(customFields) > 0 {
		updateData["custom_fields_values"] = customFields
	}

	// Добавляем теги если есть
	if len(contact.Tags) > 0 {
		tags := make([]map[string]any, 0, len(contact.Tags))
		for _, tag := range contact.Tags {
			tags = append(tags, map[string]any{
				"name": tag,
			})
		}
		updateData["_embedded"] = map[string]any{
			"tags": tags,
		}
	}

	requestBody := []map[string]any{updateData}

	// Сериализуем в JSON
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("ошибка сериализации запроса: %w", err)
	}

	// Логируем тело запроса для отладки
	logger.Debug("AmoCRM UpdateContact request body: %s", string(jsonData), userID)

	// Формируем URL
	apiURL := fmt.Sprintf("https://%s.amocrm.ru/api/v4/contacts", p.subdomain)

	// Создаем HTTP запрос
	req, err := http.NewRequestWithContext(ctx, "PATCH", apiURL, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.accessToken)
	req.Header.Set("Content-Type", "application/json")

	// Выполняем запрос с механизмом повторных попыток
	resp, err := p.DoRequestWithRetry(ctx, req)
	if err != nil {
		logger.Error("Ошибка при выполнении HTTP запроса к AmoCRM API (URL: %s): %v", apiURL, err, userID)
		return nil, fmt.Errorf("ошибка выполнения запроса к AmoCRM API (проверьте интернет-соединение и subdomain '%s'): %w", p.subdomain, err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Читаем ответ
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа: %w", err)
	}

	// Проверяем статус
	if resp.StatusCode != http.StatusOK {
		logger.Error("AmoCRM API вернул ошибку (%d): %s", resp.StatusCode, string(body), userID)

		// Пытаемся распарсить ошибку валидации для более понятного сообщения
		if resp.StatusCode == http.StatusBadRequest {
			var validationError struct {
				ValidationErrors []struct {
					Errors []struct {
						Code   string `json:"code"`
						Path   string `json:"path"`
						Detail string `json:"detail"`
					} `json:"errors"`
				} `json:"validation-errors"`
			}
			if err := json.Unmarshal(body, &validationError); err == nil && len(validationError.ValidationErrors) > 0 {
				for _, ve := range validationError.ValidationErrors {
					for _, e := range ve.Errors {
						logger.Error("AmoCRM validation error: path=%s, code=%s, detail=%s", e.Path, e.Code, e.Detail, userID)
					}
				}
			}
		}

		return nil, fmt.Errorf("AmoCRM API вернул ошибку (%d): %s", resp.StatusCode, string(body))
	}

	// Парсим ответ
	var result struct {
		Embedded struct {
			Contacts []struct {
				ID        int    `json:"id"`
				Name      string `json:"name"`
				FirstName string `json:"first_name"`
				LastName  string `json:"last_name"`
			} `json:"contacts"`
		} `json:"_embedded"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("ошибка парсинга ответа: %w", err)
	}

	if len(result.Embedded.Contacts) == 0 {
		return nil, fmt.Errorf("контакт не обновлен: пустой ответ")
	}

	// Возвращаем обновленный контакт
	updated := result.Embedded.Contacts[0]
	contact.ID = fmt.Sprintf("%d", updated.ID)

	logger.Info("AmoCRM: контакт успешно обновлен, ID=%s", contactID, userID)
	return contact, nil
}

// GetContact получает контакт из AmoCRM по ID
// Использует API: GET /api/v4/contacts/{id}
func (p *AmoCRMProvider) GetContact(ctx context.Context, userID uint32, contactID string) (*models.Contact, error) {
	if p.subdomain == "" || p.accessToken == "" {
		return nil, fmt.Errorf("провайдер не инициализирован: отсутствует subdomain или access_token")
	}

	// Формируем URL
	apiURL := fmt.Sprintf("https://%s.amocrm.ru/api/v4/contacts/%s", p.subdomain, contactID)

	// Создаем HTTP запрос
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.accessToken)
	req.Header.Set("Content-Type", "application/json")

	// Выполняем запрос с механизмом повторных попыток
	resp, err := p.DoRequestWithRetry(ctx, req)
	if err != nil {
		logger.Error("Ошибка при выполнении HTTP запроса к AmoCRM API (URL: %s): %v", apiURL, err, userID)
		return nil, fmt.Errorf("ошибка выполнения запроса к AmoCRM API (проверьте интернет-соединение и subdomain '%s'): %w", p.subdomain, err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Читаем ответ
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа: %w", err)
	}

	// Проверяем статус
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("контакт с ID %s не найден", contactID)
	}
	if resp.StatusCode != http.StatusOK {
		logger.Error("AmoCRM API вернул ошибку (%d): %s", resp.StatusCode, string(body), userID)
		return nil, fmt.Errorf("AmoCRM API вернул ошибку (%d): %s", resp.StatusCode, string(body))
	}

	// Парсим ответ
	var result struct {
		ID             int    `json:"id"`
		Name           string `json:"name"`
		FirstName      string `json:"first_name"`
		LastName       string `json:"last_name"`
		ResponsibleUID int    `json:"responsible_user_id"`
		GroupID        int    `json:"group_id"`
		CreatedAt      int64  `json:"created_at"`
		UpdatedAt      int64  `json:"updated_at"`
		CustomFields   []struct {
			ID     int    `json:"id"`
			Name   string `json:"name"`
			Values []struct {
				Value string `json:"value"`
				Enum  string `json:"enum"`
			} `json:"values"`
		} `json:"custom_fields_values"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("ошибка парсинга ответа: %w", err)
	}

	// Извлекаем email и телефон из custom_fields
	var email, contactPhone string
	for _, field := range result.CustomFields {
		if len(field.Values) > 0 {
			switch field.Name {
			case "Email", "EMAIL":
				email = field.Values[0].Value
			case "Phone", "PHONE", "Телефон":
				contactPhone = field.Values[0].Value
			}
		}
	}

	// Формируем результат
	contact := &models.Contact{
		ID:        fmt.Sprintf("%d", result.ID),
		Name:      result.Name,
		FirstName: result.FirstName,
		LastName:  result.LastName,
		Email:     email,
		Phone:     contactPhone,
	}

	logger.Debug("AmoCRM: контакт получен, ID=%s", contactID, userID)
	return contact, nil
}

// FindContactByPhone ищет контакт по номеру телефона в AmoCRM
// Использует API: GET /api/v4/contacts?query={phone}
func (p *AmoCRMProvider) FindContactByPhone(ctx context.Context, userID uint32, phoneNumber string) (*models.Contact, error) {
	// Проверяем, что провайдер инициализирован (есть subdomain и accessToken)
	if p.subdomain == "" || p.accessToken == "" {
		return nil, fmt.Errorf("провайдер не инициализирован: отсутствует subdomain или access_token")
	}

	// Нормализуем номер телефона
	normalizedPhone := phone.Normalize(phoneNumber)
	if !phone.IsValid(normalizedPhone) {
		return nil, fmt.Errorf("некорректный номер телефона: %s", phoneNumber)
	}

	// Формируем URL запроса
	apiURL := fmt.Sprintf("https://%s.amocrm.ru/api/v4/contacts", p.subdomain)
	reqURL, err := url.Parse(apiURL)
	if err != nil {
		return nil, fmt.Errorf("ошибка формирования URL: %w", err)
	}

	// Добавляем параметры запроса
	query := reqURL.Query()
	query.Set("query", normalizedPhone)
	reqURL.RawQuery = query.Encode()

	// Создаем HTTP запрос
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса: %w", err)
	}

	// Устанавливаем заголовки
	req.Header.Set("Authorization", "Bearer "+p.accessToken)
	req.Header.Set("Content-Type", "application/json")

	// Выполняем запрос с механизмом повторных попыток
	resp, err := p.DoRequestWithRetry(ctx, req)
	if err != nil {
		logger.Error("Ошибка при выполнении HTTP запроса к AmoCRM API (URL: %s): %v", reqURL.String(), err, userID)
		return nil, fmt.Errorf("ошибка выполнения запроса к AmoCRM API (проверьте интернет-соединение и subdomain '%s'): %w", p.subdomain, err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Читаем тело ответа
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа: %w", err)
	}

	// Проверяем статус ответа
	if resp.StatusCode != http.StatusOK {
		logger.Error("AmoCRM API вернул ошибку (%d): %s", resp.StatusCode, string(body), userID)
		return nil, fmt.Errorf("AmoCRM API вернул ошибку (%d): %s", resp.StatusCode, string(body))
	}

	// Парсим ответ
	var result struct {
		Embedded struct {
			Contacts []struct {
				ID             int    `json:"id"`
				Name           string `json:"name"`
				FirstName      string `json:"first_name"`
				LastName       string `json:"last_name"`
				ResponsibleUID int    `json:"responsible_user_id"`
				GroupID        int    `json:"group_id"`
				CreatedAt      int64  `json:"created_at"`
				UpdatedAt      int64  `json:"updated_at"`
				CustomFields   []struct {
					ID     int    `json:"id"`
					Name   string `json:"name"`
					Values []struct {
						Value string `json:"value"`
						Enum  string `json:"enum"`
					} `json:"values"`
				} `json:"custom_fields_values"`
			} `json:"contacts"`
		} `json:"_embedded"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("ошибка парсинга ответа: %w", err)
	}

	// Проверяем, что контакты найдены
	if len(result.Embedded.Contacts) == 0 {
		return nil, fmt.Errorf("контакт с телефоном %s не найден", phoneNumber)
	}

	// Берем первый найденный контакт
	contact := result.Embedded.Contacts[0]

	// Извлекаем email и телефон из custom_fields
	var email, contactPhone string
	for _, field := range contact.CustomFields {
		if len(field.Values) > 0 {
			switch field.Name {
			case "Email", "EMAIL":
				email = field.Values[0].Value
			case "Phone", "PHONE", "Телефон":
				contactPhone = field.Values[0].Value
			}
		}
	}

	// Формируем результат
	return &models.Contact{
		ID:        fmt.Sprintf("%d", contact.ID),
		Name:      contact.Name,
		FirstName: contact.FirstName,
		LastName:  contact.LastName,
		Email:     email,
		Phone:     contactPhone,
	}, nil
}

// FindContactByAltContact ищет контакт по альтернативному контакту (например, UserID Telegram, VK ID и т.д.)
// Использует API: GET /api/v4/contacts?query={alt_contact}
func (p *AmoCRMProvider) FindContactByAltContact(ctx context.Context, userID uint32, altContact string) (*models.Contact, error) {
	// Проверяем, что провайдер инициализирован (есть subdomain и accessToken)
	if p.subdomain == "" || p.accessToken == "" {
		return nil, fmt.Errorf("провайдер не инициализирован: отсутствует subdomain или access_token")
	}

	if altContact == "" {
		return nil, fmt.Errorf("altContact не может быть пустым")
	}

	// Формируем URL запроса
	apiURL := fmt.Sprintf("https://%s.amocrm.ru/api/v4/contacts", p.subdomain)
	reqURL, err := url.Parse(apiURL)
	if err != nil {
		return nil, fmt.Errorf("ошибка формирования URL: %w", err)
	}

	// Добавляем параметры запроса
	query := reqURL.Query()
	query.Set("query", altContact)
	reqURL.RawQuery = query.Encode()

	// Создаем HTTP запрос
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса: %w", err)
	}

	// Устанавливаем заголовки
	req.Header.Set("Authorization", "Bearer "+p.accessToken)
	req.Header.Set("Content-Type", "application/json")

	// Выполняем запрос с механизмом повторных попыток
	resp, err := p.DoRequestWithRetry(ctx, req)
	if err != nil {
		logger.Error("Ошибка при выполнении HTTP запроса к AmoCRM API (URL: %s): %v", reqURL.String(), err, userID)
		return nil, fmt.Errorf("ошибка выполнения запроса к AmoCRM API (проверьте интернет-соединение и subdomain '%s'): %w", p.subdomain, err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Читаем тело ответа
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа: %w", err)
	}

	// Проверяем статус ответа
	if resp.StatusCode != http.StatusOK {
		logger.Error("AmoCRM API вернул ошибку (%d): %s", resp.StatusCode, string(body), userID)
		return nil, fmt.Errorf("AmoCRM API вернул ошибку (%d): %s", resp.StatusCode, string(body))
	}

	// Парсим ответ
	var result struct {
		Embedded struct {
			Contacts []struct {
				ID             int    `json:"id"`
				Name           string `json:"name"`
				FirstName      string `json:"first_name"`
				LastName       string `json:"last_name"`
				ResponsibleUID int    `json:"responsible_user_id"`
				GroupID        int    `json:"group_id"`
				CreatedAt      int64  `json:"created_at"`
				UpdatedAt      int64  `json:"updated_at"`
				CustomFields   []struct {
					ID     int    `json:"id"`
					Name   string `json:"name"`
					Code   string `json:"code"`
					Values []struct {
						Value string `json:"value"`
						Enum  string `json:"enum"`
					} `json:"values"`
				} `json:"custom_fields_values"`
			} `json:"contacts"`
		} `json:"_embedded"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("ошибка парсинга ответа: %w", err)
	}

	// Проверяем, что контакты найдены
	if len(result.Embedded.Contacts) == 0 {
		return nil, fmt.Errorf("контакт с альтернативным контактом %s не найден", altContact)
	}

	// Берем первый найденный контакт
	contact := result.Embedded.Contacts[0]

	// Извлекаем email, телефон и alt_contact из custom_fields
	var email, contactPhone, altContactValue string
	for _, field := range contact.CustomFields {
		if len(field.Values) > 0 {
			switch field.Name {
			case "Email", "EMAIL":
				email = field.Values[0].Value
			case "Phone", "PHONE", "Телефон":
				contactPhone = field.Values[0].Value
			}
			// Ищем поле с альтернативным контактом по коду или имени
			if field.Code != "" && (field.Code == "ALT_CONTACT" || field.Code == "alt_contact") {
				altContactValue = field.Values[0].Value
			}
		}
	}

	// Формируем результат
	return &models.Contact{
		ID:         fmt.Sprintf("%d", contact.ID),
		Name:       contact.Name,
		FirstName:  contact.FirstName,
		LastName:   contact.LastName,
		Email:      email,
		Phone:      contactPhone,
		AltContact: altContactValue,
	}, nil
}

// SetMarusiaSource устанавливает источник "MarusiaAI" в пользовательское поле "Источник перехода"
func (p *AmoCRMProvider) SetMarusiaSource(ctx context.Context, userID uint32, contactID string) error {
	if p.subdomain == "" || p.accessToken == "" {
		return fmt.Errorf("провайдер не инициализирован: отсутствует subdomain или access_token")
	}

	// Проверяем, что sourceFieldID установлен в конфигурации
	if p.sourceFieldID == 0 {
		logger.Warn("source_field_id не установлен в конфигурации. Установите его через POST /configs/amocrm/marusia-source-field", userID)
		return fmt.Errorf("source_field_id не установлен в конфигурации (используйте POST /configs/amocrm/marusia-source-field для его установки)")
	}

	logger.Debug("Используется source_field_id=%d для автоматического заполнения источника MarusiaAI", p.sourceFieldID, userID)

	// Преобразуем contactID из строки в int для AmoCRM API
	var contactIDInt int64
	if _, err := fmt.Sscanf(contactID, "%d", &contactIDInt); err != nil {
		return fmt.Errorf("некорректный contactID: %w", err)
	}

	// Обновляем контакт с заполненным полем "Источник перехода"
	updateBody := []map[string]any{
		{
			"id": contactIDInt, // Используем int вместо string
			"custom_fields_values": []map[string]any{
				{
					"field_id": p.sourceFieldID,
					"values": []map[string]any{
						{
							"value": "MarusiaAI",
						},
					},
				},
			},
		},
	}

	// Сериализуем в JSON
	jsonData, err := json.Marshal(updateBody)
	if err != nil {
		return fmt.Errorf("ошибка сериализации запроса: %w", err)
	}

	logger.Debug("AmoCRM SetMarusiaSource request body: %s", string(jsonData), userID)

	// Формируем URL для обновления контакта
	apiURL := fmt.Sprintf("https://%s.amocrm.ru/api/v4/contacts", p.subdomain)

	// Создаем HTTP запрос
	req, err := http.NewRequestWithContext(ctx, "PATCH", apiURL, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("ошибка создания запроса: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.accessToken)
	req.Header.Set("Content-Type", "application/json")

	// Выполняем запрос
	resp, err := p.DoRequestWithRetry(ctx, req)
	if err != nil {
		logger.Error("Ошибка при добавлении источника MarusiaAI для контакта %s: %v", contactID, err, userID)
		return fmt.Errorf("ошибка выполнения запроса: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Читаем ответ
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("ошибка чтения ответа: %w", err)
	}

	// Логируем ответ для отладки
	logger.Debug("AmoCRM SetMarusiaSource response (status=%d): %s", resp.StatusCode, string(body), userID)

	// Проверяем статус
	if resp.StatusCode != http.StatusOK {
		logger.Error("AmoCRM API вернул ошибку при добавлении источника (%d): %s", resp.StatusCode, string(body), userID)
		return fmt.Errorf("AmoCRM API вернул ошибку (%d): %s", resp.StatusCode, string(body))
	}

	logger.Info("Источник MarusiaAI успешно добавлен в поле 'Источник перехода' для контакта ID=%s (field_id=%d)", contactID, p.sourceFieldID, userID)
	return nil
}

// GetCustomFields получает список пользовательских полей контактов из AmoCRM
func (p *AmoCRMProvider) GetCustomFields(ctx context.Context, userID uint32, entityType string) (*models.CustomFieldsMetadataResponse, error) {
	if p.subdomain == "" || p.accessToken == "" {
		return nil, fmt.Errorf("провайдер не инициализирован: отсутствует subdomain или access_token")
	}

	// Формируем URL для получения полей
	apiURL := fmt.Sprintf("https://%s.amocrm.ru/api/v4/%s/custom_fields", p.subdomain, entityType)

	// Создаем HTTP запрос
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.accessToken)
	req.Header.Set("Content-Type", "application/json")

	// Выполняем запрос
	resp, err := p.DoRequestWithRetry(ctx, req)
	if err != nil {
		logger.Error("Ошибка при получении custom fields из AmoCRM (URL: %s): %v", apiURL, err, userID)
		return nil, fmt.Errorf("ошибка выполнения запроса: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Читаем ответ
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа: %w", err)
	}

	// Проверяем статус
	if resp.StatusCode != http.StatusOK {
		logger.Error("AmoCRM API вернул ошибку при получении custom fields (%d): %s", resp.StatusCode, string(body), userID)
		return nil, fmt.Errorf("AmoCRM API вернул ошибку (%d): %s", resp.StatusCode, string(body))
	}

	// Парсим ответ AmoCRM (формат с _embedded)
	var amoCRMResponse struct {
		Embedded struct {
			CustomFields []models.CustomFieldMetadata `json:"custom_fields"`
		} `json:"_embedded"`
	}

	if err := json.Unmarshal(body, &amoCRMResponse); err != nil {
		return nil, fmt.Errorf("ошибка парсинга ответа: %w", err)
	}

	// Преобразуем в универсальную структуру
	result := &models.CustomFieldsMetadataResponse{
		Fields: amoCRMResponse.Embedded.CustomFields,
	}

	logger.Debug("AmoCRM: получено %d пользовательских полей для %s", len(result.Fields), entityType, userID)
	return result, nil
}

// GetContactCustomFields получает список пользовательских полей контактов из AmoCRM
// Использует API: GET /api/v4/contacts/custom_fields
func (p *AmoCRMProvider) GetContactCustomFields(ctx context.Context) (any, error) {
	if p.subdomain == "" || p.accessToken == "" {
		return nil, fmt.Errorf("провайдер не инициализирован: отсутствует subdomain или access_token")
	}

	apiURL := fmt.Sprintf("https://%s.amocrm.ru/api/v4/contacts/custom_fields", p.subdomain)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания HTTP запроса: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.DoRequestWithRetry(ctx, req)
	if err != nil {
		logger.Error("Ошибка при получении custom fields контактов из AmoCRM (URL: %s): %v", apiURL, err)
		return nil, fmt.Errorf("ошибка выполнения запроса: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		logger.Error("AmoCRM API вернул ошибку при получении custom fields контактов (%d): %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("AmoCRM API вернул ошибку (%d): %s", resp.StatusCode, string(body))
	}

	// Возвращаем сырой JSON как any для гибкости
	var result any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("ошибка парсинга ответа: %w", err)
	}

	logger.Debug("AmoCRM: получены пользовательские поля контактов")
	return result, nil
}

// CreateCustomField создает новое пользовательское поле в AmoCRM
// POST /api/v4/{entityType}/custom_fields
func (p *AmoCRMProvider) CreateCustomField(ctx context.Context, entityType string, fieldData any) (any, error) {
	if p.subdomain == "" || p.accessToken == "" {
		return nil, fmt.Errorf("провайдер не инициализирован: отсутствует subdomain или access_token")
	}

	// Формируем URL для создания поля
	apiURL := fmt.Sprintf("https://%s.amocrm.ru/api/v4/%s/custom_fields", p.subdomain, entityType)

	// AmoCRM API требует массив объектов, а не один объект
	// Оборачиваем данные в массив, если это ещё не массив
	var requestData any
	switch v := fieldData.(type) {
	case []any:
		requestData = v
	default:
		requestData = []any{fieldData}
	}

	// Маршалим данные поля в JSON
	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("ошибка маршалинга данных поля: %w", err)
	}

	logger.Debug("AmoCRM: отправка данных для создания custom field: %s", string(jsonData))

	// Создаем HTTP запрос
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.accessToken)
	req.Header.Set("Content-Type", "application/json")

	// Выполняем запрос
	resp, err := p.DoRequestWithRetry(ctx, req)
	if err != nil {
		logger.Error("Ошибка при создании custom field в AmoCRM (URL: %s): %v", apiURL, err)
		return nil, fmt.Errorf("ошибка выполнения запроса: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Читаем ответ
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа: %w", err)
	}

	// Проверяем статус
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		logger.Error("AmoCRM API вернул ошибку при создании custom field (%d): %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("AmoCRM API вернул ошибку (%d): %s", resp.StatusCode, string(body))
	}

	// Парсим ответ
	var result any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("ошибка парсинга ответа: %w", err)
	}

	logger.Debug("AmoCRM: создано пользовательское поле для %s", entityType)
	return result, nil
}
