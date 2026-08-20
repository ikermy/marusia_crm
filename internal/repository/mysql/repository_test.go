package mysql

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/ikermy/air_common/pkg/comdb"
	"github.com/ikermy/air_common/pkg/crypto"
)

func TestGetCRMConfig_ReadsAndDecryptsFromRealDB(t *testing.T) {
	t.Setenv("DB_HOST", "localhost:3306")
	t.Setenv("DB_NAME", "air")
	t.Setenv("DB_USER", "root")
	t.Setenv("DB_PASSWORD", "123456")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	base, err := comdb.New(ctx)
	if err != nil {
		t.Fatalf("comdb.New failed: %v", err)
	}
	defer func() {
		if err := base.Close(); err != nil {
			t.Fatalf("base.Close failed: %v", err)
		}
	}()

	masterKey := [32]byte{145, 16, 233, 69, 142, 78, 119, 37, 108, 117, 47, 117, 253, 200, 4, 221, 48, 229, 209, 20, 42, 189, 224, 95, 51, 87, 19, 180, 46, 171, 46, 241}
	base.SetMasterKeyResolver(func(userID uint32) ([32]byte, bool) {
		return masterKey, true
	})

	repo, err := New(base)
	if err != nil {
		t.Fatalf("New repository failed: %v", err)
	}

	var userID uint32
	row := base.Conn().QueryRowContext(ctx, "SELECT Id FROM users ORDER BY Id LIMIT 1")
	if err := row.Scan(&userID); err != nil {
		t.Fatalf("failed to find existing user in users table: %v", err)
	}

	plainName := "Тестовый CRM"
	plainSubdomain := "demo"
	plainCredentials := `{"client_id":"client123","client_secret":"secret123","access_token":"access-token-1","refresh_token":"refresh-token-1","redirect_url":"https://example.com/callback"}`
	plainOptions := `{"default_pipeline_id":123}`
	plainChannels := `{"telegram":{"Assist":"hello"}}`

	encryptedName, err := crypto.EncryptFieldWithMasterKey(masterKey, plainName)
	if err != nil {
		t.Fatalf("EncryptFieldWithMasterKey(Name) failed: %v", err)
	}
	encryptedSubdomain, err := crypto.EncryptFieldWithMasterKey(masterKey, plainSubdomain)
	if err != nil {
		t.Fatalf("EncryptFieldWithMasterKey(Subdomain) failed: %v", err)
	}
	encryptedCredentials, err := crypto.EncryptFieldWithMasterKey(masterKey, plainCredentials)
	if err != nil {
		t.Fatalf("EncryptFieldWithMasterKey(Credentials) failed: %v", err)
	}
	encryptedOptions, err := crypto.EncryptFieldWithMasterKey(masterKey, plainOptions)
	if err != nil {
		t.Fatalf("EncryptFieldWithMasterKey(Options) failed: %v", err)
	}
	encryptedChannels, err := crypto.EncryptFieldWithMasterKey(masterKey, plainChannels)
	if err != nil {
		t.Fatalf("EncryptFieldWithMasterKey(Channels) failed: %v", err)
	}

	res, err := base.Conn().ExecContext(ctx, `
		INSERT INTO crm_configs
		(user_id, crm_type, name, subdomain, credentials, options, channels, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
	`, userID, "amocrm", encryptedName, encryptedSubdomain, encryptedCredentials, encryptedOptions, encryptedChannels, true)
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) {
			t.Fatalf("insert crm_config failed: %v", mysqlErr)
		}
		t.Fatalf("insert crm_config failed: %v", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId failed: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = base.Conn().ExecContext(cleanupCtx, "DELETE FROM crm_configs WHERE id = ?", id)
	})

	cfg, err := repo.Internal.GetCRMConfig(uint32(id))
	if err != nil {
		t.Fatalf("GetCRMConfig failed: %v", err)
	}

	if cfg.UserID != userID {
		t.Fatalf("expected user_id %d, got %d", userID, cfg.UserID)
	}
	if cfg.Name != plainName {
		t.Fatalf("expected decrypted Name %q, got %q", plainName, cfg.Name)
	}
	if cfg.Subdomain != plainSubdomain {
		t.Fatalf("expected decrypted Subdomain %q, got %q", plainSubdomain, cfg.Subdomain)
	}
	if cfg.Options != plainOptions {
		t.Fatalf("expected decrypted Options %q, got %q", plainOptions, cfg.Options)
	}
	if cfg.Chanells != plainChannels {
		t.Fatalf("expected decrypted Channels %q, got %q", plainChannels, cfg.Chanells)
	}
	if strings.Contains(cfg.Credentials, "access_token") || strings.Contains(cfg.Credentials, "refresh_token") || strings.Contains(cfg.Credentials, "client_secret") {
		t.Fatalf("expected credentials to be sanitized, got %q", cfg.Credentials)
	}
	if !strings.Contains(cfg.Credentials, "client_id") || !strings.Contains(cfg.Credentials, "redirect_url") {
		t.Fatalf("expected credentials to contain client_id/redirect_url after sanitization, got %q", cfg.Credentials)
	}
}
