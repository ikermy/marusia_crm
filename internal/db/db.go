package db

import (
	"Marusia_CRM/internal/domain/state"
	"Marusia_CRM/internal/repository"
	"Marusia_CRM/internal/repository/mysql"
	"context"

	_ "github.com/go-sql-driver/mysql"
	"github.com/ikermy/air_common/pkg/comdb"
	"github.com/ikermy/air_logger/v2/pkg/logger"
)

// DB обёртка соединения с базой данных и репозиториями
type DB struct {
	*comdb.DB
	repo repository.Repository
}

// New создаёт подключение к БД и инициализирует репозитории
func New(parent context.Context) (*DB, error) {
	base, err := comdb.New(parent)
	if err != nil {
		return nil, err
	}
	repo, err := mysql.New(base)
	if err != nil {
		return nil, err
	}
	return &DB{
		DB:   base,
		repo: repo,
	}, nil
}

func (d *DB) HandlerClose() {
	go func() {
		// Получаю сигнал о завершении работы от главного контекста приложения
		<-d.MainCTX().Done()
		logger.Info("DB: контекст отменен, ожидаю завершения всех операций...")

		// Ожидаем сигнал о завершении от компонентов работающих с ДБ
		<-state.UsersDB
		logger.Info("DB: все модули работающие с БД завершили работу, продолжаю остановку...")

		if err := d.Close(); err != nil {
			logger.Error("DB: ошибка при закрытии: %v", err)
		}

		close(state.Exit)
	}()
}

// Repo возвращает набор репозиториев
func (d *DB) Repo() repository.Repository {
	return d.repo
}
