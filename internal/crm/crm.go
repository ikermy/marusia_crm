package crm

import (
	handlers2 "Marusia_CRM/internal/crm/handlers"
	"Marusia_CRM/internal/db"
	deliveryhttp "Marusia_CRM/internal/delivery/http"
	"Marusia_CRM/internal/domain/service"
	"Marusia_CRM/internal/providers"
	"Marusia_CRM/internal/providers/amocrm"
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/ikermy/air_common/pkg/endpoint"
	"github.com/ikermy/air_logger/v2/pkg/logger"
)

type CRM struct {
	ctx    context.Context
	cancel context.CancelFunc
	App    *fiber.App
	end    *endpoint.Endpoint
	db     *db.DB

	// Новые компоненты
	registry         *providers.Registry
	crmService       *service.CRMService
	configHandler    *handlers2.ConfigHandler
	leadHandler      *handlers2.LeadHandler
	contactHandler   *handlers2.ContactHandler
	talkHandler      *handlers2.TalkHandler
	noteHandler      *handlers2.NoteHandler
	pipelineHandler  *handlers2.PipelineHandler
	oauthHandler     *handlers2.OAuthHandler
	mockOAuthHandler *handlers2.MockOAuthHandler
	server           *deliveryhttp.Server
}

// New создает новый экземпляр CRM сервиса
func New(parent context.Context, d *db.DB, e *endpoint.Endpoint) *CRM {
	ctx, cancel := context.WithCancel(parent)

	// Создаем Fiber приложение
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	// Создаем реестр провайдеров
	registry := providers.NewRegistry()

	// Регистрируем провайдер AmoCRM
	registry.RegisterProvider("amocrm", func() providers.CRMProvider {
		return amocrm.NewAmoCRMProvider()
	})

	// Получаем репозиторий из DB
	repo := d.Repo()

	// Создаем сервис
	crmService := service.NewCRMService(repo, registry)

	// Создаем handlers
	configHandler := handlers2.NewConfigHandler(crmService)
	leadHandler := handlers2.NewLeadHandler(crmService)
	contactHandler := handlers2.NewContactHandler(crmService)
	talkHandler := handlers2.NewTalkHandler(crmService)
	noteHandler := handlers2.NewNoteHandler(crmService)
	pipelineHandler := handlers2.NewPipelineHandler(crmService)
	oauthHandler := handlers2.NewOAuthHandler(crmService)
	mockOAuthHandler := handlers2.NewMockOAuthHandler(crmService)

	c := &CRM{
		ctx:              ctx,
		cancel:           cancel,
		db:               d,
		end:              e,
		App:              app,
		registry:         registry,
		crmService:       crmService,
		configHandler:    configHandler,
		leadHandler:      leadHandler,
		contactHandler:   contactHandler,
		talkHandler:      talkHandler,
		noteHandler:      noteHandler,
		pipelineHandler:  pipelineHandler,
		oauthHandler:     oauthHandler,
		mockOAuthHandler: mockOAuthHandler,
	}
	c.server = deliveryhttp.NewServer(app, configHandler, leadHandler, contactHandler, talkHandler, noteHandler, pipelineHandler, oauthHandler, mockOAuthHandler)
	return c
}

func (w *CRM) Handler() error { return w.server.Handler() }

func (w *CRM) Shutdown() {
	logger.Info("Marusia_CRM: Завершение работы CRM сервиса")
	w.cancel()
}

func (w *CRM) StartOAuthStateCleanup(interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Minute
	}

	go func() {
		if w.crmService != nil {
			if err := w.crmService.CleanupExpiredOAuthStates(); err != nil {
				logger.Error("Ошибка очистки просроченных OAuth states: %v", err)
			}
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-w.ctx.Done():
				logger.Info("Marusia_CRM: остановка очистки OAuth states")
				return
			case <-ticker.C:
				if w.crmService == nil {
					continue
				}
				if err := w.crmService.CleanupExpiredOAuthStates(); err != nil {
					logger.Error("Ошибка очистки просроченных OAuth states: %v", err)
				}
			}
		}
	}()
}
