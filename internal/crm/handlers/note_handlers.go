package handlers

import (
	"Marusia_CRM/internal/domain/models"
	"Marusia_CRM/internal/domain/service"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/ikermy/air_logger/v2/pkg/logger"
)

// NoteHandler обрабатывает запросы по работе с примечаниями
type NoteHandler struct {
	service *service.CRMService
}

// NewNoteHandler создает новый handler
func NewNoteHandler(service *service.CRMService) *NoteHandler {
	return &NoteHandler{service: service}
}

// CreateLeadNote создает примечание для лида
// POST /leads/:crm_type/:lead_id/notes
func (h *NoteHandler) CreateLeadNote(c *fiber.Ctx) error {
	userID, ok := GetUserID(c)
	if !ok {
		observeEntityMetric("note", "create", http.StatusUnauthorized, "missing_user_id")
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "user_id не найден"})
	}

	crmType := c.Params("crm_type")
	leadID := c.Params("lead_id")
	if crmType == "" || leadID == "" {
		observeEntityMetric("note", "create", http.StatusBadRequest, "missing_params")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "crm_type и lead_id обязательны"})
	}

	var req models.CreateNoteRequest
	if err := c.BodyParser(&req); err != nil {
		observeEntityMetric("note", "create", http.StatusBadRequest, "invalid_body")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "некорректный запрос", "details": err.Error()})
	}

	logger.Debug("Создание примечания для лида: userID=%d, crmType=%s, leadID=%s, noteType=%s", userID, crmType, leadID, req.NoteType)

	note, err := h.service.CreateLeadNote(c.Context(), userID, crmType, leadID, &req)
	if err != nil {
		observeEntityMetric("note", "create", http.StatusInternalServerError, "service_error")
		logger.Error("Ошибка создания примечания: %v", err, userID)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "ошибка создания примечания", "details": err.Error()})
	}

	logger.Info("Примечание создано для лида ID=%s, note_id=%s", leadID, note.ID, userID)

	observeEntityMetric("note", "create", http.StatusCreated, "")
	return c.Status(http.StatusCreated).JSON(fiber.Map{
		"note":    note,
		"success": true,
		"message": "примечание успешно создано",
	})
}
