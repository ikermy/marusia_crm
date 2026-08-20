package phone

import (
	"regexp"
	"strings"
)

// Normalize нормализует номер телефона для поиска в AmoCRM
// Удаляет все символы кроме цифр, обрабатывает разные форматы
func Normalize(phone string) string {
	if phone == "" {
		return ""
	}

	// Удаляем все символы кроме цифр и '+'
	phone = strings.TrimSpace(phone)

	// Регулярное выражение для удаления всех символов кроме цифр
	reg := regexp.MustCompile(`[^\d]`)
	phone = reg.ReplaceAllString(phone, "")

	// Если номер начинается с 8 (российский формат), заменяем на 7
	if strings.HasPrefix(phone, "8") && len(phone) == 11 {
		phone = "7" + phone[1:]
	}

	// Если номер 10 цифр (без кода страны), добавляем 7
	if len(phone) == 10 {
		phone = "7" + phone
	}

	return phone
}

// IsValid проверяет, является ли номер валидным (минимум 10 цифр)
func IsValid(phone string) bool {
	normalized := Normalize(phone)
	return len(normalized) >= 10
}

// Format форматирует номер в красивый вид +7 (XXX) XXX-XX-XX
func Format(phone string) string {
	normalized := Normalize(phone)
	if len(normalized) != 11 || !strings.HasPrefix(normalized, "7") {
		return phone // Возвращаем как есть, если не российский номер
	}

	// +7 (XXX) XXX-XX-XX
	return "+" + normalized[0:1] + " (" + normalized[1:4] + ") " + normalized[4:7] + "-" + normalized[7:9] + "-" + normalized[9:11]
}
