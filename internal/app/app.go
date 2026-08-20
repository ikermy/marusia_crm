package app

import (
	"Marusia_CRM/internal/crm"
	"Marusia_CRM/internal/db"
	"Marusia_CRM/internal/domain/state"
	"context"
	"fmt"
	"time"

	"github.com/ikermy/air_common/pkg/com"
	"github.com/ikermy/air_common/pkg/endpoint"
	"github.com/ikermy/air_common/pkg/rpc"
	"github.com/ikermy/air_logger/v2/pkg/logger"
)

const oauthStateCleanupInterval = 15 * time.Minute

type CRM interface {
	Handler() error
	Shutdown()
	StartOAuthStateCleanup(interval time.Duration)
}

type End interface {
	Shutdown(shutCh chan<- com.LogMsg)
}

type DB interface {
	HandlerClose()
}

type App struct {
	ctx    context.Context
	cancel context.CancelFunc
	crm    CRM
	db     DB
}

func New(parent context.Context) *App {
	// Локальный дочерний контекст для уровня app
	ctx, cancel := context.WithCancel(parent)

	rpcClient, err := rpc.New()
	if err != nil {
		logger.Fatal(fmt.Errorf("ошибка создания rpc клиента: %w", err))
	}

	d, err := db.New(ctx)
	if err != nil {
		logger.Fatal("Ошибка инициализации базы данных: %v", err)
	}

	// Инжектируем resolver в comdb.DB
	// Каждый раз когда DB-методу нужен MasterKey — он делает gRPC-запрос к Landing
	d.SetMasterKeyResolver(func(userId uint32) ([32]byte, bool) {
		mk, err := rpcClient.GetUserMasterKey(context.Background(), userId)
		if err != nil {
			// codes.Unavailable — пользователь не логинился после рестарта Landing
			// codes.Unauthenticated / PermissionDenied — неверный SERVICE_KEY
			return [32]byte{}, false
		}
		return mk, true
	})

	e := endpoint.New(ctx, d)
	c := crm.New(ctx, d, e)
	return &App{
		ctx:    ctx,
		cancel: cancel,
		crm:    c,
		db:     d,
	}
}

func (a *App) Run() {
	go func() {
		err := a.crm.Handler()
		if err != nil {
			//errCh <- err
			logger.Fatal(err)
		}
	}()

	// Слушаю сигнал завершения приложения
	go a.db.HandlerClose()

	go a.crm.StartOAuthStateCleanup(oauthStateCleanupInterval)

	// Обработка сигнала завершения
	go func() {
		<-a.ctx.Done()
		logger.Info("App: получен сигнал завершения, начинаю shutdown")
		a.crm.Shutdown()
		logger.Info("App: все модули завершены, закрываю соединение с БД")
		// Закрываю соединение с БД
		close(state.UsersDB)
	}()
}
