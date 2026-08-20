package db

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func TestDB_GetNotificationChannel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := New(ctx)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	res, err := db.GetNotificationChannel(uint32(23))
	if err != nil {
		t.Fatalf("ReadContext failed: %v", err)
	}

	// Парсим JSON и выводим в консоль в читаемом виде
	var channels []map[string]any
	err = json.Unmarshal(res, &channels)
	if err != nil {
		t.Fatalf("Ошибка парсинга JSON: %v", err)
	}
	fmt.Printf("Каналы уведомлений пользователя: %v\n", channels)
	for _, ch := range channels {
		fmt.Printf("Тип: %v, Значение: %v\n", ch["channel_type"], ch["channel_value"])
	}

	t.Log(channels)
}
