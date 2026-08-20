# Marusia_CRM

![marusia_crm](marusia_crm_logo.png)

[🇬🇧 English version](README.md)

![Версия Go](https://img.shields.io/badge/Go-1.25.8-00ADD8?logo=go)
![Лицензия](https://img.shields.io/badge/license-MIT-blue)
[![Telegram](https://img.shields.io/badge/Telegram-Join%20Chat-blue?logo=telegram)](https://t.me/marusia_dev)

Микросервис-интегратор CRM для платформы AiR.

Важно: `marusia_crm` не является самостоятельной CRM-системой. У сервиса нет собственной пользовательской CRM-панели, воронки или базы клиентов. Он выступает промежуточным backend-слоем между платформой AiR и внешними CRM-системами. В текущей реализации поддерживается AmoCRM.

## Защита пользовательских данных

Все пользовательские данные шифруются индивидуальным `MasterKey`. Данный ключ доступен только по паролю пользователя и шифрует данные авторизации в CRM, токены и все прочие настройки. Расшифровка этих данных возможна только после авторизации пользователя в системе индивидуальным паролем пользователя. Даже в случае компрометации или утечки базы данных, все пользовательские данные останутся недоступны как для злоумышленников, так и для администрации сервиса.

## Назначение

Сервис получает запросы от платформы AiR, определяет CRM-конфигурацию пользователя и выполняет операции во внешней CRM через её API. Он скрывает детали авторизации, преобразования данных, хранения токенов и взаимодействия с AmoCRM от остальных сервисов платформы.

Основные возможности:

- подключение AmoCRM к пользовательскому аккаунту;
- OAuth-авторизация, callback и обновление токенов;
- проверка подключения к CRM;
- создание, получение и обновление контактов и лидов;
- поиск контактов по телефону и альтернативному идентификатору;
- получение воронок, статусов и бесед;
- добавление примечаний и диалогов к лидам;
- получение и создание пользовательских полей;
- настройка каналов, сопоставлений и параметров CRM;
- mock OAuth для локальной разработки;
- Prometheus-метрики и структурированное логирование.

## Архитектура

```text
Платформа AiR
      |
      | HTTP/JSON, X-User-ID
      v
  marusia_crm (:8080)
      |                    |
      |                    +--> air_orchestrator (gRPC)
      |                         конфигурация и master key
      |
      +--> MySQL             хранение конфигураций и данных
      +--> Redis             опциональный кеш OAuth-состояния
      |
      +--> AmoCRM API v4    внешняя CRM
```

При запуске приложение инициализирует gRPC-клиент, подключение к MySQL и CRM-модули. Master key пользователя запрашивается у `air_orchestrator`, а OAuth-состояния периодически очищаются. При завершении приложение корректно останавливает HTTP-сервер и закрывает соединения с базой данных.

## HTTP API

HTTP-сервер по умолчанию работает на порту `8080`.

Основной контракт доступен с префиксом `/v1/crm`:

```text
GET  /health
GET  /metrics
GET  /v1/crm/health

GET|POST|PATCH|DELETE /v1/crm/api/...
POST /v1/crm/oauth/amocrm/auth
GET  /v1/crm/oauth/amocrm/callback
POST /v1/crm/oauth/amocrm/refresh
POST /v1/crm/mock/oauth/amocrm/auth
POST /v1/crm/mock/oauth/amocrm/refresh
```

CRM-эндpoints требуют заголовок:

```text
X-User-ID: <идентификатор пользователя>
```

Публичными являются health-check, callback OAuth и endpoint метрик. Legacy-маршруты без префикса `/v1/crm` сохраняются для обратной совместимости. Полный контракт описан в [`doc/openapi.yaml`](doc/openapi.yaml).

## Технологии

- Go 1.25.8;
- Fiber v2 и fasthttp — HTTP-сервер;
- MySQL — постоянное хранилище;
- Redis — опциональное кеширование OAuth-состояния;
- REST/HTTP API AmoCRM v4;
- OAuth 2.0 — авторизация в AmoCRM;
- gRPC и Protocol Buffers — взаимодействие с `air_orchestrator`;
- `air_common` — общие модели, конфигурация, RPC и инфраструктурные компоненты;
- `air_logger` — структурированное логирование;
- Prometheus — сбор метрик;
- Docker и Docker Compose — запуск и развёртывание;
- OpenAPI 3.0.3 — описание HTTP API.

## Запуск

Для разработки:

```bash
docker compose -f dev.yml up --build
```

Для production:

```bash
docker compose -f prod.yml up -d
```

Для запуска необходимы внешние Docker-сети `air_shared` и `monitoring_shared`. В инфраструктуре должны быть доступны MySQL `air_db`, Redis `air_redis` и gRPC-сервис `air_orchestrator` (`airorc`).

## Конфигурация

Основные переменные окружения:

```text
DB_HOST=air_db:3306
DB_NAME=air
DB_USER=<пользователь MySQL>
DB_PASSWORD=<пароль MySQL>
REDIS_ADDR=air_redis:6379
REDIS_PASSWORD=
REDIS_DB=0
GRPC_CONFIG_HOST=airorc:50051
SERVICE_KEY_FILE=/run/secrets/service_key
REAL_URL=<домен или localhost>
LOG_LEVEL=info
```

`SERVICE_KEY_FILE` содержит сервисный ключ для защищённого взаимодействия с `air_orchestrator`. Настройки AmoCRM и OAuth credentials хранятся в базе данных и не задаются напрямую в Docker Compose.

## Структура проекта

- `cmd/main.go` — точка входа;
- `internal/app` — инициализация приложения и жизненный цикл;
- `internal/delivery/http` — HTTP-сервер и маршрутизация;
- `internal/crm/handlers` — обработчики API;
- `internal/domain` — доменные модели и CRM-сервисы;
- `internal/providers/amocrm` — клиент и операции AmoCRM;
- `internal/repository/mysql` — репозиторий MySQL;
- `internal/cache` — кеширование OAuth-состояния;
- `internal/metrics` — Prometheus-метрики;
- `doc/openapi.yaml` — спецификация API.

## Связанные сервисы

- [air_common](https://github.com/ikermy/air_common) — общая библиотека платформы AiR;
- [air_orchestrator](https://github.com/ikermy/air_orchestrator) — сервис-оркестратор;
- [air_logger](https://github.com/ikermy/air_logger) — инфраструктура логирования.

## Лицензия

Проект распространяется по лицензии [MIT](LICENSE). Она разрешает свободно использовать, копировать, изменять и распространять программное обеспечение при сохранении текста лицензии и уведомления об авторских правах.

Полный текст лицензии доступен в файле [`LICENSE`](LICENSE).

## Контакты
[![Telegram](https://img.shields.io/badge/Telegram-Contact-blue?logo=telegram)](https://t.me/ikermy)

