package handlers

import (
	"Marusia_CRM/internal/domain/models"
	"Marusia_CRM/internal/domain/service"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/ikermy/air_logger/v2/pkg/logger"
)

// ConfigHandler обрабатывает запросы по управлению CRM конфигурациями
type ConfigHandler struct {
	service *service.CRMService
}

// NewConfigHandler создает новый handler
func NewConfigHandler(service *service.CRMService) *ConfigHandler {
	return &ConfigHandler{service: service}
}

// GetCRMConfigs получает список всех CRM конфигураций пользователя
// GET /crm/configs
func (h *ConfigHandler) GetCRMConfigs(c *fiber.Ctx) error {
	userID, ok := GetUserID(c)
	if !ok {
		observeEntityMetric("config", "list", http.StatusBadRequest, "missing_user_id")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "user_id не найден"})
	}

	configs, err := h.service.GetUserCRMConfigs(userID)
	if err != nil {
		observeEntityMetric("config", "list", http.StatusInternalServerError, "service_error")
		logger.Error("Ошибка получения CRM конфигов: %v", err, userID)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "ошибка получения конфигураций", "details": err.Error()})
	}

	observeEntityMetric("config", "list", http.StatusOK, "")
	// Возвращаем полные конфигурации с credentials
	return c.Status(http.StatusOK).JSON(fiber.Map{"configs": configs})
}

// SaveCRMConfig сохраняет (создает или обновляет) CRM конфигурацию
// POST /crm/configs/:crm_type
func (h *ConfigHandler) SaveCRMConfig(c *fiber.Ctx) error {
	crmType := c.Params("crm_type")
	if crmType == "" {
		observeEntityMetric("config", "save", http.StatusBadRequest, "missing_crm_type")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "некорректный crm_type"})
	}

	// Структура для приема запроса с credentials как объектом
	var req struct {
		Name        string                `json:"name" binding:"required"`
		Subdomain   string                `json:"subdomain"`
		Credentials models.CRMCredentials `json:"credentials" binding:"required"`
		IsActive    bool                  `json:"is_active"`
	}

	if err := c.BodyParser(&req); err != nil {
		observeEntityMetric("config", "save", http.StatusBadRequest, "invalid_body")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "некорректный запрос", "details": err.Error()})
	}

	// Извлекаем userID из контекста
	userID, ok := GetUserID(c)
	if !ok {
		observeEntityMetric("config", "save", http.StatusBadRequest, "missing_user_id")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "user_id не найден"})
	}

	// Маршалим credentials в JSON строку для сохранения в БД
	credentialsJSON, err := json.Marshal(req.Credentials)
	if err != nil {
		observeEntityMetric("config", "save", http.StatusInternalServerError, "marshal_error")
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "ошибка сохранения credentials"})
	}

	// Создаем модель для сохранения в БД
	config := &models.CRMConfig{
		UserID:      userID,
		CRMType:     crmType,
		Name:        req.Name,
		Subdomain:   req.Subdomain,
		Credentials: string(credentialsJSON),
		IsActive:    req.IsActive,
	}

	// Используем Upsert (создание или обновление)
	if err := h.service.UpsertCRMConfig(config); err != nil {
		observeEntityMetric("config", "save", http.StatusInternalServerError, "service_error")
		logger.Error("Ошибка сохранения CRM конфига: %v", err, userID)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "ошибка сохранения конфигурации", "details": err.Error()})
	}

	observeEntityMetric("config", "save", http.StatusOK, "")
	// Возвращаем сохраненную конфигурацию
	return c.Status(http.StatusOK).JSON(fiber.Map{
		"message": "конфигурация сохранена",
		"config":  config,
	})
}

// GetCRMConfigByType получает CRM конфигурацию по crm_type
// GET /crm/configs/:crm_type
func (h *ConfigHandler) GetCRMConfigByType(c *fiber.Ctx) error {
	crmType := c.Params("crm_type")
	if crmType == "" {
		observeEntityMetric("config", "get", http.StatusBadRequest, "missing_crm_type")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "некорректный crm_type"})
	}

	userID, ok := GetUserID(c)
	if !ok {
		observeEntityMetric("config", "get", http.StatusBadRequest, "missing_user_id")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "user_id не найден"})
	}

	config, err := h.service.GetCRMConfigByType(userID, crmType)
	if err != nil {
		observeEntityMetric("config", "get", http.StatusNotFound, "not_found")
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "конфигурация не найдена"})
	}

	observeEntityMetric("config", "get", http.StatusOK, "")
	// Возвращаем полную конфигурацию с credentials
	return c.Status(http.StatusOK).JSON(fiber.Map{
		"crm_type":    config.CRMType,
		"name":        config.Name,
		"subdomain":   config.Subdomain,
		"credentials": config.Credentials,
		"is_active":   config.IsActive,
		"created_at":  config.CreatedAt,
		"updated_at":  config.UpdatedAt,
	})
}

// DeleteCRMConfigByType удаляет CRM конфигурацию по crm_type
// DELETE /crm/configs/:crm_type
func (h *ConfigHandler) DeleteCRMConfigByType(c *fiber.Ctx) error {
	crmType := c.Params("crm_type")
	if crmType == "" {
		observeEntityMetric("config", "delete", http.StatusBadRequest, "missing_crm_type")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "некорректный crm_type"})
	}

	userID, ok := GetUserID(c)
	if !ok {
		observeEntityMetric("config", "delete", http.StatusBadRequest, "missing_user_id")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "user_id не найден"})
	}

	if err := h.service.DeleteCRMConfigByType(userID, crmType); err != nil {
		observeEntityMetric("config", "delete", http.StatusInternalServerError, "service_error")
		logger.Error("Ошибка удаления CRM конфига: %v", err, userID)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "ошибка удаления конфигурации", "details": err.Error()})
	}

	observeEntityMetric("config", "delete", http.StatusOK, "")
	return c.Status(http.StatusOK).JSON(fiber.Map{"message": "конфигурация удалена"})
}

// SetMarusiaSourceFieldIDRequest структура запроса для установки field ID
type SetMarusiaSourceFieldIDRequest struct {
	FieldID int64 `json:"field_id" binding:"required"`
}

// SetMarusiaSourceFieldID устанавливает ID поля "Источник перехода" для автоматического использования
// POST /configs/:crm_type/marusia-source-field
func (h *ConfigHandler) SetMarusiaSourceFieldID(c *fiber.Ctx) error {
	// Получаем userID из контекста
	userID, ok := GetUserID(c)
	if !ok {
		observeEntityMetric("config", "set_source_field", http.StatusUnauthorized, "missing_user_id")
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "user_id не найден"})
	}

	// Получаем crm_type из URL
	crmType := c.Params("crm_type")
	if crmType == "" {
		observeEntityMetric("config", "set_source_field", http.StatusBadRequest, "missing_crm_type")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "crm_type обязателен"})
	}

	// Парсим тело запроса
	var req SetMarusiaSourceFieldIDRequest
	if err := c.BodyParser(&req); err != nil {
		observeEntityMetric("config", "set_source_field", http.StatusBadRequest, "invalid_body")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "некорректный запрос", "details": err.Error()})
	}

	logger.Debug("Установка field_id источника MarusiaAI: userID=%d, crmType=%s, fieldID=%d", userID, crmType, req.FieldID, userID)

	// Устанавливаем field ID через сервис
	err := h.service.SetMarusiaSourceFieldID(userID, crmType, req.FieldID)
	if err != nil {
		observeEntityMetric("config", "set_source_field", http.StatusInternalServerError, "service_error")
		logger.Error("Ошибка установки field_id источника: %v", err, userID)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "ошибка установки field_id", "details": err.Error()})
	}

	logger.Info("Field ID источника MarusiaAI установлен: userID=%d, crmType=%s, fieldID=%d", userID, crmType, req.FieldID)

	observeEntityMetric("config", "set_source_field", http.StatusOK, "")
	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success":  true,
		"message":  "ID поля источника перехода успешно сохранен",
		"field_id": req.FieldID,
	})
}

// GetCustomFields получает список всех пользовательских полей для указанного типа сущности
// GET /configs/:crm_type/custom-fields?entity_type=contacts|leads|companies
func (h *ConfigHandler) GetCustomFields(c *fiber.Ctx) error {
	// Получаем userID из контекста
	userID, ok := GetUserID(c)
	if !ok {
		observeEntityMetric("config", "list_custom_fields", http.StatusUnauthorized, "missing_user_id")
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "user_id не найден"})
	}

	// Получаем crm_type из URL
	crmType := c.Params("crm_type")
	if crmType == "" {
		observeEntityMetric("config", "list_custom_fields", http.StatusBadRequest, "missing_crm_type")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "crm_type обязателен"})
	}

	// Получаем entity_type из query параметра (по умолчанию "contacts")
	entityType := c.Query("entity_type")
	if entityType == "" {
		entityType = "contacts"
	}

	logger.Debug("Получение custom fields: userID=%d, crmType=%s, entityType=%s", userID, crmType, entityType, userID)

	// Получаем custom fields через сервис
	fields, err := h.service.GetCustomFields(c.Context(), userID, crmType, entityType)
	if err != nil {
		observeEntityMetric("config", "list_custom_fields", http.StatusInternalServerError, "service_error")
		logger.Error("Ошибка получения custom fields: %v", err, userID)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "ошибка получения custom fields", "details": err.Error()})
	}

	observeEntityMetric("config", "list_custom_fields", http.StatusOK, "")
	return c.Status(http.StatusOK).JSON(fiber.Map{
		"custom_fields": fields,
		"success":       true,
		"entity_type":   entityType,
	})
}

// CreateCustomField создает новое пользовательское поле
// POST /configs/:crm_type/custom-fields?entity_type=contacts|leads|companies
func (h *ConfigHandler) CreateCustomField(c *fiber.Ctx) error {
	// Получаем userID из контекста
	userID, ok := GetUserID(c)
	if !ok {
		observeEntityMetric("config", "create_custom_field", http.StatusUnauthorized, "missing_user_id")
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "user_id не найден"})
	}

	// Получаем crm_type из URL
	crmType := c.Params("crm_type")
	if crmType == "" {
		observeEntityMetric("config", "create_custom_field", http.StatusBadRequest, "missing_crm_type")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "crm_type обязателен"})
	}

	// Получаем entity_type из query параметра (по умолчанию "contacts")
	entityType := c.Query("entity_type")
	if entityType == "" {
		entityType = "contacts"
	}

	// Парсим тело запроса
	var req CreateCustomFieldRequest
	if err := c.BodyParser(&req); err != nil {
		observeEntityMetric("config", "create_custom_field", http.StatusBadRequest, "invalid_body")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "некорректный запрос", "details": err.Error()})
	}

	logger.Debug("Создание custom field: userID=%d, crmType=%s, entityType=%s, name=%s, type=%s",
		userID, crmType, entityType, req.Name, req.FieldType)

	// Создаем custom field через сервис
	field, err := h.service.CreateCustomField(c.Context(), userID, crmType, entityType, &req)
	if err != nil {
		observeEntityMetric("config", "create_custom_field", http.StatusInternalServerError, "service_error")
		logger.Error("Ошибка создания custom field: %v", err, userID)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "ошибка создания custom field", "details": err.Error()})
	}

	logger.Info("Custom field создан: userID=%d, crmType=%s, entityType=%s, name=%s",
		userID, crmType, entityType, req.Name)

	observeEntityMetric("config", "create_custom_field", http.StatusCreated, "")
	return c.Status(http.StatusCreated).JSON(fiber.Map{
		"custom_field": field,
		"success":      true,
		"message":      "пользовательское поле успешно создано",
		"entity_type":  entityType,
	})
}

// SetDefaultLeadSettingsRequest структура запроса для установки настроек лида по умолчанию
type SetDefaultLeadSettingsRequest struct {
	PipelineID int64 `json:"pipeline_id" binding:"required"`
	StatusID   int64 `json:"status_id" binding:"required"`
}

// SetDefaultLeadSettings устанавливает pipeline_id и status_id по умолчанию для UpdateLead
// POST /configs/:crm_type/default-lead-settings
func (h *ConfigHandler) SetDefaultLeadSettings(c *fiber.Ctx) error {
	userID, ok := GetUserID(c)
	if !ok {
		observeEntityMetric("config", "set_default_lead_settings", http.StatusUnauthorized, "missing_user_id")
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "user_id не найден"})
	}

	crmType := c.Params("crm_type")
	if crmType == "" {
		observeEntityMetric("config", "set_default_lead_settings", http.StatusBadRequest, "missing_crm_type")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "crm_type обязателен"})
	}

	var req SetDefaultLeadSettingsRequest
	if err := c.BodyParser(&req); err != nil {
		observeEntityMetric("config", "set_default_lead_settings", http.StatusBadRequest, "invalid_body")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "некорректный запрос", "details": err.Error()})
	}

	logger.Debug("Установка default lead settings: userID=%d, crmType=%s, pipelineID=%d, statusID=%d",
		userID, crmType, req.PipelineID, req.StatusID)

	err := h.service.SetDefaultLeadSettings(userID, crmType, req.PipelineID, req.StatusID)
	if err != nil {
		observeEntityMetric("config", "set_default_lead_settings", http.StatusInternalServerError, "service_error")
		logger.Error("Ошибка установки default lead settings: %v", err, userID)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "ошибка установки настроек лида", "details": err.Error()})
	}

	logger.Info("Настройки лида по умолчанию установлены: userID=%d, crmType=%s, pipelineID=%d, statusID=%d",
		userID, crmType, req.PipelineID, req.StatusID)

	observeEntityMetric("config", "set_default_lead_settings", http.StatusOK, "")
	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "настройки лида по умолчанию успешно установлены",
	})
}

// GetChannelSettings получает настройки каналов из поля chanells
// GET /configs/:crm_type/channels
func (h *ConfigHandler) GetChannelSettings(c *fiber.Ctx) error {
	userID, ok := GetUserID(c)
	if !ok {
		observeEntityMetric("config", "get_channel_settings", http.StatusUnauthorized, "missing_user_id")
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "user_id не найден"})
	}

	crmType := c.Params("crm_type")
	if crmType == "" {
		observeEntityMetric("config", "get_channel_settings", http.StatusBadRequest, "missing_crm_type")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "crm_type обязателен"})
	}

	logger.Debug("Получение настроек каналов: userID=%d, crmType=%s", userID, crmType)

	// Получаем конфигурацию CRM
	config, err := h.service.GetCRMConfigByType(userID, crmType)
	if err != nil {
		observeEntityMetric("config", "get_channel_settings", http.StatusNotFound, "not_found")
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "конфигурация не найдена"})
	}

	// Проверяем, что конфигурация активна
	if !config.IsActive {
		observeEntityMetric("config", "get_channel_settings", http.StatusForbidden, "inactive_config")
		return c.Status(http.StatusForbidden).JSON(fiber.Map{"error": "конфигурация CRM неактивна"})
	}

	// Парсим channels JSON - ожидаем структуру вида: { "amocrm": { ... }, "bitrix24": { ... } }
	var allChannels map[string]models.CRMChannelSettings
	if config.Chanells != "" {
		if err := json.Unmarshal([]byte(config.Chanells), &allChannels); err != nil {
			observeEntityMetric("config", "get_channel_settings", http.StatusInternalServerError, "parse_error")
			logger.Error("[USER:%d] Не удалось распарсить channels: %v", userID, err)
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
				"error": "ошибка парсинга настроек каналов",
			})
		}
	}

	// Извлекаем настройки для конкретного CRM типа
	channelSettings, exists := allChannels[crmType]
	if !exists {
		observeEntityMetric("config", "get_channel_settings", http.StatusNotFound, "not_found")
		logger.Warn("[USER:%d] Настройки для CRM типа %s не найдены", userID, crmType)
		return c.Status(http.StatusNotFound).JSON(fiber.Map{
			"error": fmt.Sprintf("настройки для CRM типа '%s' не найдены", crmType),
		})
	}

	logger.Debug("Настройки каналов получены: %v", channelSettings, userID)

	observeEntityMetric("config", "get_channel_settings", http.StatusOK, "")
	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success":  true,
		"settings": channelSettings,
	})
}

// SetChannelSettings сохраняет настройки каналов для указанного CRM типа
// POST /configs/:crm_type/channels
func (h *ConfigHandler) SetChannelSettings(c *fiber.Ctx) error {
	userID, ok := GetUserID(c)
	if !ok {
		observeEntityMetric("config", "set_channel_settings", http.StatusUnauthorized, "missing_user_id")
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "user_id не найден"})
	}

	crmType := c.Params("crm_type")
	if crmType == "" {
		observeEntityMetric("config", "set_channel_settings", http.StatusBadRequest, "missing_crm_type")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "crm_type обязателен"})
	}

	// Используем структуру с указателями для частичного обновления
	var req models.UpdateChannelSettingsRequest
	if err := c.BodyParser(&req); err != nil {
		observeEntityMetric("config", "set_channel_settings", http.StatusBadRequest, "invalid_body")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "некорректный запрос", "details": err.Error()})
	}

	logger.Debug("Сохранение настроек каналов: userID=%d, crmType=%s", userID, crmType)

	// Получаем текущую конфигурацию для мерджа
	config, err := h.service.GetCRMConfigByType(userID, crmType)
	if err != nil {
		observeEntityMetric("config", "set_channel_settings", http.StatusNotFound, "not_found")
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "конфигурация не найдена"})
	}

	// Парсим существующие настройки каналов
	var allChannels map[string]models.CRMChannelSettings
	if config.Chanells != "" {
		if err := json.Unmarshal([]byte(config.Chanells), &allChannels); err != nil {
			observeEntityMetric("config", "set_channel_settings", http.StatusInternalServerError, "parse_error")
			logger.Error("[USER:%d] Не удалось распарсить channels: %v", userID, err)
			allChannels = make(map[string]models.CRMChannelSettings)
		}
	} else {
		allChannels = make(map[string]models.CRMChannelSettings)
	}

	// Получаем существующие настройки для данного CRM типа
	existingSettings := allChannels[crmType]

	// Мерджим: обновляем только те поля, которые были переданы (не nil)
	if req.Assist != nil {
		existingSettings.Assist = *req.Assist
	}
	if req.User != nil {
		existingSettings.User = *req.User
	}
	if req.Meta != nil {
		existingSettings.Meta = *req.Meta
	}
	if req.Voice != nil {
		existingSettings.Voice = *req.Voice
	}
	if req.File != nil {
		existingSettings.File = *req.File
	}
	if req.LeadName != nil {
		existingSettings.LeadName = *req.LeadName
	}
	if req.Tags != nil {
		existingSettings.Tags = *req.Tags
	}
	if req.CreateNewContact != nil {
		existingSettings.CreateNewContact = *req.CreateNewContact
	}
	if req.CreateNewLead != nil {
		existingSettings.CreateNewLead = *req.CreateNewLead
	}
	if req.ChatMessages != nil {
		existingSettings.ChatMessages = *req.ChatMessages
	}
	if req.MetaExist != nil {
		existingSettings.MetaExist = *req.MetaExist
	}
	if req.AltContact != nil {
		existingSettings.AltContact = *req.AltContact
	}
	if req.Telegram != nil {
		existingSettings.Telegram = *req.Telegram
	}
	if req.Instagram != nil {
		existingSettings.Instagram = *req.Instagram
	}
	if req.Widget != nil {
		existingSettings.Widget = *req.Widget
	}

	// Вызываем сервис для сохранения объединённых настроек
	err = h.service.SetChannelSettings(userID, crmType, &existingSettings)
	if err != nil {
		observeEntityMetric("config", "set_channel_settings", http.StatusInternalServerError, "service_error")
		logger.Error("Ошибка сохранения настроек каналов: %v", err, userID)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "ошибка сохранения настроек", "details": err.Error()})
	}

	logger.Info("Настройки каналов успешно сохранены: userID=%d, crmType=%s", userID, crmType, userID)

	observeEntityMetric("config", "set_channel_settings", http.StatusOK, "")
	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success":  true,
		"message":  "настройки каналов успешно сохранены",
		"settings": existingSettings,
	})
}

// TestConnectionByType тестирует подключение к CRM по crm_type
// GET /configs/:crm_type/test
func (h *ConfigHandler) TestConnectionByType(c *fiber.Ctx) error {
	crmType := c.Params("crm_type")
	if crmType == "" {
		observeEntityMetric("config", "test_connection", http.StatusBadRequest, "missing_crm_type")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "некорректный crm_type"})
	}

	userID, ok := GetUserID(c)
	if !ok {
		observeEntityMetric("config", "test_connection", http.StatusBadRequest, "missing_user_id")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "user_id не найден"})
	}

	// Для AmoCRM возвращаем подробную информацию об аккаунте
	if crmType == "amocrm" {
		accountInfo, err := h.service.GetAmoCRMAccountInfo(c.Context(), userID, crmType)
		if err != nil {
			observeEntityMetric("config", "test_connection", http.StatusBadRequest, "service_error")
			logger.Error("Ошибка получения информации об аккаунте AmoCRM: %v", err, userID)
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"status": "failed",
				"error":  err.Error(),
			})
		}

		logger.Debug("AccountInfo %-v", accountInfo, userID)

		observeEntityMetric("config", "test_connection", http.StatusOK, "")
		return c.Status(http.StatusOK).JSON(fiber.Map{
			"status":  "success",
			"message": "подключение к AmoCRM успешно",
			"account": fiber.Map{
				"id":         accountInfo.ID,
				"name":       accountInfo.Name,
				"subdomain":  accountInfo.Subdomain,
				"created_at": accountInfo.CreatedAt,
				"country":    accountInfo.Country,
				"currency":   accountInfo.Currency,
			},
		})
	}

	// Для других CRM используем простую проверку
	config, err := h.service.GetCRMConfigByType(userID, crmType)
	if err != nil {
		observeEntityMetric("config", "test_connection", http.StatusNotFound, "not_found")
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "конфигурация не найдена"})
	}

	if err := h.service.TestCRMConnection(c.Context(), userID, config.ID); err != nil {
		observeEntityMetric("config", "test_connection", http.StatusBadRequest, "service_error")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"status": "failed", "error": err.Error()})
	}

	observeEntityMetric("config", "test_connection", http.StatusOK, "")
	return c.Status(http.StatusOK).JSON(fiber.Map{"status": "success", "message": "подключение успешно"})
}

// SetCRMConfigActive устанавливает статус активности конфигурации (is_active)
// POST /crm/configs/:crm_type/active
func (h *ConfigHandler) SetCRMConfigActive(c *fiber.Ctx) error {
	crmType := c.Params("crm_type")
	if crmType == "" {
		observeEntityMetric("config", "set_active", http.StatusBadRequest, "missing_crm_type")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "некорректный crm_type"})
	}

	var req struct {
		IsActive *bool `json:"is_active" binding:"required"`
	}

	if err := c.BodyParser(&req); err != nil {
		observeEntityMetric("config", "set_active", http.StatusBadRequest, "invalid_body")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "некорректный запрос", "details": err.Error()})
	}

	userID, ok := GetUserID(c)
	if !ok {
		observeEntityMetric("config", "set_active", http.StatusBadRequest, "missing_user_id")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "user_id не найден"})
	}

	// Получаем конфигурацию
	config, err := h.service.GetCRMConfigByType(userID, crmType)
	if err != nil {
		observeEntityMetric("config", "set_active", http.StatusNotFound, "not_found")
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "конфигурация не найдена"})
	}

	// Обновляем статус
	config.IsActive = *req.IsActive

	if err := h.service.UpdateCRMConfig(config); err != nil {
		observeEntityMetric("config", "set_active", http.StatusInternalServerError, "service_error")
		logger.Error("Ошибка обновления статуса CRM конфига: %v", err, userID)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "ошибка обновления статуса", "details": err.Error()})
	}

	observeEntityMetric("config", "set_active", http.StatusOK, "")
	return c.Status(http.StatusOK).JSON(fiber.Map{
		"message":   "статус конфигурации обновлен",
		"crm_type":  config.CRMType,
		"is_active": config.IsActive,
	})
}
