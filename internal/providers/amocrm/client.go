package amocrm

import (
	"Marusia_CRM/internal/domain/models"
	"Marusia_CRM/internal/metrics"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"time"

	"github.com/ikermy/air_logger/v2/pkg/logger"
)

// AmoCRMProvider реализует интерфейс CRMProvider для AmoCRM
// Работает через прямые HTTP запросы к AmoCRM API
type AmoCRMProvider struct {
	config        *models.CRMConfig
	credentials   *models.CRMCredentials
	subdomain     string       // Поддомен AmoCRM (например, "vsevdom")
	accessToken   string       // Токен доступа для прямых API запросов
	sourceFieldID int64        // ID поля "Источник перехода" для автоматического заполнения
	httpClient    *http.Client // HTTP клиент с таймаутами
}

var (
	// Регулярное выражение для валидации поддомена
	subdomainRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]{0,61}[a-zA-Z0-9]$`)
)

// NewAmoCRMProvider создает новый провайдер AmoCRM
func NewAmoCRMProvider() *AmoCRMProvider {
	return &AmoCRMProvider{}
}

// GetProviderType возвращает тип провайдера
func (p *AmoCRMProvider) GetProviderType() string {
	return "amocrm"
}

// Initialize инициализирует провайдер
func (p *AmoCRMProvider) Initialize(_ context.Context, userID uint32, config *models.CRMConfig, httpClient any) error {
	p.config = config

	// Валидация поддомена
	if config.Subdomain == "" {
		return fmt.Errorf("subdomain не может быть пустым")
	}
	if !subdomainRegex.MatchString(config.Subdomain) {
		return fmt.Errorf("некорректный формат subdomain: %s (должен содержать только латинские буквы, цифры и дефисы)", config.Subdomain)
	}

	// Парсим credentials
	var creds models.CRMCredentials
	if err := json.Unmarshal([]byte(config.Credentials), &creds); err != nil {
		return fmt.Errorf("ошибка парсинга credentials: %w", err)
	}

	// Проверяем наличие access_token
	if creds.AccessToken == "" {
		return fmt.Errorf("access_token отсутствует в credentials")
	}

	p.credentials = &creds

	// Сохраняем subdomain и accessToken для прямых API запросов
	p.subdomain = config.Subdomain
	p.accessToken = creds.AccessToken

	// Парсим options для получения field_id источника
	var options models.CRMConfigOptions
	if config.Options != "" {
		if err := json.Unmarshal([]byte(config.Options), &options); err != nil {
			logger.Warn("Не удалось распарсить options: %v", err, userID)
		} else {
			p.sourceFieldID = options.MarusiaSourceFieldID
		}
	}

	// Используем переданный HTTP клиент (единый для всего сервиса)
	if client, ok := httpClient.(*http.Client); ok && client != nil {
		p.httpClient = client
		logger.Debug("AmoCRM Provider использует общий HTTP клиент с keep-alive", userID)
	} else {
		// Fallback: создаем свой клиент
		p.httpClient = &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
				DisableKeepAlives:   false,
			},
		}
		logger.Warn("AmoCRM Provider создал собственный HTTP клиент (fallback)", userID)
	}

	logger.Debug("AmoCRM Provider инициализирован для subdomain=%s (прямые HTTP API запросы)", config.Subdomain, userID)
	logger.Debug("AmoCRM API URL: https://%s.amocrm.ru/api/v4/", config.Subdomain, userID)

	// Выполняем диагностику подключения
	if err := p.diagnoseConnection(userID, config.Subdomain); err != nil {
		logger.Warn("Диагностика подключения к AmoCRM выявила проблему: %v", err, userID)
		// Не прерываем инициализацию, просто предупреждаем
	}

	return nil
}

// ValidateCredentials проверяет корректность учетных данных
func (p *AmoCRMProvider) ValidateCredentials(_ context.Context) error {
	// Проверка через прямой API запрос реализована в CRMService.TestCRMConnection
	// Этот метод используется как fallback для других CRM
	logger.Debug("AmoCRM: ValidateCredentials вызван (используется TestCRMConnection)")
	return nil
}

// TestConnection проверяет подключение к AmoCRM через /api/v4/account
func (p *AmoCRMProvider) TestConnection(ctx context.Context) error {
	if p.httpClient == nil {
		return fmt.Errorf("провайдер не инициализирован")
	}

	// Делаем GET запрос к /api/v4/account
	apiURL := fmt.Sprintf("https://%s.amocrm.ru/api/v4/account", p.subdomain)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return fmt.Errorf("ошибка создания запроса: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.accessToken)
	req.Header.Set("Content-Type", "application/json")

	startedAt := time.Now()
	resp, err := p.httpClient.Do(req)
	if err != nil {
		metrics.ObserveProviderRequest("amocrm", req.Method, "error", startedAt)
		logger.Error("Ошибка при выполнении HTTP запроса к AmoCRM API (URL: %s): %v", apiURL, err)
		return fmt.Errorf("ошибка выполнения запроса к AmoCRM API (проверьте интернет-соединение и subdomain '%s'): %w", p.subdomain, err)
	}
	defer func() { _ = resp.Body.Close() }()

	statusLabel := fmt.Sprintf("%d", resp.StatusCode)
	metrics.ObserveProviderRequest("amocrm", req.Method, statusLabel, startedAt)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("AmoCRM вернул ошибку (status=%d)", resp.StatusCode)
	}

	// Парсим ответ для валидации
	var accountInfo map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&accountInfo); err != nil {
		return fmt.Errorf("ошибка чтения ответа: %w", err)
	}

	logger.Debug("AmoCRM подключение успешно: subdomain=%-v", accountInfo)
	return nil
}

// RefreshToken обновляет токен доступа
func (p *AmoCRMProvider) RefreshToken(_ context.Context) error {
	// Обновление токенов реализовано в CRMService через Lazy Refresh
	// Этот метод не используется для AmoCRM
	logger.Debug("AmoCRM: RefreshToken вызван (используется Lazy Refresh в CRMService)")
	return nil
}

// ParseWebhook парсит webhook от AmoCRM
func (p *AmoCRMProvider) ParseWebhook(_ context.Context, payload []byte) (any, error) {
	var webhook map[string]any
	if err := json.Unmarshal(payload, &webhook); err != nil {
		return nil, fmt.Errorf("ошибка парсинга webhook: %w", err)
	}
	return webhook, nil
}

// diagnoseConnection выполняет диагностику подключения к AmoCRM
func (p *AmoCRMProvider) diagnoseConnection(userID uint32, subdomain string) error {
	hostname := fmt.Sprintf("%s.amocrm.ru", subdomain)

	// 1. Проверка DNS
	logger.Debug("Диагностика AmoCRM: проверка DNS для %s", hostname, userID)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	addrs, err := net.DefaultResolver.LookupHost(ctx, hostname)
	if err != nil {
		logger.Error("Диагностика AmoCRM: ошибка DNS lookup для %s: %v", hostname, err)
		return fmt.Errorf("не удается разрешить доменное имя %s: %w (проверьте подключение к интернету и правильность subdomain)", hostname, err)
	}

	logger.Debug("Диагностика AmoCRM: DNS успешно разрешен, IP адреса: %v", addrs, userID)

	// 2. Проверка TCP подключения
	logger.Debug("Диагностика AmoCRM: проверка TCP подключения к %s:443", hostname, userID)
	conn, err := net.DialTimeout("tcp", hostname+":443", 10*time.Second)
	if err != nil {
		logger.Error("Диагностика AmoCRM: ошибка TCP подключения к %s:443: %v", hostname, err, userID)
		return fmt.Errorf("не удается установить TCP соединение с %s:443: %w (проверьте firewall и доступ к интернету)", hostname, err)
	}
	err = conn.Close()
	if err != nil {
		return err
	}

	logger.Debug("Диагностика AmoCRM: TCP подключение успешно", userID)
	logger.Debug("Диагностика AmoCRM: соединение с %s проверено успешно", hostname, userID)

	return nil
}
