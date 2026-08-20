package http

import (
	"Marusia_CRM/internal/crm/handlers"
	"fmt"
	"strconv"
	"strings"
	"time"

	"Marusia_CRM/internal/metrics"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/ikermy/air_logger/v2/pkg/logger"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

// Server registers and serves the CRM HTTP API.
type Server struct {
	App              *fiber.App
	configHandler    *handlers.ConfigHandler
	leadHandler      *handlers.LeadHandler
	contactHandler   *handlers.ContactHandler
	talkHandler      *handlers.TalkHandler
	noteHandler      *handlers.NoteHandler
	pipelineHandler  *handlers.PipelineHandler
	oauthHandler     *handlers.OAuthHandler
	mockOAuthHandler *handlers.MockOAuthHandler
}

func NewServer(app *fiber.App, config *handlers.ConfigHandler, lead *handlers.LeadHandler, contact *handlers.ContactHandler, talk *handlers.TalkHandler, note *handlers.NoteHandler, pipeline *handlers.PipelineHandler, oauth *handlers.OAuthHandler, mockOAuth *handlers.MockOAuthHandler) *Server {
	return &Server{App: app, configHandler: config, leadHandler: lead, contactHandler: contact, talkHandler: talk, noteHandler: note, pipelineHandler: pipeline, oauthHandler: oauth, mockOAuthHandler: mockOAuth}
}

// extractUserID middleware для извлечения userID из заголовка X-User-ID
// Микросервис авторизации передает userID через заголовок X-User-ID
func (w *Server) extractUserID() fiber.Handler {
	return func(c *fiber.Ctx) error {
		startedAt := time.Now()
		userIDStr := c.Get("X-User-ID")

		if userIDStr == "" {
			handlers.ObserveHandlerMetric("auth", startedAt, fiber.StatusUnauthorized, "missing_header")
			logger.Error("extractUserID: отсутствует заголовок X-User-ID для %s %s", c.Method(), c.Path())
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing X-User-ID header"})
		}

		userID, err := strconv.ParseUint(userIDStr, 10, 32)
		if err != nil {
			handlers.ObserveHandlerMetric("auth", startedAt, fiber.StatusBadRequest, "invalid_header")
			logger.Error("extractUserID: ошибка парсинга X-User-ID='%s': %v", userIDStr, err)
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid X-User-ID format"})
		}

		if userID == 0 {
			handlers.ObserveHandlerMetric("auth", startedAt, fiber.StatusBadRequest, "invalid_user_id")
			logger.Error("extractUserID: userID не может быть 0")
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "userID cannot be 0"})
		}

		handlers.ObserveHandlerMetric("auth", startedAt, fiber.StatusOK, "")
		c.Locals("user_id", uint32(userID))
		return c.Next()
	}
}

func (w *Server) Handler() error {
	logger.Infoln("Web server Marusia_CRM started")

	// CORS middleware
	w.App.Use(cors.New(cors.Config{
		AllowOriginsFunc: func(origin string) bool {
			return strings.Contains(origin, "localhost")
		},
		AllowCredentials: true,
		AllowMethods:     "POST,GET,DELETE,PATCH,PUT,OPTIONS",
		AllowHeaders:     "Content-Type,Authorization,X-User-ID,X-Service-Key",
	}))

	// Health check (без авторизации)
	w.App.Get("/health", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	// Prometheus metrics middleware (для всех маршрутов, включая публичные)
	w.App.Use(metrics.FiberMiddleware())

	// Public metrics endpoint (Prometheus scrape)
	w.App.Get("/metrics", func(c *fiber.Ctx) error {
		fasthttpadaptor.NewFastHTTPHandler(metrics.Handler())(c.Context())
		return nil
	})

	// Группы маршрутов по схеме оркестратора
	v1 := w.App.Group("/v1")
	crm := v1.Group("/crm")

	// Public routes для orchestrator
	crm.Get("/health", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	// API группа (новый контракт: /v1/crm/api/...)
	api := crm.Group("/api")
	api.Use(w.extractUserID())

	// Legacy API группа (совместимость со старым контрактом: /configs, /leads, ...)
	legacy := w.App.Group("/")
	legacy.Use(w.extractUserID())

	routes := func(group fiber.Router) {
		// ========== CRM Configs ==========
		// Получить список всех конфигураций пользователя
		group.Get("/configs", w.configHandler.GetCRMConfigs)
		// Получить конфигурацию по crm_type
		group.Get("/configs/:crm_type", w.configHandler.GetCRMConfigByType)
		// Сохранить (создать или обновить) конфигурацию по crm_type
		group.Post("/configs/:crm_type", w.configHandler.SaveCRMConfig)
		// Установить статус активности конфигурации (is_active)
		group.Post("/configs/:crm_type/active", w.configHandler.SetCRMConfigActive)
		// Получить список всех пользовательских полей (стандартных и кастомных)
		group.Get("/configs/:crm_type/custom-fields", w.configHandler.GetCustomFields)
		// Создать новое пользовательское поле
		group.Post("/configs/:crm_type/custom-fields", w.configHandler.CreateCustomField)
		// Установить ID поля "Источник перехода" для автоматического заполнения
		group.Post("/configs/:crm_type/marusia-source-field", w.configHandler.SetMarusiaSourceFieldID)
		// Установить настройки лида по умолчанию (pipeline_id и status_id) для UpdateLead
		group.Post("/configs/:crm_type/default-lead-settings", w.configHandler.SetDefaultLeadSettings)
		// Получить настройки для каналов
		group.Get("/configs/:crm_type/channels", w.configHandler.GetChannelSettings)
		// Сохранить настройки для каналов
		group.Post("/configs/:crm_type/channels", w.configHandler.SetChannelSettings)
		// Удалить конфигурацию по crm_type
		group.Delete("/configs/:crm_type", w.configHandler.DeleteCRMConfigByType)
		// Тестировать подключение по crm_type
		group.Get("/configs/:crm_type/test", w.configHandler.TestConnectionByType)

		// ========== Leads ==========
		group.Post("/leads", w.leadHandler.CreateLead)
		// Создать лид AI диалог
		group.Post("/leads/:crm_type/ai-dialog/:contact_id", w.leadHandler.CreateAIDialogLead)
		// Получить лиды по ID контакта
		group.Get("/leads/:crm_type/by-contact/:contact_id", w.leadHandler.GetLeadsByContactID)
		// Обновить лид
		group.Patch("/leads/:crm_type/:lead_id", w.leadHandler.UpdateLead)
		// Получить лид
		group.Get("/leads/:crm_type/:lead_id", w.leadHandler.GetLead)
		// Добавить примечание к лиду (фактически будет использоваться для добавления диалогов)
		group.Post("/leads/:crm_type/:lead_id/notes", w.noteHandler.CreateLeadNote)

		// ========== Pipelines (Воронки) ==========
		group.Get("/pipelines/:crm_type", w.pipelineHandler.GetPipelines) // Получить список воронок и статусов

		// ========== Talks (Беседы) ==========
		group.Get("/talks/:crm_type/:talk_id", w.talkHandler.GetTalk) // Получить беседу по ID

		// ========== Contacts ==========
		// Получить список пользовательских полей контактов
		group.Get("/contacts/:crm_type/custom-fields", w.contactHandler.GetCustomFields)
		// Создать новое пользовательское поле для контактов
		group.Post("/contacts/:crm_type/custom-fields", w.contactHandler.CreateCustomField)
		// Создать контакт
		group.Post("/contacts/:crm_type", w.contactHandler.CreateContact)
		// Получить контакт по ID
		group.Get("/contacts/:crm_type/:contact_id", w.contactHandler.GetContact)
		// Обновить контакт
		group.Patch("/contacts/:crm_type/:contact_id", w.contactHandler.UpdateContact)
		// Поиск контакта по телефону
		group.Get("/contacts/:crm_type/search", w.contactHandler.FindContactByPhone)
		// Поиск контакта по альтернативному контакту (например, UserID Telegram, VK ID)
		group.Get("/contacts/:crm_type/search-by-alt", w.contactHandler.FindContactByAltContact)
	}

	routes(api)
	routes(legacy)

	// ========== OAuth Callbacks (без middleware userID для callback) ==========
	crm.Get("/oauth/amocrm/callback", w.oauthHandler.AmoCRMCallback)
	w.App.Get("/oauth/amocrm/callback", w.oauthHandler.AmoCRMCallback)

	// OAuth endpoints с авторизацией
	oauth := crm.Group("/oauth")
	oauth.Use(w.extractUserID())
	oauth.Post("/amocrm/auth", w.oauthHandler.AmoCRMAuth)
	oauth.Post("/amocrm/refresh", w.oauthHandler.AmoCRMRefreshToken)

	// Legacy OAuth endpoints (совместимость)
	legacyOAuth := w.App.Group("/oauth")
	legacyOAuth.Use(w.extractUserID())
	legacyOAuth.Post("/amocrm/auth", w.oauthHandler.AmoCRMAuth)
	legacyOAuth.Post("/amocrm/refresh", w.oauthHandler.AmoCRMRefreshToken)

	// ========== Webhooks (без middleware userID) ==========
	// TODO: Реализовать webhooks
	// w.App.Post("/api/v1/webhooks/amocrm/:config_id", w.webhookHandler.AmoCRMWebhook)

	// ========== Mock OAuth (для локального тестирования БЕЗ ngrok) ==========
	mock := crm.Group("/mock/oauth")
	mock.Use(w.extractUserID())
	mock.Post("/amocrm/auth", w.mockOAuthHandler.MockAmoCRMAuth)
	mock.Post("/amocrm/refresh", w.mockOAuthHandler.MockAmoCRMRefreshToken)

	// Legacy Mock OAuth endpoints (совместимость)
	legacyMock := w.App.Group("/mock/oauth")
	legacyMock.Use(w.extractUserID())
	legacyMock.Post("/amocrm/auth", w.mockOAuthHandler.MockAmoCRMAuth)
	legacyMock.Post("/amocrm/refresh", w.mockOAuthHandler.MockAmoCRMRefreshToken)

	// Запуск сервера
	if err := w.App.Listen("0.0.0.0:8080"); err != nil {
		return fmt.Errorf("ошибка запуска WEB сервера Marusia_CRM: %w", err)
	}

	return nil
}
