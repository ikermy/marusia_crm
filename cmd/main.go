package main

import (
	"Marusia_CRM/internal/app"
	"Marusia_CRM/internal/domain/state"
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/ikermy/air_common/pkg/com"
	"github.com/ikermy/air_common/pkg/mode"
	"github.com/ikermy/air_logger/v2/pkg/logger"
)

func main() {
	logger.Debug(com.GetVersionInfo())

	// Инициализируем инфраструктурные переменные из env vars (порты, домен, TTL, логи).
	// Все значения имеют разумные дефолты; некорректные критичные — fatal.
	mode.InitFromEnv(logger.Fatalf)
	mode.SetTextMode(true)

	// Логгер: режим os.Stdout для Docker
	logSetup := logger.StdOut()
	logSetup.WithLogLevel(logSetup.FromString(mode.GetLogLevel()))
	logSetup.Apply()

	// TODO задел на когда нибудь потом
	// ── Redis ───────────────────────────────────────────────────────────────────
	//state.RedisAddr = os.Getenv("REDIS_ADDR")
	//state.RedisPassword = os.Getenv("REDIS_PASSWORD")
	//dbStr := os.Getenv("REDIS_DB")
	//if dbStr != "" {
	//	if n, err := strconv.Atoi(dbStr); err == nil {
	//		state.RedisDB = n
	//	}
	//}
	//if state.RedisAddr != "" {
	//	logger.Info("Redis: адрес=%s, db=%d", state.RedisAddr, state.RedisDB)
	//} else {
	//	logger.Info("Redis: не настроен (REDIS_ADDR пуст)")
	//}

	// Корневой контекст процесса, отменяется по сигналам ОС
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	a := app.New(ctx)
	a.Run()

	// Ожидание завершения работы
	<-state.Exit

	logger.Infoln("Приложение air_crm завершено")
}
