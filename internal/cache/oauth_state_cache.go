package cache

import (
	"Marusia_CRM/internal/domain/models"
	"sync"
	"time"
)

// OAuthStateCache кеш для OAuth states с TTL
type OAuthStateCache struct {
	mu     sync.RWMutex
	states map[string]*cacheEntry
}

type cacheEntry struct {
	state     *models.OAuthState
	expiresAt time.Time
}

// NewOAuthStateCache создает новый кеш
func NewOAuthStateCache() *OAuthStateCache {
	cache := &OAuthStateCache{
		states: make(map[string]*cacheEntry),
	}

	// Запускаем goroutine для очистки истекших записей
	go cache.cleanupExpired()

	return cache
}

// Set сохраняет state в кеш
func (c *OAuthStateCache) Set(state *models.OAuthState) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.states[state.State] = &cacheEntry{
		state:     state,
		expiresAt: state.ExpiresAt,
	}
}

// Get получает state из кеша
func (c *OAuthStateCache) Get(stateKey string) (*models.OAuthState, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.states[stateKey]
	if !exists {
		return nil, false
	}

	// Проверяем, не истек ли срок
	if time.Now().After(entry.expiresAt) {
		return nil, false
	}

	return entry.state, true
}

// Delete удаляет state из кеша
func (c *OAuthStateCache) Delete(stateKey string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.states, stateKey)
}

// cleanupExpired периодически очищает истекшие записи
func (c *OAuthStateCache) cleanupExpired() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()

		for key, entry := range c.states {
			if now.After(entry.expiresAt) {
				delete(c.states, key)
			}
		}

		c.mu.Unlock()
	}
}

// Size возвращает количество записей в кеше
func (c *OAuthStateCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.states)
}
