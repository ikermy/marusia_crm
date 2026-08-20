package handlers

import (
	"Marusia_CRM/internal/domain/service"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/ikermy/air_logger/v2/pkg/logger"
)

// TalkHandler обрабатывает запросы по работе с беседами
type TalkHandler struct {
	service *service.CRMService
}

// NewTalkHandler создает новый handler
func NewTalkHandler(service *service.CRMService) *TalkHandler {
	return &TalkHandler{service: service}
}

// GetTalk получает беседу из CRM
// GET /talks/:crm_type/:talk_id
func (h *TalkHandler) GetTalk(c *fiber.Ctx) error {
	userID, ok := GetUserID(c)
	if !ok {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "user_id не найден"})
	}

	crmType := c.Params("crm_type")
	talkID := c.Params("talk_id")
	if crmType == "" || talkID == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "crm_type и talk_id обязательны"})
	}

	logger.Debug("Получение беседы: userID=%d, crmType=%s, talkID=%s", userID, crmType, talkID)

	talk, err := h.service.GetTalk(c.Context(), userID, crmType, talkID)
	if err != nil {
		logger.Error("Ошибка получения беседы: %v", err, userID)
		if strings.Contains(err.Error(), "не найдена") {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "беседа не найдена", "details": err.Error()})
		}
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "ошибка получения беседы", "details": err.Error()})
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"talk":    talk,
		"success": true,
	})
}
