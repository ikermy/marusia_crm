package handlers

import (
	"Marusia_CRM/internal/domain/models"
	"Marusia_CRM/internal/domain/service"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/ikermy/air_logger/v2/pkg/logger"
)

// ContactHandler обрабатывает запросы по работе с контактами
type ContactHandler struct {
	service *service.CRMService
}

// NewContactHandler создает новый handler
func NewContactHandler(service *service.CRMService) *ContactHandler {
	return &ContactHandler{service: service}
}

// FindContactByPhoneRequest структура запроса на поиск контакта по телефону
type FindContactByPhoneRequest struct {
	Phone string `json:"phone" binding:"required"`
}

// FindContactByPhone ищет контакт по номеру телефона
// GET /contacts/:crm_type/search?phone=+79991234567
// или POST /contacts/:crm_type/search с JSON {"phone": "+79991234567"}
func (h *ContactHandler) FindContactByPhone(c *fiber.Ctx) error {
	// Получаем userID из контекста (установлен middleware)
	userID, ok := GetUserID(c)
	if !ok {
		observeEntityMetric("contact", "find_by_phone", http.StatusUnauthorized, "missing_user_id")
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "user_id не найден"})
	}

	// Получаем crm_type из URL
	crmType := c.Params("crm_type")
	if crmType == "" {
		observeEntityMetric("contact", "find_by_phone", http.StatusBadRequest, "missing_crm_type")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "crm_type обязателен"})
	}

	// Получаем номер телефона из query параметра или JSON тела
	var phone string
	if c.Method() == "GET" {
		phone = c.Query("phone")
	} else {
		var req FindContactByPhoneRequest
		if err := c.BodyParser(&req); err != nil {
			observeEntityMetric("contact", "find_by_phone", http.StatusBadRequest, "invalid_body")
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "некорректный запрос", "details": err.Error()})
		}
		phone = req.Phone
	}

	if phone == "" {
		observeEntityMetric("contact", "find_by_phone", http.StatusBadRequest, "missing_phone")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "phone обязателен"})
	}

	logger.Debug("Поиск контакта по телефону: userID=%d, crmType=%s, phone=%s", userID, crmType, phone)

	// Ищем контакт через сервис
	contact, err := h.service.FindContactByPhone(c.Context(), userID, crmType, phone)
	if err != nil {
		observeEntityMetric("contact", "find_by_phone", http.StatusBadRequest, "service_error")
		logger.Error("Ошибка поиска контакта: %v", err, userID)
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "контакт не найден", "details": err.Error()})
	}

	logger.Debug("Контакт найден: ID=%s, Name=%s", contact.ID, contact.Name)

	observeEntityMetric("contact", "find_by_phone", http.StatusOK, "")
	return c.Status(http.StatusOK).JSON(fiber.Map{
		"contact": contact,
		"success": true,
	})
}

// FindContactByAltContactRequest структура запроса на поиск контакта по альтернативному контакту
type FindContactByAltContactRequest struct {
	AltContact string `json:"alt_contact" binding:"required"`
}

// FindContactByAltContact ищет контакт по альтернативному контакту (например, UserID Telegram, VK ID)
// GET /contacts/:crm_type/search-by-alt?alt_contact=123456789
// или POST /contacts/:crm_type/search-by-alt с JSON {"alt_contact": "123456789"}
func (h *ContactHandler) FindContactByAltContact(c *fiber.Ctx) error {
	// Получаем userID из контекста (установлен middleware)
	userID, ok := GetUserID(c)
	if !ok {
		observeEntityMetric("contact", "find_by_alt_contact", http.StatusUnauthorized, "missing_user_id")
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "user_id не найден"})
	}

	// Получаем crm_type из URL
	crmType := c.Params("crm_type")
	if crmType == "" {
		observeEntityMetric("contact", "find_by_alt_contact", http.StatusBadRequest, "missing_crm_type")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "crm_type обязателен"})
	}

	// Получаем альтернативный контакт из query параметра или JSON тела
	var altContact string
	if c.Method() == "GET" {
		altContact = c.Query("alt_contact")
	} else {
		var req FindContactByAltContactRequest
		if err := c.BodyParser(&req); err != nil {
			observeEntityMetric("contact", "find_by_alt_contact", http.StatusBadRequest, "invalid_body")
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "некорректный запрос", "details": err.Error()})
		}
		altContact = req.AltContact
	}

	if altContact == "" {
		observeEntityMetric("contact", "find_by_alt_contact", http.StatusBadRequest, "missing_alt_contact")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "alt_contact обязателен"})
	}

	logger.Debug("Поиск контакта по альтернативному контакту: userID=%d, crmType=%s, alt_contact=%s", userID, crmType, altContact)

	// Ищем контакт через сервис
	contact, err := h.service.FindContactByAltContact(c.Context(), userID, crmType, altContact)
	if err != nil {
		observeEntityMetric("contact", "find_by_alt_contact", http.StatusBadRequest, "service_error")
		logger.Error("Ошибка поиска контакта по альтернативному контакту: %v", err, userID)
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "контакт не найден", "details": err.Error()})
	}

	logger.Debug("Контакт найден: ID=%s, Name=%s, AltContact=%s", contact.ID, contact.Name, contact.AltContact)

	observeEntityMetric("contact", "find_by_alt_contact", http.StatusOK, "")
	return c.Status(http.StatusOK).JSON(fiber.Map{
		"contact": contact,
		"success": true,
	})
}

// CreateContact создает новый контакт в CRM
// POST /contacts/:crm_type
func (h *ContactHandler) CreateContact(c *fiber.Ctx) error {
	// Получаем userID из контекста
	userID, ok := GetUserID(c)
	if !ok {
		observeEntityMetric("contact", "create", http.StatusUnauthorized, "missing_user_id")
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "user_id не найден"})
	}

	// Получаем crm_type из URL
	crmType := c.Params("crm_type")
	if crmType == "" {
		observeEntityMetric("contact", "create", http.StatusBadRequest, "missing_crm_type")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "crm_type обязателен"})
	}

	// Парсим тело запроса
	var contact models.Contact
	if err := c.BodyParser(&contact); err != nil {
		observeEntityMetric("contact", "create", http.StatusBadRequest, "invalid_body")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "некорректный запрос", "details": err.Error()})
	}

	// Валидация
	if contact.Name == "" {
		observeEntityMetric("contact", "create", http.StatusBadRequest, "missing_name")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "name обязателен"})
	}

	logger.Debug("Создание контакта: userID=%d, crmType=%s, name=%s", userID, crmType, contact.Name)

	// Создаем контакт через сервис
	createdContact, err := h.service.CreateContact(c.Context(), userID, crmType, &contact)
	if err != nil {
		observeEntityMetric("contact", "create", http.StatusInternalServerError, "service_error")
		logger.Error("Ошибка создания контакта: %v", err, userID)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "ошибка создания контакта", "details": err.Error()})
	}

	logger.Info("Контакт создан: ID=%s, Name=%s", createdContact.ID, createdContact.Name, userID)

	observeEntityMetric("contact", "create", http.StatusCreated, "")
	return c.Status(http.StatusCreated).JSON(fiber.Map{
		"contact": createdContact,
		"success": true,
		"message": "контакт успешно создан",
	})
}

// GetContact получает контакт по ID
// GET /contacts/:crm_type/:contact_id
func (h *ContactHandler) GetContact(c *fiber.Ctx) error {
	// Получаем userID из контекста
	userID, ok := GetUserID(c)
	if !ok {
		observeEntityMetric("contact", "get", http.StatusUnauthorized, "missing_user_id")
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "user_id не найден"})
	}

	// Получаем параметры из URL
	crmType := c.Params("crm_type")
	contactID := c.Params("contact_id")

	if crmType == "" || contactID == "" {
		observeEntityMetric("contact", "get", http.StatusBadRequest, "missing_params")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "crm_type и contact_id обязательны"})
	}

	logger.Debug("Получение контакта: userID=%d, crmType=%s, contactID=%s", userID, crmType, contactID)

	// Получаем контакт через сервис
	contact, err := h.service.GetContact(c.Context(), userID, crmType, contactID)
	if err != nil {
		observeEntityMetric("contact", "get", http.StatusNotFound, "service_error")
		logger.Error("Ошибка получения контакта: %v", err, userID)
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "контакт не найден", "details": err.Error()})
	}

	logger.Debug("Контакт получен: ID=%s, Name=%s", contact.ID, contact.Name)

	observeEntityMetric("contact", "get", http.StatusOK, "")
	return c.Status(http.StatusOK).JSON(fiber.Map{
		"contact": contact,
		"success": true,
	})
}

// UpdateContact обновляет контакт в CRM
// PATCH /contacts/:crm_type/:contact_id
func (h *ContactHandler) UpdateContact(c *fiber.Ctx) error {
	// Получаем userID из контекста
	userID, ok := GetUserID(c)
	if !ok {
		observeEntityMetric("contact", "update", http.StatusUnauthorized, "missing_user_id")
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "user_id не найден"})
	}

	// Получаем параметры из URL
	crmType := c.Params("crm_type")
	contactID := c.Params("contact_id")

	if crmType == "" || contactID == "" {
		observeEntityMetric("contact", "update", http.StatusBadRequest, "missing_params")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "crm_type и contact_id обязательны"})
	}

	// Парсим тело запроса
	var contact models.Contact
	if err := c.BodyParser(&contact); err != nil {
		observeEntityMetric("contact", "update", http.StatusBadRequest, "invalid_body")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "некорректный запрос", "details": err.Error()})
	}

	logger.Debug("Обновление контакта: userID=%d, crmType=%s, contactID=%s", userID, crmType, contactID)

	// Обновляем контакт через сервис
	updatedContact, err := h.service.UpdateContact(c.Context(), userID, crmType, contactID, &contact)
	if err != nil {
		observeEntityMetric("contact", "update", http.StatusInternalServerError, "service_error")
		logger.Error("Ошибка обновления контакта: %v", err, userID)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "ошибка обновления контакта", "details": err.Error()})
	}

	logger.Info("Контакт обновлен: ID=%s, Name=%s", updatedContact.ID, updatedContact.Name, userID)

	observeEntityMetric("contact", "update", http.StatusOK, "")
	return c.Status(http.StatusOK).JSON(fiber.Map{
		"contact": updatedContact,
		"success": true,
		"message": "контакт успешно обновлен",
	})
}

// GetCustomFields получает список пользовательских полей контактов
// GET /contacts/:crm_type/custom-fields
func (h *ContactHandler) GetCustomFields(c *fiber.Ctx) error {
	// Получаем userID из контекста
	userID, ok := GetUserID(c)
	if !ok {
		observeEntityMetric("contact", "list_custom_fields", http.StatusUnauthorized, "missing_user_id")
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "user_id не найден"})
	}

	// Получаем crm_type из URL
	crmType := c.Params("crm_type")
	if crmType == "" {
		observeEntityMetric("contact", "list_custom_fields", http.StatusBadRequest, "missing_crm_type")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "crm_type обязателен"})
	}

	logger.Debug("Получение custom fields: userID=%d, crmType=%s", userID, crmType)

	// Получаем custom fields через сервис
	fields, err := h.service.GetCustomFields(c.Context(), userID, crmType, "contacts")
	if err != nil {
		observeEntityMetric("contact", "list_custom_fields", http.StatusInternalServerError, "service_error")
		logger.Error("Ошибка получения custom fields: %v", err, userID)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "ошибка получения custom fields", "details": err.Error()})
	}

	observeEntityMetric("contact", "list_custom_fields", http.StatusOK, "")
	return c.Status(http.StatusOK).JSON(fiber.Map{
		"custom_fields": fields,
		"success":       true,
	})
}

// CreateCustomFieldRequest структура запроса на создание пользовательского поля
type CreateCustomFieldRequest struct {
	Name      string `json:"name" binding:"required"`
	FieldType string `json:"type" binding:"required"` // text, numeric, checkbox, select, multiselect, date, url, textarea, streetaddress, smart_address, birthday, legal_entity
	Code      string `json:"code,omitempty"`
	Enums     []struct {
		Value string `json:"value"`
		Sort  int    `json:"sort,omitempty"`
	} `json:"enums,omitempty"` // Только для select и multiselect
}

// CreateCustomField создает новое пользовательское поле для контактов
// POST /contacts/:crm_type/custom-fields
func (h *ContactHandler) CreateCustomField(c *fiber.Ctx) error {
	// Получаем userID из контекста
	userID, ok := GetUserID(c)
	if !ok {
		observeEntityMetric("contact", "create_custom_field", http.StatusUnauthorized, "missing_user_id")
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "user_id не найден"})
	}

	// Получаем crm_type из URL
	crmType := c.Params("crm_type")
	if crmType == "" {
		observeEntityMetric("contact", "create_custom_field", http.StatusBadRequest, "missing_crm_type")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "crm_type обязателен"})
	}

	// Парсим тело запроса
	var req CreateCustomFieldRequest
	if err := c.BodyParser(&req); err != nil {
		observeEntityMetric("contact", "create_custom_field", http.StatusBadRequest, "invalid_body")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "некорректный запрос", "details": err.Error()})
	}

	logger.Debug("Создание custom field: userID=%d, crmType=%s, name=%s, type=%s", userID, crmType, req.Name, req.FieldType)

	// Создаем custom field через сервис
	field, err := h.service.CreateCustomField(c.Context(), userID, crmType, "contacts", &req)
	if err != nil {
		observeEntityMetric("contact", "create_custom_field", http.StatusInternalServerError, "service_error")
		logger.Error("Ошибка создания custom field: %v", err, userID)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "ошибка создания custom field", "details": err.Error()})
	}

	logger.Info("Custom field создан: ID=%v, Name=%s", field, req.Name, userID)

	observeEntityMetric("contact", "create_custom_field", http.StatusCreated, "")
	return c.Status(http.StatusCreated).JSON(fiber.Map{
		"custom_field": field,
		"success":      true,
		"message":      "пользовательское поле успешно создано",
	})
}
