package handlers

import (
	"Marusia_CRM/internal/domain/models"
	"Marusia_CRM/internal/domain/service"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/ikermy/air_logger/v2/pkg/logger"
)

// OAuthHandler обрабатывает OAuth 2.0 запросы для различных CRM
type OAuthHandler struct {
	service *service.CRMService
}

// NewOAuthHandler создает новый handler
func NewOAuthHandler(service *service.CRMService) *OAuthHandler {
	return &OAuthHandler{service: service}
}

// AmoCRMAuthRequest структура запроса для получения URL авторизации
// Важно: client_id, client_secret, subdomain должны быть уже сохранены в crm_configs!
type AmoCRMAuthRequest struct {
	RedirectURL string `json:"redirect_url"` // Опционально (не используется при mode=post_message)
	State       string `json:"state"`        // Опционально (генерируется автоматически если не указан)
}

// AmoCRMAuth генерирует URL для авторизации в AmoCRM
// Требует предварительного сохранения конфигурации (client_id, client_secret, subdomain)
// POST /api/v1/oauth/amocrm/auth
func (h *OAuthHandler) AmoCRMAuth(c *fiber.Ctx) error {
	var req AmoCRMAuthRequest
	if err := c.BodyParser(&req); err != nil {
		observeOAuthMetric("amocrm", "auth", http.StatusBadRequest, "invalid_body")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "некорректный запрос", "details": err.Error()})
	}

	userID, ok := GetUserID(c)
	if !ok {
		observeOAuthMetric("amocrm", "auth", http.StatusUnauthorized, "missing_user_id")
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "user_id не найден"})
	}
	userIDUint := userID

	// Получаем существующую конфигурацию AmoCRM для этого пользователя
	// Используем Internal метод чтобы получить client_secret (не удаляется sanitizeCredentials)
	config, err := h.service.GetCRMConfigByTypeInternal(userIDUint, "amocrm")
	if err != nil {
		observeOAuthMetric("amocrm", "auth", http.StatusNotFound, "config_not_found")
		return c.Status(http.StatusNotFound).JSON(fiber.Map{
			"error": "конфигурация AmoCRM не найдена",
			"hint":  "Сначала создайте конфигурацию через POST /crm/configs/amocrm с client_id, client_secret и subdomain",
		})
	}

	// Парсим credentials для получения client_id
	var credentials models.CRMCredentials
	if err := json.Unmarshal([]byte(config.Credentials), &credentials); err != nil {
		observeOAuthMetric("amocrm", "auth", http.StatusInternalServerError, "credentials_parse_error")
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "ошибка чтения credentials"})
	}

	if credentials.ClientID == "" || credentials.ClientSecret == "" || config.Subdomain == "" {
		observeOAuthMetric("amocrm", "auth", http.StatusBadRequest, "incomplete_config")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "неполная конфигурация",
			"hint":  "В конфигурации должны быть указаны client_id, client_secret и subdomain",
		})
	}

	state := req.State
	if state == "" {
		state = fmt.Sprintf("%d_%d", userIDUint, time.Now().Unix())
	}

	oauthState := &models.OAuthState{
		State:        state,
		UserID:       userIDUint,
		ClientID:     credentials.ClientID,
		ClientSecret: credentials.ClientSecret,
		RedirectURL:  credentials.RedirectURL,
		Subdomain:    config.Subdomain,
		CRMType:      "amocrm",
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(20 * time.Minute),
	}

	if err := h.service.SaveOAuthState(oauthState); err != nil {
		logger.Error("Ошибка сохранения OAuth state: %v", err)
		observeOAuthMetric("amocrm", "auth", http.StatusInternalServerError, "save_state_failed")
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "ошибка сохранения состояния"})
	}

	authURL := fmt.Sprintf(
		"https://www.amocrm.ru/oauth?client_id=%s&state=%s&redirect_uri=%s",
		url.QueryEscape(credentials.ClientID),
		url.QueryEscape(state),
		url.QueryEscape(credentials.RedirectURL),
	)

	observeOAuthMetric("amocrm", "auth", http.StatusOK, "")
	return c.Status(http.StatusOK).JSON(fiber.Map{
		"auth_url":     authURL,
		"state":        state,
		"client_id":    credentials.ClientID,
		"subdomain":    config.Subdomain,
		"instructions": "Откройте auth_url в новом окне. После авторизации amoCRM вернет код через postMessage",
		"expires_in":   1200,
		"note":         "Код авторизации действителен 20 минут",
	})
}

// AmoCRMCallback обрабатывает redirect от AmoCRM после авторизации
// AmoCRM редиректит на этот URL с параметрами code и state
// GET /oauth/amocrm/callback?code=xxx&state=yyy&client_id=zzz&referer=company
func (h *OAuthHandler) AmoCRMCallback(c *fiber.Ctx) error {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" {
		logger.Error("AmoCRMCallback: отсутствует код авторизации")
		observeOAuthMetric("amocrm", "callback", http.StatusBadRequest, "missing_code")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "отсутствует код авторизации"})
	}

	if state == "" {
		logger.Error("AmoCRMCallback: отсутствует state")
		observeOAuthMetric("amocrm", "callback", http.StatusBadRequest, "missing_state")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "отсутствует state"})
	}

	defer func() {
		err := h.service.DeleteOAuthState(state)
		if err != nil {
			logger.Error("AmoCRMCallback: ошибка удаления OAuth state: %v", err)
		}
	}()

	oauthState, err := h.service.GetOAuthState(state)
	if err != nil {
		logger.Error("AmoCRMCallback: OAuth state не найден или истек: %v", err)
		observeOAuthMetric("amocrm", "callback", http.StatusBadRequest, "invalid_state")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error":   "неверный или истекший state",
			"details": "Возможно, прошло более 20 минут с момента инициации авторизации",
		})
	}

	tokens, err := h.service.ExchangeAmoCRMCode(
		c.Context(),
		code,
		oauthState.ClientID,
		oauthState.ClientSecret,
		oauthState.RedirectURL,
		oauthState.Subdomain,
	)
	if err != nil {
		logger.Error("AmoCRMCallback: ошибка обмена кода AmoCRM: %v", err)
		observeOAuthMetric("amocrm", "callback", http.StatusInternalServerError, "exchange_failed")
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error":   "ошибка получения токенов",
			"details": err.Error(),
		})
	}

	config, err := h.service.GetCRMConfigByType(oauthState.UserID, "amocrm")
	if err != nil {
		logger.Error("AmoCRMCallback: конфигурация не найдена: %v", err)
		observeOAuthMetric("amocrm", "callback", http.StatusNotFound, "config_not_found")
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "конфигурация AmoCRM не найдена"})
	}

	var credentials models.CRMCredentials
	if err := json.Unmarshal([]byte(config.Credentials), &credentials); err != nil {
		logger.Error("AmoCRMCallback: ошибка парсинга credentials: %v", err)
		observeOAuthMetric("amocrm", "callback", http.StatusInternalServerError, "credentials_parse_error")
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "ошибка чтения credentials"})
	}

	credentials.AccessToken = tokens.AccessToken
	credentials.RefreshToken = tokens.RefreshToken
	credentials.ExpiresAt = tokens.ExpiresAt

	credentialsJSON, err := json.Marshal(credentials)
	if err != nil {
		logger.Error("AmoCRMCallback: ошибка маршалинга credentials: %v", err)
		observeOAuthMetric("amocrm", "callback", http.StatusInternalServerError, "persist_failed")
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "ошибка сохранения credentials"})
	}

	config.Credentials = string(credentialsJSON)
	config.IsActive = true

	if err := h.service.UpsertCRMConfig(config); err != nil {
		logger.Error("AmoCRMCallback: ошибка обновления CRM конфига: %v", err)
		observeOAuthMetric("amocrm", "callback", http.StatusInternalServerError, "persist_failed")
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error":   "ошибка сохранения токенов",
			"details": err.Error(),
		})
	}

	observeOAuthMetric("amocrm", "callback", http.StatusOK, "")
	c.Set("Content-Type", "text/html; charset=utf-8")
	htmlContent := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Авторизация завершена</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            height: 100vh;
            margin: 0;
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            color: white;
        }
        .container {
            text-align: center;
            padding: 40px;
            background: rgba(255, 255, 255, 0.1);
            border-radius: 10px;
            backdrop-filter: blur(10px);
        }
        .success-icon {
            font-size: 64px;
            margin-bottom: 20px;
        }
        h1 {
            margin: 0 0 10px 0;
            font-size: 24px;
        }
        p {
            margin: 0;
            opacity: 0.9;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="success-icon">✅</div>
        <h1>amoCRM успешно авторизована!</h1>

        <p>Окно будет закрыто автоматически...</p>
    </div>
    <script>
        // Отправляем результат родительскому окну (если есть)
        if (window.opener) {
            window.opener.postMessage({
                type: 'amocrm_auth_success',
                success: true,
                message: 'amoCRM успешно авторизована',
                crm_type: 'amocrm',
                subdomain: '%s',
                expires_at: %d
            }, '*');
        }
        
        // Закрываем окно через 2 секунды
        setTimeout(function() {
            window.close();
        }, 2000);
    </script>
</body>
</html>
`, config.Subdomain, tokens.ExpiresAt)
	return c.Status(http.StatusOK).SendString(htmlContent)
}

// AmoCRMRefreshToken обновляет токен доступа
// POST /api/v1/oauth/amocrm/refresh
func (h *OAuthHandler) AmoCRMRefreshToken(c *fiber.Ctx) error {
	var req struct {
		ConfigID uint32 `json:"config_id" binding:"required"`
	}

	if err := c.BodyParser(&req); err != nil {
		observeOAuthMetric("amocrm", "refresh", http.StatusBadRequest, "invalid_body")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "некорректный запрос", "details": err.Error()})
	}

	userID, ok := GetUserID(c)
	if !ok {
		observeOAuthMetric("amocrm", "refresh", http.StatusBadRequest, "missing_user_id")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "user_id не найден"})
	}

	config, err := h.service.GetCRMConfig(req.ConfigID)
	if err != nil {
		observeOAuthMetric("amocrm", "refresh", http.StatusNotFound, "config_not_found")
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "конфигурация не найдена"})
	}

	if config.UserID != userID {
		observeOAuthMetric("amocrm", "refresh", http.StatusForbidden, "forbidden")
		return c.Status(http.StatusForbidden).JSON(fiber.Map{"error": "доступ запрещен"})
	}

	newTokens, err := h.service.RefreshAmoCRMToken(c.Context(), req.ConfigID)
	if err != nil {
		logger.Error("Ошибка обновления токена AmoCRM: %v", err)
		observeOAuthMetric("amocrm", "refresh", http.StatusInternalServerError, "refresh_failed")
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error":   "ошибка обновления токена",
			"details": err.Error(),
		})
	}

	observeOAuthMetric("amocrm", "refresh", http.StatusOK, "")
	return c.Status(http.StatusOK).JSON(fiber.Map{
		"message":    "токен успешно обновлен",
		"expires_at": newTokens.ExpiresAt,
	})
}
