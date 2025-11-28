# Dev Guide

Практический гид по работе с кодовой базой WireChat.

## Структура

- `cmd/server` — CLI (cobra), конфиг, логгер, запуск `App`.
- `internal/app` — связывает hub и HTTP сервер, graceful shutdown.
- `internal/core` — доменная логика (Hub, Client, Room, Message, команды/события).
- `internal/proto` — wire-модели и константы (`ProtocolVersion`, типы сообщений).
- `internal/transport/http` — HTTP/WS сервер, маппинг proto↔core, лимиты, JWT.
- `internal/config` — структура конфига + загрузчик (viper, env override, автосоздание файла).
- `internal/log` — фабрика zerolog.
- `internal/utils` — вспомогательные функции (ID).
- `scripts` — утилиты для ручной проверки (ws_chat, ws_smoke).
- `PROTOCOL_DRAFT.md` — описание протокола.

## Кодстайл и линтеры

- Формат: `make fmt` (gofmt/gofumpt).  
- Линты: `make lint` (golangci-lint, см. `.golangci.yaml`).  
- Тесты: `make test`, гонки `make race`, бенчи `make bench`.  
- CI (GitHub Actions) гоняет lint + test.

## Конфигурация

- Конфиг создаётся автоматически при первом старте (`config.yaml`), пример: `config.example.yaml`.
- Viper приоритет: defaults < файл < env (`WIRECHAT_*`) < CLI флаги.
- Основные параметры:
  - `addr`, `read_header_timeout`, `shutdown_timeout`
  - `max_message_bytes`, `rate_limit_join_per_min`, `rate_limit_msg_per_min`
  - `ping_interval`, `client_idle_timeout`
  - JWT: `jwt_required`, `jwt_secret`, `jwt_audience`, `jwt_issuer`

## Протокол

- Версия: `ProtocolVersion = 1` (hello.protocol).  
- Inbound: `hello`, `join`, `leave`, `msg`.  
- Outbound: `event` (`message`, `user_joined`, `user_left`) и `error`.  
- Ошибки: `unsupported_version`, `unauthorized`, `invalid_message`, `bad_request`, `room_not_found`, `already_joined`, `not_in_room`, `rate_limited`.  
- Детали: `PROTOCOL_DRAFT.md`.

## Логирование

- zerolog, создаётся в `cmd/server` и передаётся в app/transport.
- Логируются подключения/отключения, inbound команды, outbound события, ошибки, rate-limit, протокольные ошибки.
- Уровень задаётся флагом `--log-level`.

## Тесты

- Core: join/leave/broadcast, ошибки (already_joined, not_in_room, room_not_found).
- Transport: hello/join/msg e2e, oversize message, rate-limit, shutdown, JWT success/invalid, unsupported_version.
- Бенчи: broadcast на 10/100/500 клиентов (`internal/core/bench_test.go`).

## Запуск

- Локально: `make run` или `make build && ./bin/wirechat-server`.
- Docker: `make docker && docker run -p 8080:8080 wirechat-server:latest`.
- Конфиг путь: `--config` флаг или env `WIRECHAT_CONFIG_DEFAULT_PATH`.

## Добавление фич

- Слои не смешивать: core без зависимостей на transport/JSON; transport только адаптирует протокол ↔ core.
- Новые команды/события: добавить в core (Command/Event), маппинг в `internal/transport/http/mapper.go`, протокол в `internal/proto`, тесты (core + transport).
- Лимиты/таймауты: в конфиг + handler; обязательно тест.
- Изменения протокола: обновить константы/описание в `PROTOCOL_DRAFT.md` и тесты на handshake.

## Полезные команды

```bash
make fmt lint test      # формат + линты + тесты
make race               # гонки
make bench              # бенчи core
make docker             # сборка контейнера
make ci                 # fmt + lint + test
```
