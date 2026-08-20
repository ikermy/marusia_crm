package service

import (
	"Marusia_CRM/internal/cache"
	"Marusia_CRM/internal/domain/models"
	"Marusia_CRM/internal/providers"
	"Marusia_CRM/internal/repository"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ikermy/air_logger/v2/pkg/logger"
)

// AmoCRMTokens структура ответа от AmoCRM при обмене кода на токены
// и при обновлении токена.
type AmoCRMTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	ExpiresAt    int64  `json:"expires_at"`
}

// CRMService бизнес-логика для работы с CRM
type CRMService struct {
	repo       repository.Repository
	registry   *providers.Registry
	stateCache *cache.OAuthStateCache
	httpClient *http.Client
}

// NewCRMService создает новый сервис
func NewCRMService(repo repository.Repository, registry *providers.Registry) *CRMService {
	// Создаем единый HTTP клиент с keep-alive для всех провайдеров
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
			DisableKeepAlives:   false,
		},
	}

	// Передаем HTTP клиент в Registry для использования провайдерами
	registry.SetHTTPClient(httpClient)

	return &CRMService{
		repo:       repo,
		registry:   registry,
		stateCache: cache.NewOAuthStateCache(),
		httpClient: httpClient,
	}
}

// ============= CRM Configs =============

func (s *CRMService) CreateCRMConfig(config *models.CRMConfig) error {
	// Валидация
	if config.UserID == 0 || config.CRMType == "" || config.Name == "" {
		return fmt.Errorf("обязательные поля не заполнены")
	}

	return s.repo.Internal.CreateCRMConfig(config)
}

// UpsertCRMConfig создает или обновляет CRM конфигурацию
func (s *CRMService) UpsertCRMConfig(config *models.CRMConfig) error {
	// Валидация
	if config.UserID == 0 || config.CRMType == "" || config.Name == "" {
		return fmt.Errorf("обязательные поля не заполнены")
	}

	// Инвалидируем кэш провайдера
	s.registry.RemoveProvider(config.UserID, config.CRMType)

	return s.repo.Internal.UpsertCRMConfig(config)
}

func (s *CRMService) GetCRMConfig(id uint32) (*models.CRMConfig, error) {
	return s.repo.Internal.GetCRMConfig(id)
}

func (s *CRMService) GetUserCRMConfigs(userID uint32) ([]models.CRMConfig, error) {
	return s.repo.Internal.GetUserCRMConfigs(userID)
}

func (s *CRMService) UpdateCRMConfig(config *models.CRMConfig) error {
	// Инвалидируем кэш провайдера
	s.registry.RemoveProvider(config.UserID, config.CRMType)
	return s.repo.Internal.UpdateCRMConfig(config)
}

func (s *CRMService) DeleteCRMConfig(id uint32) error {
	// Получаем конфиг для инвалидации кэша
	config, err := s.repo.Internal.GetCRMConfig(id)
	if err == nil {
		s.registry.RemoveProvider(config.UserID, config.CRMType)
	}
	return s.repo.Internal.DeleteCRMConfig(id)
}

// GetCRMConfigByType получает CRM конфигурацию по user_id и crm_type (БЕЗ токенов - для API)
func (s *CRMService) GetCRMConfigByType(userID uint32, crmType string) (*models.CRMConfig, error) {
	return s.repo.Internal.GetCRMConfigByType(userID, crmType)
}

// GetCRMConfigByTypeInternal получает CRM конфигурацию с полными credentials (С токенами - для внутреннего использования)
func (s *CRMService) GetCRMConfigByTypeInternal(userID uint32, crmType string) (*models.CRMConfig, error) {
	return s.repo.Internal.GetCRMConfigByTypeInternal(userID, crmType)
}

// DeleteCRMConfigByType удаляет CRM конфигурацию по user_id и crm_type
func (s *CRMService) DeleteCRMConfigByType(userID uint32, crmType string) error {
	// Получаем конфиг для получения ID
	config, err := s.repo.Internal.GetCRMConfigByType(userID, crmType)
	if err != nil {
		return err
	}

	// Инвалидируем кэш провайдера
	s.registry.RemoveProvider(userID, crmType)

	return s.repo.Internal.DeleteCRMConfig(config.ID)
}

// AmoCRMAccountInfo структура ответа от /api/v4/account
type AmoCRMAccountInfo struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Subdomain string `json:"subdomain"`
	CreatedAt int64  `json:"created_at"`
	Country   string `json:"country"`
	Currency  string `json:"currency"`
}

// TestCRMConnection проверяет подключение к CRM через /api/v4/account
func (s *CRMService) TestCRMConnection(ctx context.Context, userID uint32, configID uint32) error {
	// Используем Internal метод для получения полных credentials с токенами
	config, err := s.repo.Internal.GetCRMConfigInternal(configID)
	if err != nil {
		return fmt.Errorf("конфигурация не найдена: %w", err)
	}

	// Для AmoCRM проверяем и обновляем токен при необходимости (Lazy Refresh)
	if config.CRMType == "amocrm" {
		if err := s.ensureValidToken(ctx, config); err != nil {
			return err
		}

		// Перечитываем config на случай обновления токена
		config, err = s.repo.Internal.GetCRMConfigInternal(configID)
		if err != nil {
			return fmt.Errorf("ошибка получения обновленной конфигурации: %w", err)
		}
	}

	// Используем единый подход через провайдера для всех CRM
	provider, err := s.registry.GetOrCreateProvider(ctx, userID, config)
	if err != nil {
		return fmt.Errorf("ошибка создания провайдера: %w", err)
	}

	return provider.TestConnection(ctx)
}

// GetAmoCRMAccountInfo получает информацию об аккаунте AmoCRM для возврата клиенту
func (s *CRMService) GetAmoCRMAccountInfo(ctx context.Context, userID uint32, crmType string) (*AmoCRMAccountInfo, error) {
	// Получаем конфигурацию и автоматически обновляем токен при необходимости (Lazy Refresh)
	config, err := s.ensureValidTokenByType(ctx, userID, crmType)
	if err != nil {
		return nil, err
	}

	if config.CRMType != "amocrm" {
		return nil, fmt.Errorf("конфигурация не является AmoCRM")
	}

	// Парсим credentials (токен уже проверен и обновлен если нужно)
	var creds models.CRMCredentials
	if err := json.Unmarshal([]byte(config.Credentials), &creds); err != nil {
		return nil, fmt.Errorf("ошибка парсинга credentials: %w", err)
	}

	// Делаем GET запрос к /api/v4/account
	apiURL := fmt.Sprintf("https://%s.amocrm.ru/api/v4/account", config.Subdomain)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		logger.Error("Ошибка при выполнении HTTP запроса к AmoCRM API (URL: %s): %v", apiURL, err)
		return nil, fmt.Errorf("ошибка выполнения запроса к AmoCRM API (проверьте интернет-соединение и subdomain '%s'): %w", config.Subdomain, err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Читаем ответ
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AmoCRM вернул ошибку (%d): %s", resp.StatusCode, string(body))
	}

	// Парсим ответ
	var accountInfo AmoCRMAccountInfo
	if err := json.Unmarshal(body, &accountInfo); err != nil {
		return nil, fmt.Errorf("ошибка парсинга ответа: %w", err)
	}

	logger.Debug("Получена информация об аккаунте amoCRM: ID=%d, название=%s", accountInfo.ID, accountInfo.Name, userID)

	return &accountInfo, nil
}

// ============= Lead Operations =============

// CreateLeadForUser создает лид в CRM на основе маппингов пользователя
func (s *CRMService) CreateLeadForUser(ctx context.Context, userID uint32, appID uint32, leadData *models.Lead) error {
	startTime := time.Now()

	// Получаем активные маппинги для этого приложения
	mappings, err := s.repo.Internal.GetActiveMappings(userID, appID)
	if err != nil {
		return fmt.Errorf("ошибка получения маппингов: %w", err)
	}

	if len(mappings) == 0 {
		return fmt.Errorf("не найдено активных маппингов для userID=%d, appID=%d", userID, appID)
	}

	// Для каждого маппинга отправляем лид в соответствующую CRM
	var lastError error
	successCount := 0

	for _, mapping := range mappings {
		// Получаем CRM конфиг (используем Internal для получения полных credentials с токенами)
		config, err := s.repo.Internal.GetCRMConfigInternal(mapping.CRMConfigID)
		if err != nil {
			logger.Error("Ошибка получения CRM конфига ID=%d для userID=%d: %v", mapping.CRMConfigID, userID, err)
			lastError = err
			continue
		}

		if !config.IsActive {
			logger.Debug("CRM конфиг ID=%d неактивен, пропускаю", config.ID)
			continue
		}

		// Проверяем и обновляем токен при необходимости (Lazy Refresh)
		if err := s.ensureValidToken(ctx, config); err != nil {
			logger.Error("Ошибка проверки/обновления токена для CRM %s: userID=%d, crmConfigID=%d, err=%v", config.CRMType, userID, config.ID, err)
			lastError = err
			continue
		}

		// Перечитываем config после возможного обновления токена (используем Internal)
		config, err = s.repo.Internal.GetCRMConfigInternal(mapping.CRMConfigID)
		if err != nil {
			logger.Error("Ошибка повторного получения CRM конфига ID=%d для userID=%d: %v", mapping.CRMConfigID, userID, err)
			lastError = err
			continue
		}

		// Получаем или создаем провайдер
		provider, err := s.registry.GetOrCreateProvider(ctx, userID, config)
		if err != nil {
			logger.Error("Ошибка создания провайдера для CRM %s: userID=%d, crmConfigID=%d, err=%v", config.CRMType, userID, config.ID, err)
			lastError = err
			continue
		}

		// Применяем правила маппинга полей
		mappedLead := s.applyLeadMapping(leadData, &mapping)

		// Сериализуем для лога
		requestJSON, _ := json.Marshal(mappedLead)

		// Создаем лид
		createdLead, err := provider.CreateLead(ctx, mappedLead)
		duration := int(time.Since(startTime).Milliseconds())

		if err != nil {
			logger.Error("Ошибка создания лида в CRM %s: userID=%d, crmConfigID=%d, request=%s, err=%v", config.CRMType, userID, config.ID, string(requestJSON), err)
			lastError = err
			continue
		}

		// Успешно создано
		responseJSON, _ := json.Marshal(createdLead)
		logger.Info("Лид успешно создан в CRM %s: userID=%d, crmConfigID=%d, request=%s, response=%s, duration_ms=%d", config.CRMType, userID, config.ID, string(requestJSON), string(responseJSON), duration)
		successCount++
		logger.Debug("Лид успешно создан в CRM %s (ID: %s)", config.CRMType, createdLead.ID)
	}

	if successCount == 0 && lastError != nil {
		return fmt.Errorf("не удалось создать лид ни в одной CRM: %w", lastError)
	}

	return nil
}

// applyLeadMapping применяет правила маппинга к лиду
func (s *CRMService) applyLeadMapping(lead *models.Lead, mapping *models.CRMMapping) *models.Lead {
	mappedLead := *lead // копируем

	// Устанавливаем параметры из маппинга
	if mapping.PipelineID != "" {
		mappedLead.PipelineID = mapping.PipelineID
	}
	if mapping.StatusID != "" {
		mappedLead.StatusID = mapping.StatusID
	}
	if mapping.ResponsibleUserID != "" {
		mappedLead.ResponsibleUser = mapping.ResponsibleUserID
	}

	// TODO: Применить field mapping для custom fields

	return &mappedLead
}

// ============= Contact Operations =============

// FindContactByPhone ищет контакт по номеру телефона в CRM пользователя
func (s *CRMService) FindContactByPhone(ctx context.Context, userID uint32, crmType string, phoneNumber string) (*models.Contact, error) {
	// Получаем конфигурацию и автоматически обновляем токен при необходимости (Lazy Refresh)
	config, err := s.ensureValidTokenByType(ctx, userID, crmType)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения конфигурации: %w", err)
	}

	// Получаем или создаем провайдер
	provider, err := s.registry.GetOrCreateProvider(ctx, userID, config)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания провайдера: %w", err)
	}

	// Ищем контакт
	contact, err := provider.FindContactByPhone(ctx, userID, phoneNumber)
	if err != nil {
		return nil, fmt.Errorf("ошибка поиска контакта: %w", err)
	}

	return contact, nil
}

// FindContactByAltContact ищет контакт по альтернативному контакту в CRM пользователя
func (s *CRMService) FindContactByAltContact(ctx context.Context, userID uint32, crmType string, altContact string) (*models.Contact, error) {
	// Получаем конфигурацию и автоматически обновляем токен при необходимости (Lazy Refresh)
	config, err := s.ensureValidTokenByType(ctx, userID, crmType)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения конфигурации: %w", err)
	}

	// Получаем или создаем провайдер
	provider, err := s.registry.GetOrCreateProvider(ctx, userID, config)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания провайдера: %w", err)
	}

	// Ищем контакт по альтернативному контакту
	contact, err := provider.FindContactByAltContact(ctx, userID, altContact)
	if err != nil {
		return nil, fmt.Errorf("ошибка поиска контакта: %w", err)
	}

	return contact, nil
}

// CreateContact создает новый контакт в CRM пользователя
func (s *CRMService) CreateContact(ctx context.Context, userID uint32, crmType string, contact *models.Contact) (*models.Contact, error) {
	// Получаем конфигурацию и автоматически обновляем токен при необходимости (Lazy Refresh)
	config, err := s.ensureValidTokenByType(ctx, userID, crmType)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения конфигурации: %w", err)
	}

	// Получаем или создаем провайдер
	provider, err := s.registry.GetOrCreateProvider(ctx, userID, config)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания провайдера: %w", err)
	}

	createdContact, err := provider.CreateContact(ctx, userID, contact)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания контакта: %w", err)
	}

	// Для AmoCRM автоматически добавляем источник "MarusiaAI" если есть тег MarusiaAI
	if crmType == "amocrm" {
		hasMarusiaTag := false
		for _, tag := range contact.Tags {
			if tag == "MarusiaAI" {
				hasMarusiaTag = true
				break
			}
		}

		if hasMarusiaTag {
			// Добавляем источник (не критично, если не получится)
			if err := provider.SetMarusiaSource(ctx, userID, createdContact.ID); err != nil {
				logger.Warn("Не удалось добавить источник MarusiaAI для контакта ID=%s: %v", createdContact.ID, err, userID)
			}
		}
	}

	return createdContact, nil
}

// GetContact получает контакт по ID из CRM пользователя
func (s *CRMService) GetContact(ctx context.Context, userID uint32, crmType string, contactID string) (*models.Contact, error) {
	// Получаем конфигурацию и автоматически обновляем токен при необходимости (Lazy Refresh)
	config, err := s.ensureValidTokenByType(ctx, userID, crmType)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения конфигурации: %w", err)
	}

	// Получаем или создаем провайдер
	provider, err := s.registry.GetOrCreateProvider(ctx, userID, config)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания провайдера: %w", err)
	}

	// Получаем контакт
	contact, err := provider.GetContact(ctx, userID, contactID)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения контакта: %w", err)
	}

	return contact, nil
}

// UpdateContact обновляет контакт в CRM пользователя
func (s *CRMService) UpdateContact(ctx context.Context, userID uint32, crmType string, contactID string, contact *models.Contact) (*models.Contact, error) {
	// Получаем конфигурацию и автоматически обновляем токен при необходимости (Lazy Refresh)
	config, err := s.ensureValidTokenByType(ctx, userID, crmType)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения конфигурации: %w", err)
	}

	// Получаем или создаем провайдер
	provider, err := s.registry.GetOrCreateProvider(ctx, userID, config)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания провайдера: %w", err)
	}

	// Обновляем контакт
	updatedContact, err := provider.UpdateContact(ctx, userID, contactID, contact)
	if err != nil {
		return nil, fmt.Errorf("ошибка обновления контакта: %w", err)
	}

	return updatedContact, nil
}

// ============= Lazy Token Refresh =============

// ensureValidToken проверяет актуальность токена и обновляет его при необходимости
// Используется принцип "Lazy Refresh" - токен обновляется непосредственно перед использованием
func (s *CRMService) ensureValidToken(ctx context.Context, config *models.CRMConfig) error {
	// Проверяем только для AmoCRM (другие CRM могут иметь другую логику)
	if config.CRMType != "amocrm" {
		return nil
	}

	// Парсим credentials
	var creds models.CRMCredentials
	if err := json.Unmarshal([]byte(config.Credentials), &creds); err != nil {
		return fmt.Errorf("ошибка парсинга credentials: %w", err)
	}

	// Если нет access_token - авторизация не пройдена
	if creds.AccessToken == "" {
		return fmt.Errorf("отсутствует access_token - необходима OAuth авторизация")
	}

	// Проверяем, не истекает ли токен в ближайшие 5 минут (300 секунд)
	// Обновляем заранее, чтобы избежать 401 ошибки во время операции
	const refreshThreshold = 300 // 5 минут - порог для обновления токена
	const warningThreshold = 600 // 10 минут - порог для инвалидации кеша провайдера
	currentTime := time.Now().Unix()

	tokenNeedsRefresh := currentTime >= (creds.ExpiresAt - refreshThreshold)
	tokenExpiresSoon := currentTime >= (creds.ExpiresAt - warningThreshold)

	// КРИТИЧЕСКИ ВАЖНО: Превентивно инвалидируем кеш провайдера если токен скоро истечет
	// Это защищает от использования кешированного провайдера со старым токеном
	// даже если токен еще формально валиден по времени
	if tokenExpiresSoon {
		s.registry.RemoveProvider(config.UserID, config.CRMType)
		logger.Debug("Кеш провайдера превентивно инвалидирован - токен истекает менее чем через 10 минут "+
			"(expires_at=%d, current=%d, осталось %d сек, user_id=%d, crm_type=%s)",
			creds.ExpiresAt, currentTime, creds.ExpiresAt-currentTime, config.UserID, config.CRMType)
	}

	// Обновляем токен если он реально истекает в ближайшие 5 минут
	if tokenNeedsRefresh {
		logger.Info("Токен истекает (expires_at=%d, current=%d, осталось %d сек), выполняется автообновление...",
			creds.ExpiresAt, currentTime, creds.ExpiresAt-currentTime)

		// Обновляем токен
		_, err := s.RefreshAmoCRMToken(ctx, config.ID)
		if err != nil {
			return fmt.Errorf("не удалось обновить токен: %w", err)
		}

		logger.Info("Токен успешно обновлен автоматически для config_id=%d", config.ID)
	}

	return nil
}

// ensureValidTokenByType проверяет и обновляет токен по userID + crmType
// Удобная обертка для случаев, когда у нас нет config, но есть userID и crmType
func (s *CRMService) ensureValidTokenByType(ctx context.Context, userID uint32, crmType string) (*models.CRMConfig, error) {
	// Используем Internal метод для получения полных credentials с токенами
	config, err := s.repo.Internal.GetCRMConfigByTypeInternal(userID, crmType)
	if err != nil {
		return nil, fmt.Errorf("конфигурация не найдена: %w", err)
	}

	// Проверяем и обновляем токен при необходимости
	// ensureValidToken автоматически инвалидирует кеш провайдера если токен был обновлен
	if err := s.ensureValidToken(ctx, config); err != nil {
		return nil, err
	}

	// Возвращаем конфигурацию (возможно с обновленным токеном)
	// Перечитываем из БД на случай, если токен был обновлен (используем Internal)
	return s.repo.Internal.GetCRMConfigByTypeInternal(userID, crmType)
}

// ============= OAuth State Management =============

// SaveOAuthState сохраняет state в БД и кеше
func (s *CRMService) SaveOAuthState(state *models.OAuthState) error {
	// Сохраняем в кеш (быстрый доступ)
	s.stateCache.Set(state)

	// Сохраняем в БД (персистентность)
	if err := s.repo.Internal.SaveOAuthState(state); err != nil {
		logger.Error("Ошибка сохранения OAuth state в БД: %v", err)
		// Продолжаем работу, так как state уже в кеше
	}

	return nil
}

// GetOAuthState получает state из кеша или БД
func (s *CRMService) GetOAuthState(stateKey string) (*models.OAuthState, error) {
	// Сначала проверяем кеш
	if state, found := s.stateCache.Get(stateKey); found {
		logger.Debug("OAuth state найден в кеше: %s", stateKey)
		return state, nil
	}

	// Если нет в кеше, проверяем БД
	state, err := s.repo.Internal.GetOAuthState(stateKey)
	if err != nil {
		return nil, fmt.Errorf("OAuth state не найден: %w", err)
	}

	// Добавляем в кеш для следующих запросов
	s.stateCache.Set(state)
	logger.Debug("OAuth state найден в БД и добавлен в кеш: %s", stateKey)

	return state, nil
}

// DeleteOAuthState удаляет state из кеша и БД
func (s *CRMService) DeleteOAuthState(stateKey string) error {
	// Удаляем из кеша
	s.stateCache.Delete(stateKey)

	// Удаляем из БД
	if err := s.repo.Internal.DeleteOAuthState(stateKey); err != nil {
		return fmt.Errorf("ошибка удаления OAuth state из БД: %v", err)
	}

	return nil
}

// CleanupExpiredOAuthStates очищает истекшие states из БД
func (s *CRMService) CleanupExpiredOAuthStates() error {
	return s.repo.Internal.CleanupExpiredOAuthStates()
}

// GetCustomFields получает список пользовательских полей для указанного типа сущности
func (s *CRMService) GetCustomFields(ctx context.Context, userID uint32, crmType string, entityType string) (*models.CustomFieldsMetadataResponse, error) {
	// Получаем конфигурацию и автоматически обновляем токен при необходимости
	config, err := s.ensureValidTokenByType(ctx, userID, crmType)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения конфигурации: %w", err)
	}

	// Получаем или создаем провайдер
	provider, err := s.registry.GetOrCreateProvider(ctx, userID, config)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания провайдера: %w", err)
	}

	// Получаем custom fields
	fields, err := provider.GetCustomFields(ctx, userID, entityType)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения custom fields: %w", err)
	}

	return fields, nil
}

// CreateCustomField создает новое пользовательское поле
func (s *CRMService) CreateCustomField(ctx context.Context, userID uint32, crmType string, entityType string, fieldData any) (any, error) {
	// Получаем конфигурацию и автоматически обновляем токен при необходимости
	config, err := s.ensureValidTokenByType(ctx, userID, crmType)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения конфигурации: %w", err)
	}

	// Получаем или создаем провайдер
	provider, err := s.registry.GetOrCreateProvider(ctx, userID, config)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания провайдера: %w", err)
	}

	// Проверяем, что провайдер поддерживает создание custom fields
	type CustomFieldCreator interface {
		CreateCustomField(ctx context.Context, entityType string, fieldData any) (any, error)
	}

	creator, ok := provider.(CustomFieldCreator)
	if !ok {
		return nil, fmt.Errorf("провайдер %s не поддерживает создание пользовательских полей", crmType)
	}

	// Создаем custom field
	field, err := creator.CreateCustomField(ctx, entityType, fieldData)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания custom field: %w", err)
	}

	return field, nil
}

// SetMarusiaSourceFieldID устанавливает ID поля "Источник перехода" для автоматического использования
func (s *CRMService) SetMarusiaSourceFieldID(userID uint32, crmType string, fieldID int64) error {
	// Получаем конфигурацию
	config, err := s.repo.Internal.GetCRMConfigByTypeInternal(userID, crmType)
	if err != nil {
		return fmt.Errorf("конфигурация не найдена: %w", err)
	}

	// Парсим существующие options или создаем новые
	var options models.CRMConfigOptions
	if config.Options != "" {
		if err := json.Unmarshal([]byte(config.Options), &options); err != nil {
			logger.Warn("Не удалось распарсить options, создаем новые: %v", err)
			options = models.CRMConfigOptions{}
		}
	}

	// Устанавливаем field ID в options
	options.MarusiaSourceFieldID = fieldID
	// Маршалим options в JSON
	optionsJSON, err := json.Marshal(options)
	if err != nil {
		return fmt.Errorf("ошибка сериализации options: %w", err)
	}

	config.Options = string(optionsJSON)

	// Сохраняем обновленную конфигурацию
	if err := s.repo.Internal.UpdateCRMConfig(config); err != nil {
		return fmt.Errorf("ошибка сохранения field_id: %w", err)
	}

	// Инвалидируем кэш провайдера
	s.registry.RemoveProvider(userID, crmType)

	logger.Info("Field ID источника MarusiaAI установлен: userID=%d, crmType=%s, fieldID=%d", userID, crmType, fieldID)
	return nil
}

// SetDefaultLeadSettings устанавливает pipeline_id и status_id по умолчанию для UpdateLead
func (s *CRMService) SetDefaultLeadSettings(userID uint32, crmType string, pipelineID int64, statusID int64) error {
	// Получаем конфигурацию
	config, err := s.repo.Internal.GetCRMConfigByTypeInternal(userID, crmType)
	if err != nil {
		return fmt.Errorf("конфигурация не найдена: %w", err)
	}

	// Парсим существующие options или создаем новые
	var options models.CRMConfigOptions
	if config.Options != "" {
		if err := json.Unmarshal([]byte(config.Options), &options); err != nil {
			logger.Warn("Не удалось распарсить options, создаем новые: %v", err)
			options = models.CRMConfigOptions{}
		}
	}
	// Устанавливаем pipeline ID и status ID в options
	options.DefaultLeadPipelineID = pipelineID
	options.DefaultLeadStatusID = statusID

	// Маршалим options в JSON
	optionsJSON, err := json.Marshal(options)
	if err != nil {
		return fmt.Errorf("ошибка сериализации options: %w", err)
	}

	config.Options = string(optionsJSON)

	// Сохраняем обновленную конфигурацию
	if err := s.repo.Internal.UpdateCRMConfig(config); err != nil {
		return fmt.Errorf("ошибка сохранения настроек лида: %w", err)
	}

	// Инвалидируем кэш провайдера
	s.registry.RemoveProvider(userID, crmType)

	logger.Info("Настройки лида по умолчанию установлены: userID=%d, crmType=%s, pipelineID=%d, statusID=%d",
		userID, crmType, pipelineID, statusID)
	return nil
}

// SetChannelSettings сохраняет настройки каналов для указанного CRM типа
func (s *CRMService) SetChannelSettings(userID uint32, crmType string, settings *models.CRMChannelSettings) error {
	// Получаем конфигурацию
	config, err := s.repo.Internal.GetCRMConfigByTypeInternal(userID, crmType)
	if err != nil {
		return fmt.Errorf("конфигурация не найдена: %w", err)
	}

	// Парсим существующие channels или создаем новые
	var allChannels map[string]models.CRMChannelSettings
	if config.Chanells != "" {
		if err := json.Unmarshal([]byte(config.Chanells), &allChannels); err != nil {
			logger.Warn("Не удалось распарсить channels, создаем новые: %v", err)
			allChannels = make(map[string]models.CRMChannelSettings)
		}
	} else {
		allChannels = make(map[string]models.CRMChannelSettings)
	}

	// Устанавливаем настройки для конкретного CRM типа
	allChannels[crmType] = *settings

	// Маршалим channels в JSON
	channelsJSON, err := json.Marshal(allChannels)
	if err != nil {
		return fmt.Errorf("ошибка сериализации channels: %w", err)
	}

	config.Chanells = string(channelsJSON)

	// Сохраняем обновленную конфигурацию
	if err := s.repo.Internal.UpdateCRMConfig(config); err != nil {
		return fmt.Errorf("ошибка сохранения настроек каналов: %w", err)
	}

	// Инвалидируем кэш провайдера
	s.registry.RemoveProvider(userID, crmType)

	logger.Info("Настройки каналов установлены: userID=%d, crmType=%s", userID, crmType)
	return nil
}

// CreateAIDialogLead создает лид по contactID и имени с опциональными тегами
func (s *CRMService) CreateAIDialogLead(ctx context.Context, userID uint32, crmType string, contactID string, leadName string, tags []string) (*models.Lead, error) {
	config, err := s.ensureValidTokenByType(ctx, userID, crmType)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения конфигурации: %w", err)
	}
	provider, err := s.registry.GetOrCreateProvider(ctx, userID, config)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания провайдера: %w", err)
	}
	if crmType != "amocrm" {
		return nil, fmt.Errorf("метод поддерживается только для AmoCRM")
	}

	lead, err := provider.CreateAIDialogLead(ctx, contactID, leadName, tags, userID)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания лида: %w", err)
	}
	return lead, nil
}

// GetLead получает лид по ID из CRM
func (s *CRMService) GetLead(ctx context.Context, userID uint32, crmType string, leadID string) (*models.Lead, error) {
	config, err := s.ensureValidTokenByType(ctx, userID, crmType)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения конфигурации: %w", err)
	}

	provider, err := s.registry.GetOrCreateProvider(ctx, userID, config)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания провайдера: %w", err)
	}

	lead, err := provider.GetLead(ctx, leadID)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения лида: %w", err)
	}

	return lead, nil
}

// UpdateLead обновляет лид в CRM
func (s *CRMService) UpdateLead(ctx context.Context, userID uint32, crmType string, leadID string) (*models.Lead, error) {
	config, err := s.ensureValidTokenByType(ctx, userID, crmType)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения конфигурации: %w", err)
	}

	// Парсим options для получения настроек по умолчанию
	var options models.CRMConfigOptions
	if config.Options != "" {
		if err := json.Unmarshal([]byte(config.Options), &options); err != nil {
			return nil, fmt.Errorf("ошибка парсинга options: %w", err)
		}
	}

	// Проверяем наличие обязательных настроек
	if options.DefaultLeadPipelineID == 0 || options.DefaultLeadStatusID == 0 {
		return nil, fmt.Errorf("не настроены default_lead_pipeline_id и default_lead_status_id в конфигурации CRM")
	}

	// Создаем объект лида с настройками из БД
	lead := &models.Lead{
		PipelineID: fmt.Sprintf("%d", options.DefaultLeadPipelineID),
		StatusID:   fmt.Sprintf("%d", options.DefaultLeadStatusID),
	}

	logger.Debug("Обновление лида с настройками из БД: pipelineID=%s, statusID=%s", lead.PipelineID, lead.StatusID, userID)

	provider, err := s.registry.GetOrCreateProvider(ctx, userID, config)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания провайдера: %w", err)
	}

	updatedLead, err := provider.UpdateLead(ctx, leadID, lead, userID)
	if err != nil {
		return nil, fmt.Errorf("ошибка обновления лида: %w", err)
	}

	return updatedLead, nil
}

// GetLeadsByContactID получает все лиды связанные с контактом
func (s *CRMService) GetLeadsByContactID(ctx context.Context, userID uint32, crmType string, contactID string) ([]*models.Lead, error) {
	config, err := s.ensureValidTokenByType(ctx, userID, crmType)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения конфигурации: %w", err)
	}

	provider, err := s.registry.GetOrCreateProvider(ctx, userID, config)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания провайдера: %w", err)
	}

	leads, err := provider.GetLeadsByContactID(ctx, contactID, userID)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения лидов: %w", err)
	}

	return leads, nil
}

// GetTalk получает беседу по ID из CRM
func (s *CRMService) GetTalk(ctx context.Context, userID uint32, crmType string, talkID string) (*models.Talk, error) {
	config, err := s.ensureValidTokenByType(ctx, userID, crmType)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения конфигурации: %w", err)
	}

	provider, err := s.registry.GetOrCreateProvider(ctx, userID, config)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания провайдера: %w", err)
	}

	talk, err := provider.GetTalk(ctx, talkID)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения беседы: %w", err)
	}

	return talk, nil
}

// CreateLeadNote создает примечание для лида в CRM
func (s *CRMService) CreateLeadNote(ctx context.Context, userID uint32, crmType string, leadID string, note *models.CreateNoteRequest) (*models.Note, error) {
	config, err := s.ensureValidTokenByType(ctx, userID, crmType)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения конфигурации: %w", err)
	}

	provider, err := s.registry.GetOrCreateProvider(ctx, userID, config)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания провайдера: %w", err)
	}

	createdNote, err := provider.CreateLeadNote(ctx, leadID, note, userID)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания примечания: %w", err)
	}

	return createdNote, nil
}

// GetPipelines получает список воронок и статусов из CRM
func (s *CRMService) GetPipelines(ctx context.Context, userID uint32, crmType string) (map[string]any, error) {
	config, err := s.ensureValidTokenByType(ctx, userID, crmType)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения конфигурации: %w", err)
	}

	provider, err := s.registry.GetOrCreateProvider(ctx, userID, config)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания провайдера: %w", err)
	}

	pipelines, err := provider.GetPipelines(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения воронок: %w", err)
	}

	// Парсим options для получения default_pipeline_id
	var options models.CRMConfigOptions
	if config.Options != "" {
		if err := json.Unmarshal([]byte(config.Options), &options); err != nil {
			// Если не удалось распарсить, логируем ошибку, но продолжаем работу
			logger.Error("Не удалось распарсить options для получения default_pipeline_id: %v", err, userID)
		}
	}

	// Формируем ответ с воронками и default_pipeline_id
	result := map[string]any{
		"pipelines":           pipelines,
		"default_pipeline_id": options.DefaultPipelineID,
	}

	return result, nil
}

// ExchangeAmoCRMCode обменивает код авторизации на токены
func (s *CRMService) ExchangeAmoCRMCode(ctx context.Context, code, clientID, clientSecret, redirectURI, subdomain string) (*AmoCRMTokens, error) {
	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("https://%s.amocrm.ru/oauth2/access_token", subdomain), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса обмена кода: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения запроса обмена кода: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа обмена кода: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AmoCRM вернул ошибку при обмене кода (%d): %s", resp.StatusCode, string(body))
	}

	var tokens AmoCRMTokens
	if err := json.Unmarshal(body, &tokens); err != nil {
		return nil, fmt.Errorf("ошибка парсинга ответа обмена кода: %w", err)
	}

	if tokens.AccessToken == "" {
		return nil, fmt.Errorf("AmoCRM не вернул access_token")
	}

	if tokens.ExpiresIn > 0 {
		tokens.ExpiresAt = time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second).Unix()
	}
	if tokens.TokenType == "" {
		tokens.TokenType = "Bearer"
	}

	return &tokens, nil
}

// RefreshAmoCRMToken обновляет access/refresh токены AmoCRM через OAuth endpoint
func (s *CRMService) RefreshAmoCRMToken(ctx context.Context, configID uint32) (*AmoCRMTokens, error) {
	config, err := s.repo.Internal.GetCRMConfigInternal(configID)
	if err != nil {
		return nil, fmt.Errorf("конфигурация не найдена: %w", err)
	}

	if config.CRMType != "amocrm" {
		return nil, fmt.Errorf("обновление токена поддерживается только для AmoCRM")
	}

	var creds models.CRMCredentials
	if err := json.Unmarshal([]byte(config.Credentials), &creds); err != nil {
		return nil, fmt.Errorf("ошибка парсинга credentials: %w", err)
	}

	if creds.ClientID == "" || creds.ClientSecret == "" || creds.RefreshToken == "" {
		return nil, fmt.Errorf("отсутствуют client_id/client_secret/refresh_token для обновления токена")
	}

	form := url.Values{}
	form.Set("client_id", creds.ClientID)
	form.Set("client_secret", creds.ClientSecret)
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", creds.RefreshToken)
	form.Set("redirect_uri", creds.RedirectURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("https://%s.amocrm.ru/oauth2/access_token", config.Subdomain), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса на обновление токена: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения запроса на обновление токена: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа обновления токена: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AmoCRM вернул ошибку при обновлении токена (%d): %s", resp.StatusCode, string(body))
	}

	var tokenResp AmoCRMTokens
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("ошибка парсинга ответа обновления токена: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("AmoCRM не вернул access_token")
	}

	if tokenResp.ExpiresIn > 0 {
		tokenResp.ExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).Unix()
	}
	if tokenResp.TokenType == "" {
		tokenResp.TokenType = "Bearer"
	}

	creds.AccessToken = tokenResp.AccessToken
	creds.RefreshToken = tokenResp.RefreshToken
	creds.ExpiresAt = tokenResp.ExpiresAt

	updatedCreds, err := json.Marshal(creds)
	if err != nil {
		return nil, fmt.Errorf("ошибка сериализации credentials: %w", err)
	}
	config.Credentials = string(updatedCreds)

	if err := s.repo.Internal.UpdateCRMConfig(config); err != nil {
		return nil, fmt.Errorf("ошибка сохранения обновленных credentials: %w", err)
	}

	return &tokenResp, nil
}
