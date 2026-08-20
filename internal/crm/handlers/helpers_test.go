package handlers

import (
	"testing"
)

func TestNormalizeRedirectURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Дублированный путь с amoCRM",
			input:    "https://kermy.org/crm/oauth/amoCRM/callback/crm/oauth/amoCRM/callback",
			expected: "https://kermy.org/crm/oauth/amocrm/callback",
		},
		{
			name:     "Нормализация регистра amoCRM",
			input:    "https://kermy.org/crm/oauth/amoCRM/callback",
			expected: "https://kermy.org/crm/oauth/amocrm/callback",
		},
		{
			name:     "Удаление trailing slash",
			input:    "https://kermy.org/crm/oauth/amocrm/callback/",
			expected: "https://kermy.org/crm/oauth/amocrm/callback",
		},
		{
			name:     "Корректный URL без изменений",
			input:    "https://kermy.org/crm/oauth/amocrm/callback",
			expected: "https://kermy.org/crm/oauth/amocrm/callback",
		},
		{
			name:     "Другой домен",
			input:    "https://example.com/crm/oauth/amoCRM/callback",
			expected: "https://example.com/crm/oauth/amocrm/callback",
		},
		{
			name:     "HTTP протокол",
			input:    "http://localhost:8081/crm/oauth/amoCRM/callback",
			expected: "http://localhost:8081/crm/oauth/amocrm/callback",
		},
		{
			name:     "Пустая строка",
			input:    "",
			expected: "",
		},
		{
			name:     "Дублированный путь с другим регистром",
			input:    "https://example.com/api/test/api/test",
			expected: "https://example.com/api/test",
		},
		{
			name:     "Множественные trailing slashes",
			input:    "https://kermy.org/crm/oauth/amocrm/callback///",
			expected: "https://kermy.org/crm/oauth/amocrm/callback",
		},
		{
			name:     "AmoCRM в разном регистре",
			input:    "https://kermy.org/crm/oauth/AMOCRM/callback",
			expected: "https://kermy.org/crm/oauth/amocrm/callback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeRedirectURL(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeRedirectURL(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}
