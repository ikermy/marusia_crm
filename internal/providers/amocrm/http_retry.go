package amocrm

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/ikermy/air_logger/v2/pkg/logger"
)

// RetryConfig конфигурация для retry механизма
type RetryConfig struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
}

// DefaultRetryConfig возвращает конфигурацию по умолчанию
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 500 * time.Millisecond,
		MaxBackoff:     5 * time.Second,
	}
}

// isRetryableError проверяет, можно ли повторить запрос при данной ошибке
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Проверяем типы ошибок, которые стоит повторить
	var netErr net.Error
	if errors.As(err, &netErr) {
		// Временные сетевые ошибки и таймауты
		return netErr.Temporary() || netErr.Timeout()
	}

	// DNS ошибки могут быть временными
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return dnsErr.Temporary()
	}

	return false
}

// DoRequestWithRetry выполняет HTTP запрос с механизмом повторных попыток
// Обрабатывает сетевые ошибки и ошибки авторизации (401)
func (p *AmoCRMProvider) DoRequestWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
	config := DefaultRetryConfig()
	var lastErr error
	var lastResp *http.Response

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Вычисляем время ожидания с экспоненциальным backoff
			backoff := time.Duration(attempt) * config.InitialBackoff
			if backoff > config.MaxBackoff {
				backoff = config.MaxBackoff
			}

			logger.Debug("Retry попытка %d/%d после %v", attempt, config.MaxRetries, backoff)

			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		// Клонируем запрос для повторной попытки
		reqClone := req.Clone(ctx)

		resp, err := p.httpClient.Do(reqClone)
		if err == nil {
			// Проверяем статус код 401 - проблема с авторизацией
			if resp.StatusCode == http.StatusUnauthorized {
				logger.Error("AmoCRM вернул 401 Unauthorized (попытка %d/%d). "+
					"Токен устарел или недействителен. URL: %s, subdomain: %s",
					attempt+1, config.MaxRetries+1, req.URL.String(), p.subdomain)

				// Если это первая попытка, логируем предупреждение о необходимости обновления токена
				if attempt == 0 {
					logger.Warn("Кешированный провайдер использует устаревший токен. " +
						"Необходимо обновление токена и повторная инициализация провайдера.")
				}

				// Возвращаем ответ, чтобы вызывающая функция могла обработать ошибку
				lastResp = resp
				lastErr = fmt.Errorf("unauthorized (401): токен устарел или недействителен")

				// Не повторяем запрос при 401 - это не временная ошибка сети
				// Требуется обновление токена на уровне сервиса
				break
			}

			// Успешный ответ (или другая ошибка, не 401)
			return resp, nil
		}

		lastErr = err

		// Проверяем, можно ли повторить
		if !isRetryableError(err) {
			logger.Debug("Ошибка не подлежит повтору: %v", err)
			break
		}

		logger.Warn("Повторяемая ошибка при HTTP запросе (попытка %d/%d): %v", attempt+1, config.MaxRetries+1, err)
	}

	// Если есть lastResp с 401, возвращаем его
	if lastResp != nil && lastResp.StatusCode == http.StatusUnauthorized {
		return lastResp, lastErr
	}

	return nil, fmt.Errorf("все попытки исчерпаны: %w", lastErr)
}
