package providers

import (
	"Marusia_CRM/internal/domain/models"
	"context"
	"fmt"
	"sync"
)

// Registry управляет провайдерами CRM
type Registry struct {
	providers  map[string]func() CRMProvider     // фабрики провайдеров
	instances  map[uint32]map[string]CRMProvider // userID -> crmType -> instance
	httpClient any                               // HTTP клиент для передачи провайдерам
	mu         sync.RWMutex
}

// NewRegistry создает новый реестр провайдеров
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]func() CRMProvider),
		instances: make(map[uint32]map[string]CRMProvider),
	}
}

// SetHTTPClient устанавливает HTTP клиент для провайдеров
func (r *Registry) SetHTTPClient(httpClient any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.httpClient = httpClient
}

// RegisterProvider регистрирует фабрику провайдера
func (r *Registry) RegisterProvider(crmType string, factory func() CRMProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[crmType] = factory
}

// GetOrCreateProvider получает или создает провайдер для пользователя
func (r *Registry) GetOrCreateProvider(ctx context.Context, userID uint32, config *models.CRMConfig) (CRMProvider, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Проверяем, есть ли уже инстанс
	if userProviders, exists := r.instances[config.UserID]; exists {
		if provider, exists := userProviders[config.CRMType]; exists {
			return provider, nil
		}
	}

	// Создаем новый провайдер
	factory, exists := r.providers[config.CRMType]
	if !exists {
		return nil, fmt.Errorf("провайдер типа '%s' не зарегистрирован", config.CRMType)
	}

	provider := factory()
	if err := provider.Initialize(ctx, userID, config, r.httpClient); err != nil {
		return nil, fmt.Errorf("ошибка инициализации провайдера: %w", err)
	}

	// Сохраняем в кэш
	if r.instances[config.UserID] == nil {
		r.instances[config.UserID] = make(map[string]CRMProvider)
	}
	r.instances[config.UserID][config.CRMType] = provider

	return provider, nil
}

// RemoveProvider удаляет провайдер из кэша
func (r *Registry) RemoveProvider(userID uint32, crmType string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if userProviders, exists := r.instances[userID]; exists {
		delete(userProviders, crmType)
		if len(userProviders) == 0 {
			delete(r.instances, userID)
		}
	}
}

// ClearUserProviders очищает все провайдеры пользователя
func (r *Registry) ClearUserProviders(userID uint32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.instances, userID)
}
