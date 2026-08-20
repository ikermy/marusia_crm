# Marusia_CRM

![marusia_crm](marusia_crm_logo.png)

[🇷🇺 Русская версия](README.ru.md)

![Go version](https://img.shields.io/badge/Go-1.25.8-00ADD8?logo=go)
![License](https://img.shields.io/badge/license-MIT-blue)
[![Telegram](https://img.shields.io/badge/Telegram-Join%20Chat-blue?logo=telegram)](https://t.me/marusia_dev)

CRM integration microservice for the AiR platform.

Important: `marusia_crm` is not a standalone CRM system. The service has no own user CRM panel, sales funnel, or customer database. It serves as an intermediate backend layer between the AiR platform and external CRM systems. The current implementation supports AmoCRM.

## User Data Protection

All user data is encrypted with an individual `MasterKey`. This key is accessible only with the user's password and encrypts CRM authorization data, tokens, and all other settings. This data can be decrypted only after the user has authenticated in the system with their individual password. Even if the database is compromised or leaked, all user data will remain inaccessible both to attackers and to the service administration.

## Purpose

The service receives requests from the AiR platform, determines the user's CRM configuration, and performs operations in the external CRM through its API. It hides the details of authorization, data transformation, token storage, and interaction with AmoCRM from the platform's other services.

Main capabilities:

- connecting AmoCRM to a user account;
- OAuth authorization, callback, and token refresh;
- checking the CRM connection;
- creating, retrieving, and updating contacts and leads;
- searching contacts by phone number and alternative identifier;
- retrieving pipelines, statuses, and conversations;
- adding notes and dialogues to leads;
- retrieving and creating custom fields;
- configuring CRM channels, mappings, and parameters;
- mock OAuth for local development;
- Prometheus metrics and structured logging.

## Architecture

```text
AiR platform
      |
      | HTTP/JSON, X-User-ID
      v
  marusia_crm (:8080)
      |                    |
      |                    +--> air_orchestrator (gRPC)
      |                         configuration and master key
      |
      +--> MySQL             configuration and data storage
      +--> Redis             optional OAuth state cache
      |
      +--> AmoCRM API v4    external CRM
```

On startup, the application initializes the gRPC client, the MySQL connection, and the CRM modules. The user's master key is requested from `air_orchestrator`, and OAuth states are periodically cleaned up. On shutdown, the application gracefully stops the HTTP server and closes the database connections.

## HTTP API

The HTTP server runs on port `8080` by default.

The main contract is available under the `/v1/crm` prefix:

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

CRM endpoints require the following header:

```text
X-User-ID: <user identifier>
```

The health check, OAuth callback, and metrics endpoints are public. Legacy routes without the `/v1/crm` prefix are retained for backward compatibility. The full contract is described in [`doc/openapi.yaml`](doc/openapi.yaml).

## Technologies

- Go 1.25.8;
- Fiber v2 and fasthttp — HTTP server;
- MySQL — persistent storage;
- Redis — optional OAuth state caching;
- AmoCRM v4 REST/HTTP API;
- OAuth 2.0 — AmoCRM authorization;
- gRPC and Protocol Buffers — communication with `air_orchestrator`;
- `air_common` — shared models, configuration, RPC, and infrastructure components;
- `air_logger` — structured logging;
- Prometheus — metrics collection;
- Docker and Docker Compose — running and deployment;
- OpenAPI 3.0.3 — HTTP API description.

## Running

For development:

```bash
docker compose -f dev.yml up --build
```

For production:

```bash
docker compose -f prod.yml up -d
```

The external Docker networks `air_shared` and `monitoring_shared` are required. The infrastructure must provide MySQL `air_db`, Redis `air_redis`, and the `air_orchestrator` gRPC service (`airorc`).

## Configuration

Main environment variables:

```text
DB_HOST=air_db:3306
DB_NAME=air
DB_USER=<MySQL user>
DB_PASSWORD=<MySQL password>
REDIS_ADDR=air_redis:6379
REDIS_PASSWORD=
REDIS_DB=0
GRPC_CONFIG_HOST=airorc:50051
SERVICE_KEY_FILE=/run/secrets/service_key
REAL_URL=<domain or localhost>
LOG_LEVEL=info
```

`SERVICE_KEY_FILE` contains the service key for secure communication with `air_orchestrator`. AmoCRM settings and OAuth credentials are stored in the database and are not set directly in Docker Compose.

## Project Structure

- `cmd/main.go` — entry point;
- `internal/app` — application initialization and lifecycle;
- `internal/delivery/http` — HTTP server and routing;
- `internal/crm/handlers` — API handlers;
- `internal/domain` — domain models and CRM services;
- `internal/providers/amocrm` — AmoCRM client and operations;
- `internal/repository/mysql` — MySQL repository;
- `internal/cache` — OAuth state caching;
- `internal/metrics` — Prometheus metrics;
- `doc/openapi.yaml` — API specification.

## Related Services

- [air_common](https://github.com/ikermy/air_common) — shared AiR platform library;
- [air_orchestrator](https://github.com/ikermy/air_orchestrator) — orchestration service;
- [air_logger](https://github.com/ikermy/air_logger) — logging infrastructure.

## License

The project is distributed under the [MIT](LICENSE) license. It permits the free use, copying, modification, and distribution of the software provided that the license text and copyright notice are retained.

The full license text is available in the [`LICENSE`](LICENSE) file.

## Contacts
[![Telegram](https://img.shields.io/badge/Telegram-Contact-blue?logo=telegram)](https://t.me/ikermy)
