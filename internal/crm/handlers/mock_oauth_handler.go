package handlers

import (
	"Marusia_CRM/internal/domain/models"
	"Marusia_CRM/internal/domain/service"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/ikermy/air_logger/v2/pkg/logger"
)

// MockOAuthHandler обрабатывает mock OAuth для локального тестирования
type MockOAuthHandler struct {
	service *service.CRMService
}

// NewMockOAuthHandler создает новый mock handler
func NewMockOAuthHandler(service *service.CRMService) *MockOAuthHandler {
	return &MockOAuthHandler{service: service}
}

// MockAmoCRMAuth создает mock токены и сохраняет в конфигурацию (для локального тестирования БЕЗ ngrok)
// Требует предварительного сохранения конфигурации (client_id, client_secret, subdomain)
// POST /mock/oauth/amocrm/auth
func (h *MockOAuthHandler) MockAmoCRMAuth(c *fiber.Ctx) error {
	userID, ok := GetUserID(c)
	if !ok {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "user_id не найден"})
	}
	userIDUint := userID

	// Получаем существующую конфигурацию AmoCRM для этого пользователя
	config, err := h.service.GetCRMConfigByType(userIDUint, "amocrm")
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{
			"error": "конфигурация AmoCRM не найдена",
			"hint":  "Сначала создайте конфигурацию через POST /crm/configs/amocrm с client_id, client_secret и subdomain",
		})
	}

	// Парсим credentials
	var credentials models.CRMCredentials
	if err := json.Unmarshal([]byte(config.Credentials), &credentials); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "ошибка чтения credentials"})
	}

	if credentials.ClientID == "" || credentials.ClientSecret == "" || config.Subdomain == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "неполная конфигурация",
			"hint":  "В конфигурации должны быть указаны client_id, client_secret и subdomain",
		})
	}

	// Создаем mock токены (для тестирования)
	mockTokens := service.AmoCRMTokens{
		AccessToken:  "mock_access_token_" + time.Now().Format("20060102150405"),
		RefreshToken: "mock_refresh_token_" + time.Now().Format("20060102150405"),
		TokenType:    "Bearer",
		ExpiresIn:    86400, // 24 часа
		ExpiresAt:    time.Now().Add(24 * time.Hour).Unix(),
	}

	// Обновляем credentials токенами
	credentials.AccessToken = mockTokens.AccessToken
	credentials.RefreshToken = mockTokens.RefreshToken
	credentials.ExpiresAt = mockTokens.ExpiresAt

	credentialsJSON, err := json.Marshal(credentials)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "ошибка сохранения credentials"})
	}

	config.Credentials = string(credentialsJSON)
	config.IsActive = true

	if err := h.service.UpsertCRMConfig(config); err != nil {
		logger.Error("Ошибка обновления mock CRM конфига: %v", err)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error":   "ошибка сохранения токенов",
			"details": err.Error(),
		})
	}

	logger.Info("Mock AmoCRM токены созданы для пользователя %d", userIDUint)

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success":      true,
		"message":      "Mock AmoCRM успешно авторизована (для локального тестирования)",
		"crm_type":     config.CRMType,
		"subdomain":    config.Subdomain,
		"access_token": mockTokens.AccessToken,
		"expires_at":   mockTokens.ExpiresAt,
		"note":         "⚠️  Это MOCK токены! Используйте ТОЛЬКО для локального тестирования БЕЗ ngrok!",
	})
}

// MockAmoCRMRefreshToken обновляет mock токен
// POST /mock/oauth/amocrm/refresh
func (h *MockOAuthHandler) MockAmoCRMRefreshToken(c *fiber.Ctx) error {
	userID, ok := GetUserID(c)
	if !ok {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "user_id не найден"})
	}
	userIDUint := userID

	// Получаем конфигурацию по crm_type (аналогично боевому коду)
	config, err := h.service.GetCRMConfigByType(userIDUint, "amocrm")
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "конфигурация AmoCRM не найдена"})
	}

	// Парсим существующие credentials
	var credentials models.CRMCredentials
	if err := json.Unmarshal([]byte(config.Credentials), &credentials); err != nil {
		logger.Error("Ошибка парсинга credentials: %v", err)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "ошибка чтения credentials"})
	}

	// Генерируем новые mock токены
	mockTokens := service.AmoCRMTokens{
		AccessToken:  "mock_access_token_refreshed_" + time.Now().Format("20060102150405"),
		RefreshToken: "mock_refresh_token_refreshed_" + time.Now().Format("20060102150405"),
		TokenType:    "Bearer",
		ExpiresIn:    86400,
		ExpiresAt:    time.Now().Add(24 * time.Hour).Unix(),
	}

	// Обновляем токены в credentials
	credentials.AccessToken = mockTokens.AccessToken
	credentials.RefreshToken = mockTokens.RefreshToken
	credentials.ExpiresAt = mockTokens.ExpiresAt

	credentialsJSON, err := json.Marshal(credentials)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "ошибка сохранения credentials"})
	}

	config.Credentials = string(credentialsJSON)

	if err := h.service.UpdateCRMConfig(config); err != nil {
		logger.Error("Ошибка обновления CRM конфига: %v", err)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error":   "ошибка обновления токенов",
			"details": err.Error(),
		})
	}

	logger.Info("Mock AmoCRM токен обновлен для пользователя %d", userIDUint)

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success":      true,
		"message":      "Mock токен успешно обновлен",
		"access_token": mockTokens.AccessToken,
		"expires_at":   mockTokens.ExpiresAt,
		"note":         "⚠️  Это MOCK токены! Используйте ТОЛЬКО для локального тестирования БЕЗ ngrok!",
	})
}
