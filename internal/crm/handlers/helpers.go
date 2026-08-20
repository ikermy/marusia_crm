package handlers

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/ikermy/air_logger/v2/pkg/logger"
)

// GetUserID вспомогательная функция для получения userID из контекста
// Используется во всех handlers для извлечения userID, установленного middleware extractUserID
func GetUserID(c *fiber.Ctx) (uint32, bool) {
	userID, ok := c.Locals("user_id").(uint32)
	return userID, ok
}

// normalizeRedirectURL нормализует redirect_url для OAuth callback
// Выполняет следующие операции:
// 1. Удаляет дубликаты путей (если URL повторяется дважды)
// 2. Нормализует регистр: /amoCRM/ → /amocrm/
// 3. Валидирует формат URL (схема https://)
// 4. Удаляет trailing slashes
func normalizeRedirectURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}

	original := rawURL

	// Удаляем trailing slashes
	rawURL = strings.TrimRight(rawURL, "/")

	// Парсим URL для валидации
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		logger.Warn("normalizeRedirectURL: невалидный URL '%s': %v", rawURL, err)
		return rawURL // Возвращаем как есть, если не можем распарсить
	}

	// Проверяем, что схема - https
	if parsedURL.Scheme != "https" && parsedURL.Scheme != "http" {
		logger.Warn("normalizeRedirectURL: URL должен начинаться с https:// или http://, получен: %s", rawURL)
	}

	// Удаляем дубликаты путей в URL
	// Например: "https://example.com/crm/oauth/amoCRM/callback/crm/oauth/amoCRM/callback"
	// → "https://example.com/crm/oauth/amoCRM/callback"
	path := parsedURL.Path
	if path != "" && len(path) > 1 {
		// Разбиваем путь на сегменты
		segments := strings.Split(strings.Trim(path, "/"), "/")

		// Проверяем на дубликаты: если путь содержит одинаковые последовательности подряд
		segmentCount := len(segments)
		if segmentCount >= 2 && segmentCount%2 == 0 {
			// Проверяем, дублируется ли первая половина пути во второй половине
			halfLen := segmentCount / 2
			firstHalf := strings.Join(segments[:halfLen], "/")
			secondHalf := strings.Join(segments[halfLen:], "/")

			if firstHalf == secondHalf {
				// Найден дубликат - оставляем только первую половину
				path = "/" + firstHalf
				logger.Info("normalizeRedirectURL: удален дубликат пути в URL '%s' → '%s'", original, parsedURL.Scheme+"://"+parsedURL.Host+path)
			}
		}
	}

	// Нормализуем регистр для amoCRM: /amoCRM/ → /amocrm/
	// Используем регулярное выражение для замены /oauth/amoCRM/ на /oauth/amocrm/
	// Регистронезависимый поиск и замена
	re := regexp.MustCompile(`(?i)/oauth/amocrm/`)
	path = re.ReplaceAllString(path, "/oauth/amocrm/")

	// Собираем нормализованный URL обратно
	parsedURL.Path = path
	normalizedURL := parsedURL.String()

	// Логируем, если URL был изменен
	if normalizedURL != original {
		logger.Info("normalizeRedirectURL: URL нормализован: '%s' → '%s'", original, normalizedURL)
	}

	return normalizedURL
}
