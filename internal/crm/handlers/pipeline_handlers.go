package handlers

import (
	"Marusia_CRM/internal/domain/service"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/ikermy/air_logger/v2/pkg/logger"
)

// PipelineHandler обрабатывает запросы по работе с воронками
type PipelineHandler struct {
	service *service.CRMService
}

// NewPipelineHandler создает новый handler
func NewPipelineHandler(service *service.CRMService) *PipelineHandler {
	return &PipelineHandler{service: service}
}

// GetPipelines получает список воронок и статусов из CRM
// GET /pipelines/:crm_type
func (h *PipelineHandler) GetPipelines(c *fiber.Ctx) error {
	userID, ok := GetUserID(c)
	if !ok {
		observeEntityMetric("pipeline", "list", http.StatusUnauthorized, "missing_user_id")
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "user_id не найден"})
	}

	crmType := c.Params("crm_type")
	if crmType == "" {
		observeEntityMetric("pipeline", "list", http.StatusBadRequest, "missing_crm_type")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "crm_type обязателен"})
	}

	logger.Debug("Получение воронок: crmType=%s", crmType, userID)

	result, err := h.service.GetPipelines(c.Context(), userID, crmType)
	if err != nil {
		observeEntityMetric("pipeline", "list", http.StatusInternalServerError, "service_error")
		logger.Error("Ошибка получения воронок: %v", err, userID)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "ошибка получения воронок", "details": err.Error()})
	}

	observeEntityMetric("pipeline", "list", http.StatusOK, "")
	return c.Status(http.StatusOK).JSON(fiber.Map{
		"pipelines":           result["pipelines"],
		"default_pipeline_id": result["default_pipeline_id"],
		"success":             true,
	})
}
