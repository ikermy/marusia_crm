package handlers

import (
	"Marusia_CRM/internal/domain/models"
	"Marusia_CRM/internal/domain/service"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/ikermy/air_logger/v2/pkg/logger"
)

// LeadHandler обрабатывает запросы по работе с лидами
type LeadHandler struct {
	service *service.CRMService
}

// NewLeadHandler создает новый handler
func NewLeadHandler(service *service.CRMService) *LeadHandler {
	return &LeadHandler{service: service}
}

// CreateLeadRequest структура запроса на создание лида
type CreateLeadRequest struct {
	UserID   uint32       `json:"user_id"`   // ID пользователя
	AppID    uint32       `json:"app_id"`    // ID приложения (источника)
	LeadData *models.Lead `json:"lead_data"` // Данные лида
}

// CreateLead создает новый лид в CRM
// POST /api/v1/leads
func (h *LeadHandler) CreateLead(c *fiber.Ctx) error {
	var req CreateLeadRequest
	if err := c.BodyParser(&req); err != nil {
		observeEntityMetric("lead", "create", http.StatusBadRequest, "invalid_body")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "некорректный запрос", "details": err.Error()})
	}

	// Извлекаем userID из заголовка (если не передан в теле)
	if req.UserID == 0 {
		userID, ok := GetUserID(c)
		if !ok {
			observeEntityMetric("lead", "create", http.StatusBadRequest, "missing_user_id")
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "user_id не найден"})
		}
		req.UserID = userID
	}

	// Валидация
	if req.AppID == 0 {
		observeEntityMetric("lead", "create", http.StatusBadRequest, "missing_app_id")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "app_id обязателен"})
	}
	if req.LeadData == nil {
		observeEntityMetric("lead", "create", http.StatusBadRequest, "missing_lead_data")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "lead_data обязательны"})
	}

	// Создаем лид
	err := h.service.CreateLeadForUser(c.Context(), req.UserID, req.AppID, req.LeadData)
	if err != nil {
		observeEntityMetric("lead", "create", http.StatusInternalServerError, "service_error")
		logger.Error("Ошибка создания лида для userID=%d, appID=%d: %v", req.UserID, req.AppID, err)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "ошибка создания лида", "details": err.Error()})
	}

	observeEntityMetric("lead", "create", http.StatusCreated, "")
	return c.Status(http.StatusCreated).JSON(fiber.Map{
		"message": "лид успешно создан",
		"user_id": req.UserID,
		"app_id":  req.AppID,
	})
}

// UpdateLead обновляет лид в CRM
// PATCH /leads/:crm_type/:lead_id
func (h *LeadHandler) UpdateLead(c *fiber.Ctx) error {
	userID, ok := GetUserID(c)
	if !ok {
		observeEntityMetric("lead", "update", http.StatusUnauthorized, "missing_user_id")
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "user_id не найден"})
	}

	crmType := c.Params("crm_type")
	leadID := c.Params("lead_id")
	if crmType == "" || leadID == "" {
		observeEntityMetric("lead", "update", http.StatusBadRequest, "missing_params")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "crm_type и lead_id обязательны"})
	}

	logger.Debug("Обновление лида: userID=%d, crmType=%s, leadID=%s", userID, crmType, leadID)

	// Вызываем сервис - pipeline_id и status_id берутся из настроек БД автоматически
	updatedLead, err := h.service.UpdateLead(c.Context(), userID, crmType, leadID)
	if err != nil {
		observeEntityMetric("lead", "update", http.StatusInternalServerError, "service_error")
		logger.Error("Ошибка обновления лида: %v", err, userID)
		if strings.Contains(err.Error(), "не найден") {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "лид не найден", "details": err.Error()})
		}
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "ошибка обновления лида", "details": err.Error()})
	}

	logger.Info("Лид успешно обновлен: ID=%s", leadID, userID)

	observeEntityMetric("lead", "update", http.StatusOK, "")
	return c.Status(http.StatusOK).JSON(fiber.Map{
		"lead":    updatedLead,
		"success": true,
		"message": "лид успешно обновлен",
	})
}

// GetLead получает лид из CRM
// GET /leads/:crm_type/:lead_id
func (h *LeadHandler) GetLead(c *fiber.Ctx) error {
	userID, ok := GetUserID(c)
	if !ok {
		observeEntityMetric("lead", "get", http.StatusUnauthorized, "missing_user_id")
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "user_id не найден"})
	}

	crmType := c.Params("crm_type")
	leadID := c.Params("lead_id")
	if crmType == "" || leadID == "" {
		observeEntityMetric("lead", "get", http.StatusBadRequest, "missing_params")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "crm_type и lead_id обязательны"})
	}

	logger.Debug("Получение лида: userID=%d, crmType=%s, leadID=%s", userID, crmType, leadID)

	lead, err := h.service.GetLead(c.Context(), userID, crmType, leadID)
	if err != nil {
		observeEntityMetric("lead", "get", http.StatusInternalServerError, "service_error")
		logger.Error("Ошибка получения лида: %v", err, userID)
		if strings.Contains(err.Error(), "не найден") {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "лид не найден", "details": err.Error()})
		}
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "ошибка получения лида", "details": err.Error()})
	}

	observeEntityMetric("lead", "get", http.StatusOK, "")
	return c.Status(http.StatusOK).JSON(fiber.Map{
		"lead":    lead,
		"success": true,
	})
}

// GetLeadsByContactID получает все лиды связанные с контактом
// GET /leads/:crm_type/by-contact/:contact_id
func (h *LeadHandler) GetLeadsByContactID(c *fiber.Ctx) error {
	userID, ok := GetUserID(c)
	if !ok {
		observeEntityMetric("lead", "list_by_contact", http.StatusUnauthorized, "missing_user_id")
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "user_id не найден"})
	}

	crmType := c.Params("crm_type")
	contactID := c.Params("contact_id")
	if crmType == "" || contactID == "" {
		observeEntityMetric("lead", "list_by_contact", http.StatusBadRequest, "missing_params")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "crm_type и contact_id обязательны"})
	}

	logger.Debug("Получение лидов для контакта: userID=%d, crmType=%s, contactID=%s", userID, crmType, contactID)

	leads, err := h.service.GetLeadsByContactID(c.Context(), userID, crmType, contactID)
	if err != nil {
		observeEntityMetric("lead", "list_by_contact", http.StatusInternalServerError, "service_error")
		logger.Error("Ошибка получения лидов: %v", err, userID)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "ошибка получения лидов", "details": err.Error()})
	}

	observeEntityMetric("lead", "list_by_contact", http.StatusOK, "")
	return c.Status(http.StatusOK).JSON(fiber.Map{
		"leads":   leads,
		"count":   len(leads),
		"success": true,
	})
}

// CreateAIDialogLead создает лид по contact_id и имени из запроса
// POST /api/v1/leads/create-by-contact
func (h *LeadHandler) CreateAIDialogLead(c *fiber.Ctx) error {
	userID, ok := GetUserID(c)
	if !ok {
		observeEntityMetric("lead", "create_ai_dialog", http.StatusUnauthorized, "missing_user_id")
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "user_id не найден"})
	}

	crmType := c.Params("crm_type")
	contactIDFromURL := c.Params("contact_id")
	if crmType == "" {
		observeEntityMetric("lead", "create_ai_dialog", http.StatusBadRequest, "missing_crm_type")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "crm_type обязателен"})
	}

	var req struct {
		ContactID string   `json:"contact_id"` // ID контакта из CreateContact/GetContact
		LeadName  string   `json:"lead_name"`
		Tags      []string `json:"tags"`
	}

	if err := c.BodyParser(&req); err != nil {
		observeEntityMetric("lead", "create_ai_dialog", http.StatusBadRequest, "invalid_body")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "некорректный запрос", "details": err.Error()})
	}

	// Используем contact_id из body, если указан, иначе из URL
	contactID := req.ContactID
	if contactID == "" {
		contactID = contactIDFromURL
	}

	if contactID == "" {
		observeEntityMetric("lead", "create_ai_dialog", http.StatusBadRequest, "missing_contact_id")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "contact_id обязателен (в URL или в теле запроса)"})
	}

	if strings.TrimSpace(req.LeadName) == "" {
		observeEntityMetric("lead", "create_ai_dialog", http.StatusBadRequest, "missing_lead_name")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "lead_name обязателен"})
	}

	logger.Debug("Создание лида: userID=%d, crmType=%s, contactID=%s, leadName=%s, tags=%v", userID, crmType, contactID, req.LeadName, req.Tags)

	lead, err := h.service.CreateAIDialogLead(c.Context(), userID, crmType, contactID, req.LeadName, req.Tags)
	if err != nil {
		observeEntityMetric("lead", "create_ai_dialog", http.StatusInternalServerError, "service_error")
		logger.Error("Ошибка создания лида: %v", err, userID)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "ошибка создания лида", "details": err.Error()})
	}

	observeEntityMetric("lead", "create_ai_dialog", http.StatusCreated, "")
	return c.Status(http.StatusCreated).JSON(fiber.Map{
		"lead":    lead,
		"success": true,
		"message": "лид успешно создан",
	})
}
