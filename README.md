# WireChat Server

Минимальный, но аккуратно собранный WebSocket‑чат на Go. Проект задуман как учебный скелет: чистая доменная модель, чёткое разделение слоёв, внятный протокол и готовность к SDK.

## Возможности

- WebSocket API с версией протокола (`protocol: 1`), hello/join/leave/msg.
- Комнаты, широковещание сообщений, события user_joined/user_left.
- JWT‑handshake (HS256) с опциональными audience/issuer и требованием токена.
- Ограничения: максимальный размер входящего сообщения, rate‑limit join/msg, idle timeout.
- Структурированные логи (zerolog), конфиги через Viper, CLI на cobra.
- CI, Docker, бенчмарки и интеграционные тесты.

## Стек

- Go 1.25.x
- WebSocket: `nhooyr.io/websocket`
- Логи: `github.com/rs/zerolog`
- Конфиг: `github.com/spf13/viper`, CLI: `github.com/spf13/cobra`
- JWT: `github.com/golang-jwt/jwt/v5`

## Быстрый старт

### Локальный запуск

```bash
make build         # собрать бинарь
./bin/wirechat-server --addr :8080
```

или без сборки:

```bash
make run
```

### Docker

```bash
make docker
docker run --rm -p 8080:8080 wirechat-server:latest
```

### Конфиг

При первом старте создаётся `config.yaml` (путь можно задать `WIRECHAT_CONFIG_DEFAULT_PATH`). Пример: `config.example.yaml`.

Ключевые поля:

- `addr` — адрес HTTP/WS (`:8080`).
- `max_message_bytes` — лимит размера входящих сообщений.
- `rate_limit_join_per_min`, `rate_limit_msg_per_min` — лимиты на соединение.
- `client_idle_timeout` — дедлайн чтения (закрывает idle клиентов).
- JWT:
  - `jwt_required` (bool)
  - `jwt_secret` (HS256)
  - `jwt_audience`, `jwt_issuer`

### Переменные окружения

Все поля конфигурации можно задать через `WIRECHAT_*`, например:

- `WIRECHAT_ADDR=":9000"`
- `WIRECHAT_MAX_MESSAGE_BYTES=2097152`
- `WIRECHAT_JWT_REQUIRED=true`
- `WIRECHAT_JWT_SECRET=devsecret`

### Make цели

- `make build` — сборка в `bin/wirechat-server`
- `make run` — запуск `go run ./cmd/server`
- `make test` — тесты
- `make race` — тесты с `-race`
- `make bench` — бенчмарки
- `make lint` — golangci-lint
- `make fmt` — gofmt
- `make docker` — сборка контейнера

## Протокол

- Транспорт: WebSocket.
- Версия: `protocol: 1` в `hello`.
- Inbound типы: `hello`, `join`, `leave`, `msg`.
- Outbound типы: `event`, `error`.
- События: `message`, `user_joined`, `user_left`.
- Ошибки: `unsupported_version`, `unauthorized`, `invalid_message`, `bad_request`, `room_not_found`, `already_joined`, `not_in_room`, `rate_limited`.
- Подробности и примеры: [PROTOCOL_DRAFT.md](PROTOCOL_DRAFT.md).

## Примитивный тест вручную

1. Запускаем сервер: `make run`.
2. Подключаемся `websocat`:
   ```bash
   websocat ws://localhost:8080/ws
   ```
3. Отправляем:
   ```json
   {"type":"hello","data":{"protocol":1,"user":"alice"}}
   {"type":"join","data":{"room":"general"}}
   {"type":"msg","data":{"room":"general","text":"hi"}}
   ```
4. Получаем события `user_joined` и `message`.

## Тесты и бенчи

- Юнит-тесты core и интеграционные WS: `make test`.
- Гонки: `make race`.
- Бенчи broadcast: `make bench` (см. `internal/core/bench_test.go`).

## Структура

```
cmd/server       — точка входа (cobra, конфиг, логгер)
internal/app     — wiring hub + HTTP сервер
internal/core    — доменная логика (hub, rooms, clients, messages)
internal/transport/http — WS/HTTP сервер, маппинг proto↔core, лимиты, JWT
internal/proto   — wire-модели и константы протокола
internal/config  — конфиг + загрузчик (viper)
internal/log     — zerolog фабрика
internal/utils   — вспомогательные функции (IDs)
scripts          — вспомогательные клиенты
PROTOCOL_DRAFT.md — описание протокола
```

## Безопасность (минимум)

- Лимит размера сообщений, rate-limits, idle timeout.
- JWT в `hello` (HS256) с aud/iss и обязательностью по конфигу.
- Read limit/idle dedline на WS.

## Планы

- Дополнить README/DEV_GUIDE по мере развития.
- При появлении веб-клиента добавить CORS/Origin-check.
- Вынести JWT на JWKS или внешний IdP при необходимости.
