package mysql

import (
	"Marusia_CRM/internal/domain/models"
	"Marusia_CRM/internal/repository"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ikermy/air_common/pkg/comdb"
	"github.com/ikermy/air_common/pkg/crypto"
	"github.com/ikermy/air_common/pkg/mode"
)

// Repository реализация интерфейса Repository для MySQL
type Repository struct {
	db *comdb.DB
}

// New создаёт новый MySQL репозиторий пользователей
func New(db *comdb.DB) (repository.Repository, error) {
	if db == nil {
		return repository.Repository{}, fmt.Errorf("database connection is nil")
	}

	repo := &Repository{
		db: db,
	}

	return repository.Repository{
		Internal: repo,
		External: db,
	}, nil
}

//// CRMRepository реализация репозитория на чистом SQL
//type CRMRepository struct {
//	db     *sql.DB
//	ctx    context.Context
//	cancel context.CancelFunc
//}

//// NewCRMRepository создает новый репозиторий
//func NewCRMRepository(parent context.Context, db *sql.DB) *CRMRepository {
//	ctx, cancel := context.WithCancel(parent)
//	return &CRMRepository{
//		db:     db,
//		ctx:    ctx,
//		cancel: cancel,
//	}
//}

// Close отменяет контекст репозитория
func (r *Repository) Close() {
	if r.db.Context().Done() != nil {
		defer func() { _ = r.db.Conn().Close() }()
	}
}

// sanitizeCredentials удаляет чувствительные данные (токены) из credentials JSON
// Используется для безопасного возврата данных клиенту
func sanitizeCredentials(credentialsJSON string) (string, error) {
	if credentialsJSON == "" {
		return "{}", nil
	}

	var creds map[string]any
	if err := json.Unmarshal([]byte(credentialsJSON), &creds); err != nil {
		// Если не удалось распарсить, возвращаем пустой объект для безопасности
		return "{}", nil
	}

	// Удаляем чувствительные данные
	delete(creds, "access_token")
	delete(creds, "refresh_token")
	delete(creds, "client_secret")

	// Сериализуем обратно в JSON
	sanitized, err := json.Marshal(creds)
	if err != nil {
		return "{}", err
	}

	return string(sanitized), nil
}

func (r *Repository) decryptCRMConfigFields(config *models.CRMConfig) error {
	if config == nil || r.db == nil || r.db.MasterKeyResolver == nil {
		return nil
	}

	masterKey, ok := r.db.MasterKeyResolver(config.UserID)
	if !ok {
		return nil
	}

	if crypto.IsEncryptedWithMasterKey(config.Name) {
		decrypted, err := crypto.DecryptFieldWithMasterKey(masterKey, config.Name)
		if err != nil {
			return fmt.Errorf("ошибка расшифровки Name: %w", err)
		}
		config.Name = decrypted
	}

	if crypto.IsEncryptedWithMasterKey(config.Subdomain) {
		decrypted, err := crypto.DecryptFieldWithMasterKey(masterKey, config.Subdomain)
		if err != nil {
			return fmt.Errorf("ошибка расшифровки Subdomain: %w", err)
		}
		config.Subdomain = decrypted
	}

	if crypto.IsEncryptedWithMasterKey(config.Credentials) {
		decrypted, err := crypto.DecryptFieldWithMasterKey(masterKey, config.Credentials)
		if err != nil {
			return fmt.Errorf("ошибка расшифровки Credentials: %w", err)
		}
		config.Credentials = decrypted
	}

	if crypto.IsEncryptedWithMasterKey(config.Options) {
		decrypted, err := crypto.DecryptFieldWithMasterKey(masterKey, config.Options)
		if err != nil {
			return fmt.Errorf("ошибка расшифровки Options: %w", err)
		}
		config.Options = decrypted
	}

	if crypto.IsEncryptedWithMasterKey(config.Chanells) {
		decrypted, err := crypto.DecryptFieldWithMasterKey(masterKey, config.Chanells)
		if err != nil {
			return fmt.Errorf("ошибка расшифровки Chanells: %w", err)
		}
		config.Chanells = decrypted
	}

	return nil
}

func (r *Repository) encryptCRMConfigFields(config *models.CRMConfig) error {
	if config == nil || r.db == nil || r.db.MasterKeyResolver == nil {
		return nil
	}

	masterKey, ok := r.db.MasterKeyResolver(config.UserID)
	if !ok {
		return nil
	}

	if config.Name != "" && !crypto.IsEncryptedWithMasterKey(config.Name) {
		encrypted, err := crypto.EncryptFieldWithMasterKey(masterKey, config.Name)
		if err != nil {
			return fmt.Errorf("ошибка шифрования Name: %w", err)
		}
		config.Name = encrypted
	}

	if config.Subdomain != "" && !crypto.IsEncryptedWithMasterKey(config.Subdomain) {
		encrypted, err := crypto.EncryptFieldWithMasterKey(masterKey, config.Subdomain)
		if err != nil {
			return fmt.Errorf("ошибка шифрования Subdomain: %w", err)
		}
		config.Subdomain = encrypted
	}

	if config.Credentials != "" && !crypto.IsEncryptedWithMasterKey(config.Credentials) {
		encrypted, err := crypto.EncryptFieldWithMasterKey(masterKey, config.Credentials)
		if err != nil {
			return fmt.Errorf("ошибка шифрования Credentials: %w", err)
		}
		config.Credentials = encrypted
	}

	if config.Options != "" && !crypto.IsEncryptedWithMasterKey(config.Options) {
		encrypted, err := crypto.EncryptFieldWithMasterKey(masterKey, config.Options)
		if err != nil {
			return fmt.Errorf("ошибка шифрования Options: %w", err)
		}
		config.Options = encrypted
	}

	if config.Chanells != "" && !crypto.IsEncryptedWithMasterKey(config.Chanells) {
		encrypted, err := crypto.EncryptFieldWithMasterKey(masterKey, config.Chanells)
		if err != nil {
			return fmt.Errorf("ошибка шифрования Chanells: %w", err)
		}
		config.Chanells = encrypted
	}

	return nil
}

// ============= CRM Configs =============

func (r *Repository) CreateCRMConfig(config *models.CRMConfig) error {
	// Получаем часовой пояс пользователя
	tz, err := r.db.UserTimeZone(config.UserID)
	if err != nil {
		return err
	}

	if err := r.encryptCRMConfigFields(config); err != nil {
		return err
	}

	ctxTimeout, cancel := context.WithTimeout(r.db.Context(), mode.GetSQLTimeToCancel())
	defer cancel()

	result, err := r.db.Conn().ExecContext(ctxTimeout, `
		INSERT INTO crm_configs 
		(user_id, crm_type, name, subdomain, credentials, options, channels, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, CONVERT_TZ(NOW(), @@session.time_zone, ?), CONVERT_TZ(NOW(), @@session.time_zone, ?))
	`, config.UserID, config.CRMType, config.Name, config.Subdomain, config.Credentials, config.Options, config.Chanells, config.IsActive, tz, tz)

	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			return fmt.Errorf("тайм-аут (%d с) при создании CRM конфига: %w", mode.GetSQLTimeToCancel(), err)
		case errors.Is(err, context.Canceled):
			return fmt.Errorf("операция отменена: %w", err)
		default:
			return fmt.Errorf("ошибка создания CRM конфига: %w", err)
		}
	}

	// Получаем ID вставленной записи
	id, err := result.LastInsertId()
	if err == nil {
		config.ID = uint32(id)
	}

	return nil
}

func (r *Repository) GetCRMConfig(id uint32) (*models.CRMConfig, error) {
	ctxTimeout, cancel := context.WithTimeout(r.db.Context(), mode.GetSQLTimeToCancel())
	defer cancel()

	var config models.CRMConfig
	var createdAt, updatedAt sql.NullTime

	err := r.db.Conn().QueryRowContext(ctxTimeout, `
		SELECT id, user_id, crm_type, name, subdomain, credentials, options, channels, is_active, created_at, updated_at
		FROM crm_configs
		WHERE id = ?
	`, id).Scan(&config.ID, &config.UserID, &config.CRMType, &config.Name, &config.Subdomain,
		&config.Credentials, &config.Options, &config.Chanells, &config.IsActive, &createdAt, &updatedAt)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, fmt.Errorf("CRM конфиг с ID %d не найден", id)
		case errors.Is(err, context.DeadlineExceeded):
			return nil, fmt.Errorf("тайм-аут (%d с) при получении CRM конфига: %w", mode.GetSQLTimeToCancel(), err)
		case errors.Is(err, context.Canceled):
			return nil, fmt.Errorf("операция отменена: %w", err)
		default:
			return nil, fmt.Errorf("ошибка получения CRM конфига: %w", err)
		}
	}

	if err := r.decryptCRMConfigFields(&config); err != nil {
		return nil, err
	}

	// Преобразуем nullable time в time.Time
	if createdAt.Valid {
		config.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		config.UpdatedAt = updatedAt.Time
	}

	// Удаляем токены из credentials перед возвратом клиенту
	sanitized, err := sanitizeCredentials(config.Credentials)
	if err != nil {
		return nil, fmt.Errorf("ошибка санитизации credentials: %w", err)
	}
	config.Credentials = sanitized

	return &config, nil
}

func (r *Repository) GetUserCRMConfigs(userID uint32) ([]models.CRMConfig, error) {
	ctxTimeout, cancel := context.WithTimeout(r.db.Context(), mode.GetSQLTimeToCancel())
	defer cancel()

	rows, err := r.db.Conn().QueryContext(ctxTimeout, `
		SELECT id, user_id, crm_type, name, subdomain, credentials, options, channels, is_active, created_at, updated_at
		FROM crm_configs
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)

	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			return nil, fmt.Errorf("тайм-аут (%d с) при получении CRM конфигов: %w", mode.GetSQLTimeToCancel(), err)
		case errors.Is(err, context.Canceled):
			return nil, fmt.Errorf("операция отменена: %w", err)
		default:
			return nil, fmt.Errorf("ошибка получения CRM конфигов: %w", err)
		}
	}
	defer func() { _ = rows.Close() }()

	var configs []models.CRMConfig
	for rows.Next() {
		var config models.CRMConfig
		var createdAt, updatedAt sql.NullTime

		err := rows.Scan(&config.ID, &config.UserID, &config.CRMType, &config.Name, &config.Subdomain,
			&config.Credentials, &config.Options, &config.Chanells, &config.IsActive, &createdAt, &updatedAt)
		if err != nil {
			return nil, fmt.Errorf("ошибка сканирования CRM конфига: %w", err)
		}

		// Преобразуем nullable time в time.Time
		if createdAt.Valid {
			config.CreatedAt = createdAt.Time
		}
		if updatedAt.Valid {
			config.UpdatedAt = updatedAt.Time
		}

		if err := r.decryptCRMConfigFields(&config); err != nil {
			return nil, fmt.Errorf("ошибка расшифровки CRM конфига: %w", err)
		}

		// Удаляем токены из credentials перед возвратом клиенту
		sanitized, err := sanitizeCredentials(config.Credentials)
		if err != nil {
			return nil, fmt.Errorf("ошибка санитизации credentials: %w", err)
		}
		config.Credentials = sanitized

		configs = append(configs, config)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка итерации по CRM конфигам: %w", err)
	}

	return configs, nil
}

// GetCRMConfigByType получает CRM конфигурацию по user_id и crm_type
func (r *Repository) GetCRMConfigByType(userID uint32, crmType string) (*models.CRMConfig, error) {
	ctxTimeout, cancel := context.WithTimeout(r.db.Context(), mode.GetSQLTimeToCancel())
	defer cancel()

	var config models.CRMConfig
	var createdAt, updatedAt sql.NullTime

	err := r.db.Conn().QueryRowContext(ctxTimeout, `
		SELECT id, user_id, crm_type, name, subdomain, credentials, options, channels, is_active, created_at, updated_at
		FROM crm_configs
		WHERE user_id = ? AND crm_type = ?
	`, userID, crmType).Scan(&config.ID, &config.UserID, &config.CRMType, &config.Name, &config.Subdomain,
		&config.Credentials, &config.Options, &config.Chanells, &config.IsActive, &createdAt, &updatedAt)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, fmt.Errorf("CRM конфиг типа %s для пользователя %d не найден", crmType, userID)
		case errors.Is(err, context.DeadlineExceeded):
			return nil, fmt.Errorf("тайм-аут (%d с) при получении CRM конфига: %w", mode.GetSQLTimeToCancel(), err)
		case errors.Is(err, context.Canceled):
			return nil, fmt.Errorf("операция отменена: %w", err)
		default:
			return nil, fmt.Errorf("ошибка получения CRM конфига: %w", err)
		}
	}

	// Преобразуем nullable time в time.Time
	if createdAt.Valid {
		config.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		config.UpdatedAt = updatedAt.Time
	}

	if err := r.decryptCRMConfigFields(&config); err != nil {
		return nil, err
	}

	// Удаляем токены из credentials перед возвратом клиенту
	sanitized, err := sanitizeCredentials(config.Credentials)
	if err != nil {
		return nil, fmt.Errorf("ошибка санитизации credentials: %w", err)
	}
	config.Credentials = sanitized

	return &config, nil
}

// ============= Internal методы (для использования внутри CRMService) =============
// Эти методы возвращают полные credentials с токенами для работы с CRM API

// GetCRMConfigInternal получает CRM конфигурацию с полными credentials (включая токены)
// Используется внутри CRMService для работы с CRM API
func (r *Repository) GetCRMConfigInternal(id uint32) (*models.CRMConfig, error) {
	ctxTimeout, cancel := context.WithTimeout(r.db.Context(), mode.GetSQLTimeToCancel())
	defer cancel()

	var config models.CRMConfig
	var createdAt, updatedAt sql.NullTime

	err := r.db.Conn().QueryRowContext(ctxTimeout, `
		SELECT id, user_id, crm_type, name, subdomain, credentials, options, channels, is_active, created_at, updated_at
		FROM crm_configs
		WHERE id = ?
	`, id).Scan(&config.ID, &config.UserID, &config.CRMType, &config.Name, &config.Subdomain,
		&config.Credentials, &config.Options, &config.Chanells, &config.IsActive, &createdAt, &updatedAt)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, fmt.Errorf("CRM конфиг с ID %d не найден", id)
		case errors.Is(err, context.DeadlineExceeded):
			return nil, fmt.Errorf("тайм-аут (%d с) при получении CRM конфига: %w", mode.GetSQLTimeToCancel(), err)
		case errors.Is(err, context.Canceled):
			return nil, fmt.Errorf("операция отменена: %w", err)
		default:
			return nil, fmt.Errorf("ошибка получения CRM конфига: %w", err)
		}
	}

	// Преобразуем nullable time в time.Time
	if createdAt.Valid {
		config.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		config.UpdatedAt = updatedAt.Time
	}

	if err := r.decryptCRMConfigFields(&config); err != nil {
		return nil, err
	}

	// НЕ санитизируем credentials - возвращаем полные данные для внутреннего использования
	return &config, nil
}

// GetCRMConfigByTypeInternal получает CRM конфигурацию с полными credentials (включая токены)
// Используется внутри CRMService для работы с CRM API
func (r *Repository) GetCRMConfigByTypeInternal(userID uint32, crmType string) (*models.CRMConfig, error) {
	ctxTimeout, cancel := context.WithTimeout(r.db.Context(), mode.GetSQLTimeToCancel())
	defer cancel()

	var config models.CRMConfig
	var createdAt, updatedAt sql.NullTime

	err := r.db.Conn().QueryRowContext(ctxTimeout, `
		SELECT id, user_id, crm_type, name, subdomain, credentials, options, channels, is_active, created_at, updated_at
		FROM crm_configs
		WHERE user_id = ? AND crm_type = ?
	`, userID, crmType).Scan(&config.ID, &config.UserID, &config.CRMType, &config.Name, &config.Subdomain,
		&config.Credentials, &config.Options, &config.Chanells, &config.IsActive, &createdAt, &updatedAt)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, fmt.Errorf("CRM конфиг типа %s для пользователя %d не найден", crmType, userID)
		case errors.Is(err, context.DeadlineExceeded):
			return nil, fmt.Errorf("тайм-аут (%d с) при получении CRM конфига: %w", mode.GetSQLTimeToCancel(), err)
		case errors.Is(err, context.Canceled):
			return nil, fmt.Errorf("операция отменена: %w", err)
		default:
			return nil, fmt.Errorf("ошибка получения CRM конфига: %w", err)
		}
	}

	// Преобразуем nullable time в time.Time
	if createdAt.Valid {
		config.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		config.UpdatedAt = updatedAt.Time
	}

	if err := r.decryptCRMConfigFields(&config); err != nil {
		return nil, err
	}

	// НЕ санитизируем credentials - возвращаем полные данные для внутреннего использования
	return &config, nil
}

// UpsertCRMConfig создает или обновляет CRM конфигурацию (INSERT ... ON DUPLICATE KEY UPDATE)
func (r *Repository) UpsertCRMConfig(config *models.CRMConfig) error {
	// Получаем часовой пояс пользователя
	tz, err := r.db.UserTimeZone(config.UserID)
	if err != nil {
		return err
	}

	if err := r.encryptCRMConfigFields(config); err != nil {
		return err
	}

	ctxTimeout, cancel := context.WithTimeout(r.db.Context(), mode.GetSQLTimeToCancel())
	defer cancel()

	result, err := r.db.Conn().ExecContext(ctxTimeout, `
		INSERT INTO crm_configs 
		(user_id, crm_type, name, subdomain, credentials, options, channels, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, CONVERT_TZ(NOW(), @@session.time_zone, ?), CONVERT_TZ(NOW(), @@session.time_zone, ?))
		ON DUPLICATE KEY UPDATE
		name = VALUES(name),
		subdomain = VALUES(subdomain),
		credentials = VALUES(credentials),
		options = VALUES(options),
		channels = VALUES(channels),
		is_active = VALUES(is_active),
		updated_at = CONVERT_TZ(NOW(), @@session.time_zone, ?)
	`, config.UserID, config.CRMType, config.Name, config.Subdomain, config.Credentials, config.Options, config.Chanells, config.IsActive, tz, tz, tz)

	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			return fmt.Errorf("тайм-аут (%d с) при сохранении CRM конфига: %w", mode.GetSQLTimeToCancel(), err)
		case errors.Is(err, context.Canceled):
			return fmt.Errorf("операция отменена: %w", err)
		default:
			return fmt.Errorf("ошибка сохранения CRM конфига: %w", err)
		}
	}

	// Если это был INSERT, получаем ID
	if config.ID == 0 {
		id, err := result.LastInsertId()
		if err == nil {
			config.ID = uint32(id)
		}
	}

	return nil
}

func (r *Repository) UpdateCRMConfig(config *models.CRMConfig) error {
	// Получаем часовой пояс пользователя
	tz, err := r.db.UserTimeZone(config.UserID)
	if err != nil {
		return err
	}

	if err := r.encryptCRMConfigFields(config); err != nil {
		return err
	}

	ctxTimeout, cancel := context.WithTimeout(r.db.Context(), mode.GetSQLTimeToCancel())
	defer cancel()

	_, err = r.db.Conn().ExecContext(ctxTimeout, `
		UPDATE crm_configs
		SET user_id = ?, crm_type = ?, name = ?, subdomain = ?, credentials = ?, options = ?, channels = ?, is_active = ?,
		    updated_at = CONVERT_TZ(NOW(), @@session.time_zone, ?)
		WHERE id = ?
	`, config.UserID, config.CRMType, config.Name, config.Subdomain, config.Credentials,
		config.Options, config.Chanells, config.IsActive, tz, config.ID)

	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			return fmt.Errorf("тайм-аут (%d с) при обновлении CRM конфига: %w", mode.GetSQLTimeToCancel(), err)
		case errors.Is(err, context.Canceled):
			return fmt.Errorf("операция отменена: %w", err)
		default:
			return fmt.Errorf("ошибка обновления CRM конфига: %w", err)
		}
	}

	return nil
}

func (r *Repository) DeleteCRMConfig(id uint32) error {
	ctxTimeout, cancel := context.WithTimeout(r.db.Context(), mode.GetSQLTimeToCancel())
	defer cancel()

	_, err := r.db.Conn().ExecContext(ctxTimeout, "DELETE FROM crm_configs WHERE id = ?", id)
	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			return fmt.Errorf("тайм-аут (%d с) при удалении CRM конфига: %w", mode.GetSQLTimeToCancel(), err)
		case errors.Is(err, context.Canceled):
			return fmt.Errorf("операция отменена: %w", err)
		default:
			return fmt.Errorf("ошибка удаления CRM конфига: %w", err)
		}
	}

	return nil
}

// ============= Mappings =============

func (r *Repository) CreateMapping(mapping *models.CRMMapping) error {
	// Получаем часовой пояс пользователя
	tz, err := r.db.UserTimeZone(mapping.UserID)
	if err != nil {
		return err
	}

	ctxTimeout, cancel := context.WithTimeout(r.db.Context(), mode.GetSQLTimeToCancel())
	defer cancel()

	result, err := r.db.Conn().ExecContext(ctxTimeout, `
		INSERT INTO crm_mappings 
		(user_id, application_id, crm_config_id, field_mapping, pipeline_id, status_id, 
		 responsible_user_id, is_active, priority, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CONVERT_TZ(NOW(), @@session.time_zone, ?), CONVERT_TZ(NOW(), @@session.time_zone, ?))
	`, mapping.UserID, mapping.ApplicationID, mapping.CRMConfigID, mapping.FieldMapping,
		mapping.PipelineID, mapping.StatusID, mapping.ResponsibleUserID, mapping.IsActive,
		mapping.Priority, tz, tz)

	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			return fmt.Errorf("тайм-аут (%d с) при создании маппинга: %w", mode.GetSQLTimeToCancel(), err)
		case errors.Is(err, context.Canceled):
			return fmt.Errorf("операция отменена: %w", err)
		default:
			return fmt.Errorf("ошибка создания маппинга: %w", err)
		}
	}

	id, err := result.LastInsertId()
	if err == nil {
		mapping.ID = uint32(id)
	}

	return nil
}

func (r *Repository) GetMapping(id uint32) (*models.CRMMapping, error) {
	ctxTimeout, cancel := context.WithTimeout(r.db.Context(), mode.GetSQLTimeToCancel())
	defer cancel()

	var mapping models.CRMMapping
	err := r.db.Conn().QueryRowContext(ctxTimeout, `
		SELECT id, user_id, application_id, crm_config_id, field_mapping, pipeline_id, status_id,
		       responsible_user_id, is_active, priority, created_at, updated_at
		FROM crm_mappings
		WHERE id = ?
	`, id).Scan(&mapping.ID, &mapping.UserID, &mapping.ApplicationID, &mapping.CRMConfigID,
		&mapping.FieldMapping, &mapping.PipelineID, &mapping.StatusID, &mapping.ResponsibleUserID,
		&mapping.IsActive, &mapping.Priority, &mapping.CreatedAt, &mapping.UpdatedAt)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, fmt.Errorf("маппинг с ID %d не найден", id)
		case errors.Is(err, context.DeadlineExceeded):
			return nil, fmt.Errorf("тайм-аут (%d с) при получении маппинга: %w", mode.GetSQLTimeToCancel(), err)
		case errors.Is(err, context.Canceled):
			return nil, fmt.Errorf("операция отменена: %w", err)
		default:
			return nil, fmt.Errorf("ошибка получения маппинга: %w", err)
		}
	}

	return &mapping, nil
}

func (r *Repository) GetUserMappings(userID uint32) ([]models.CRMMapping, error) {
	ctxTimeout, cancel := context.WithTimeout(r.db.Context(), mode.GetSQLTimeToCancel())
	defer cancel()

	rows, err := r.db.Conn().QueryContext(ctxTimeout, `
		SELECT id, user_id, application_id, crm_config_id, field_mapping, pipeline_id, status_id,
		       responsible_user_id, is_active, priority, created_at, updated_at
		FROM crm_mappings
		WHERE user_id = ?
		ORDER BY priority DESC, created_at DESC
	`, userID)

	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			return nil, fmt.Errorf("тайм-аут (%d с) при получении маппингов: %w", mode.GetSQLTimeToCancel(), err)
		case errors.Is(err, context.Canceled):
			return nil, fmt.Errorf("операция отменена: %w", err)
		default:
			return nil, fmt.Errorf("ошибка получения маппингов: %w", err)
		}
	}
	defer func() { _ = rows.Close() }()

	var mappings []models.CRMMapping
	for rows.Next() {
		var mapping models.CRMMapping
		err := rows.Scan(&mapping.ID, &mapping.UserID, &mapping.ApplicationID, &mapping.CRMConfigID,
			&mapping.FieldMapping, &mapping.PipelineID, &mapping.StatusID, &mapping.ResponsibleUserID,
			&mapping.IsActive, &mapping.Priority, &mapping.CreatedAt, &mapping.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("ошибка сканирования маппинга: %w", err)
		}
		mappings = append(mappings, mapping)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка итерации по маппингам: %w", err)
	}

	return mappings, nil
}

func (r *Repository) GetActiveMappings(userID uint32, appID uint32) ([]models.CRMMapping, error) {
	ctxTimeout, cancel := context.WithTimeout(r.db.Context(), mode.GetSQLTimeToCancel())
	defer cancel()

	rows, err := r.db.Conn().QueryContext(ctxTimeout, `
		SELECT id, user_id, application_id, crm_config_id, field_mapping, pipeline_id, status_id,
		       responsible_user_id, is_active, priority, created_at, updated_at
		FROM crm_mappings
		WHERE user_id = ? AND application_id = ? AND is_active = TRUE
		ORDER BY priority DESC
	`, userID, appID)

	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			return nil, fmt.Errorf("тайм-аут (%d с) при получении активных маппингов: %w", mode.GetSQLTimeToCancel(), err)
		case errors.Is(err, context.Canceled):
			return nil, fmt.Errorf("операция отменена: %w", err)
		default:
			return nil, fmt.Errorf("ошибка получения активных маппингов: %w", err)
		}
	}
	defer func() { _ = rows.Close() }()

	var mappings []models.CRMMapping
	for rows.Next() {
		var mapping models.CRMMapping
		err := rows.Scan(&mapping.ID, &mapping.UserID, &mapping.ApplicationID, &mapping.CRMConfigID,
			&mapping.FieldMapping, &mapping.PipelineID, &mapping.StatusID, &mapping.ResponsibleUserID,
			&mapping.IsActive, &mapping.Priority, &mapping.CreatedAt, &mapping.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("ошибка сканирования маппинга: %w", err)
		}
		mappings = append(mappings, mapping)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка итерации по маппингам: %w", err)
	}

	return mappings, nil
}

func (r *Repository) UpdateMapping(mapping *models.CRMMapping) error {
	// Получаем часовой пояс пользователя
	tz, err := r.db.UserTimeZone(mapping.UserID)
	if err != nil {
		return fmt.Errorf("ошибка получения часового пояса пользователя: %w", err)
	}

	ctxTimeout, cancel := context.WithTimeout(r.db.Context(), mode.GetSQLTimeToCancel())
	defer cancel()

	_, err = r.db.Conn().ExecContext(ctxTimeout, `
		UPDATE crm_mappings
		SET user_id = ?, application_id = ?, crm_config_id = ?, field_mapping = ?, 
		    pipeline_id = ?, status_id = ?, responsible_user_id = ?, is_active = ?, 
		    priority = ?, updated_at = CONVERT_TZ(NOW(), @@session.time_zone, ?)
		WHERE id = ?
	`, mapping.UserID, mapping.ApplicationID, mapping.CRMConfigID, mapping.FieldMapping,
		mapping.PipelineID, mapping.StatusID, mapping.ResponsibleUserID, mapping.IsActive,
		mapping.Priority, tz, mapping.ID)

	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			return fmt.Errorf("тайм-аут (%d с) при обновлении маппинга: %w", mode.GetSQLTimeToCancel(), err)
		case errors.Is(err, context.Canceled):
			return fmt.Errorf("операция отменена: %w", err)
		default:
			return fmt.Errorf("ошибка обновления маппинга: %w", err)
		}
	}

	return nil
}

func (r *Repository) DeleteMapping(id uint32) error {
	ctxTimeout, cancel := context.WithTimeout(r.db.Context(), mode.GetSQLTimeToCancel())
	defer cancel()

	_, err := r.db.Conn().ExecContext(ctxTimeout, "DELETE FROM crm_mappings WHERE id = ?", id)
	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			return fmt.Errorf("тайм-аут (%d с) при удалении маппинга: %w", mode.GetSQLTimeToCancel(), err)
		case errors.Is(err, context.Canceled):
			return fmt.Errorf("операция отменена: %w", err)
		default:
			return fmt.Errorf("ошибка удаления маппинга: %w", err)
		}
	}

	return nil
}

// ============= OAuth States =============

func (r *Repository) SaveOAuthState(state *models.OAuthState) error {
	ctxTimeout, cancel := context.WithTimeout(r.db.Context(), mode.GetSQLTimeToCancel())
	defer cancel()

	if err := r.encryptOAuthStateFields(state); err != nil {
		return err
	}

	_, err := r.db.Conn().ExecContext(ctxTimeout, `
		INSERT INTO crm_oauth_states 
		(state, user_id, client_id, client_secret, redirect_url, subdomain, crm_type, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		user_id = VALUES(user_id),
		client_id = VALUES(client_id),
		client_secret = VALUES(client_secret),
		redirect_url = VALUES(redirect_url),
		subdomain = VALUES(subdomain),
		crm_type = VALUES(crm_type),
		expires_at = VALUES(expires_at)
	`, state.State, state.UserID, state.ClientID, state.ClientSecret, state.RedirectURL,
		state.Subdomain, state.CRMType, state.CreatedAt, state.ExpiresAt)

	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			return fmt.Errorf("тайм-аут (%d с) при сохранении OAuth state: %w", mode.GetSQLTimeToCancel(), err)
		case errors.Is(err, context.Canceled):
			return fmt.Errorf("операция отменена: %w", err)
		default:
			return fmt.Errorf("ошибка сохранения OAuth state: %w", err)
		}
	}

	return nil
}

func (r *Repository) GetOAuthState(state string) (*models.OAuthState, error) {
	ctxTimeout, cancel := context.WithTimeout(r.db.Context(), mode.GetSQLTimeToCancel())
	defer cancel()

	var oauthState models.OAuthState
	err := r.db.Conn().QueryRowContext(ctxTimeout, `
		SELECT state, user_id, client_id, client_secret, redirect_url, subdomain, crm_type, created_at, expires_at
		FROM crm_oauth_states
		WHERE state = ? AND expires_at > NOW()
	`, state).Scan(&oauthState.State, &oauthState.UserID, &oauthState.ClientID, &oauthState.ClientSecret,
		&oauthState.RedirectURL, &oauthState.Subdomain, &oauthState.CRMType, &oauthState.CreatedAt, &oauthState.ExpiresAt)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, fmt.Errorf("OAuth state не найден или истек")
		case errors.Is(err, context.DeadlineExceeded):
			return nil, fmt.Errorf("тайм-аут (%d с) при получении OAuth state: %w", mode.GetSQLTimeToCancel(), err)
		case errors.Is(err, context.Canceled):
			return nil, fmt.Errorf("операция отменена: %w", err)
		default:
			return nil, fmt.Errorf("ошибка получения OAuth state: %w", err)
		}
	}

	if err := r.decryptOAuthStateFields(&oauthState); err != nil {
		return nil, err
	}

	return &oauthState, nil
}

func (r *Repository) DeleteOAuthState(state string) error {
	ctxTimeout, cancel := context.WithTimeout(r.db.Context(), mode.GetSQLTimeToCancel())
	defer cancel()

	_, err := r.db.Conn().ExecContext(ctxTimeout, "DELETE FROM crm_oauth_states WHERE state = ?", state)
	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			return fmt.Errorf("тайм-аут (%d с) при удалении OAuth state: %w", mode.GetSQLTimeToCancel(), err)
		case errors.Is(err, context.Canceled):
			return fmt.Errorf("операция отменена: %w", err)
		default:
			return fmt.Errorf("ошибка удаления OAuth state: %w", err)
		}
	}

	return nil
}

func (r *Repository) CleanupExpiredOAuthStates() error {
	ctxTimeout, cancel := context.WithTimeout(r.db.Context(), mode.GetSQLTimeToCancel())
	defer cancel()

	_, err := r.db.Conn().ExecContext(ctxTimeout, "DELETE FROM crm_oauth_states WHERE expires_at < NOW()")
	if err != nil {
		return fmt.Errorf("ошибка очистки истекших OAuth states: %w", err)
	}

	return nil
}

func (r *Repository) decryptOAuthStateFields(state *models.OAuthState) error {
	if state == nil || r.db == nil || r.db.MasterKeyResolver == nil {
		return nil
	}

	masterKey, ok := r.db.MasterKeyResolver(state.UserID)
	if !ok {
		return nil
	}

	if crypto.IsEncryptedWithMasterKey(state.ClientID) {
		decrypted, err := crypto.DecryptFieldWithMasterKey(masterKey, state.ClientID)
		if err != nil {
			return fmt.Errorf("ошибка расшифровки ClientID: %w", err)
		}
		state.ClientID = decrypted
	}

	if crypto.IsEncryptedWithMasterKey(state.ClientSecret) {
		decrypted, err := crypto.DecryptFieldWithMasterKey(masterKey, state.ClientSecret)
		if err != nil {
			return fmt.Errorf("ошибка расшифровки ClientSecret: %w", err)
		}
		state.ClientSecret = decrypted
	}

	if crypto.IsEncryptedWithMasterKey(state.RedirectURL) {
		decrypted, err := crypto.DecryptFieldWithMasterKey(masterKey, state.RedirectURL)
		if err != nil {
			return fmt.Errorf("ошибка расшифровки RedirectURL: %w", err)
		}
		state.RedirectURL = decrypted
	}

	if crypto.IsEncryptedWithMasterKey(state.Subdomain) {
		decrypted, err := crypto.DecryptFieldWithMasterKey(masterKey, state.Subdomain)
		if err != nil {
			return fmt.Errorf("ошибка расшифровки Subdomain: %w", err)
		}
		state.Subdomain = decrypted
	}

	return nil
}

func (r *Repository) encryptOAuthStateFields(state *models.OAuthState) error {
	if state == nil || r.db == nil || r.db.MasterKeyResolver == nil {
		return nil
	}

	masterKey, ok := r.db.MasterKeyResolver(state.UserID)
	if !ok {
		return nil
	}

	if state.ClientID != "" && !crypto.IsEncryptedWithMasterKey(state.ClientID) {
		encrypted, err := crypto.EncryptFieldWithMasterKey(masterKey, state.ClientID)
		if err != nil {
			return fmt.Errorf("ошибка шифрования ClientID: %w", err)
		}
		state.ClientID = encrypted
	}

	if state.ClientSecret != "" && !crypto.IsEncryptedWithMasterKey(state.ClientSecret) {
		encrypted, err := crypto.EncryptFieldWithMasterKey(masterKey, state.ClientSecret)
		if err != nil {
			return fmt.Errorf("ошибка шифрования ClientSecret: %w", err)
		}
		state.ClientSecret = encrypted
	}

	if state.RedirectURL != "" && !crypto.IsEncryptedWithMasterKey(state.RedirectURL) {
		encrypted, err := crypto.EncryptFieldWithMasterKey(masterKey, state.RedirectURL)
		if err != nil {
			return fmt.Errorf("ошибка шифрования RedirectURL: %w", err)
		}
		state.RedirectURL = encrypted
	}

	if state.Subdomain != "" && !crypto.IsEncryptedWithMasterKey(state.Subdomain) {
		encrypted, err := crypto.EncryptFieldWithMasterKey(masterKey, state.Subdomain)
		if err != nil {
			return fmt.Errorf("ошибка шифрования Subdomain: %w", err)
		}
		state.Subdomain = encrypted
	}

	return nil
}
