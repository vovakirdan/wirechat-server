# WireChat Protocol v1 (draft)

Транспорт: WebSocket поверх HTTP(S).

Версия протокола: `1` (поле `protocol` в `hello`).

## Envelopes

```json
// inbound (client -> server)
{
  "type": "<hello|join|leave|msg>",
  "data": { ... }
}
```

```json
// outbound (server -> client)
{
  "type": "event" | "error",
  "event": "<message|user_joined|user_left>",
  "data": { ... },
  "error": { "code": "<string>", "msg": "<string>" }
}
```

## Inbound messages

- `hello`
  - Обязателен первым сообщением, если включено `jwt_required`.
  - Поля:
    - `protocol` (int, optional) — номер протокола. Если указан и не равен `1` → `error` `unsupported_version`.
    - `token` (string, optional) — JWT, если авторизация включена.
    - `user` (string, optional) — fallback имя, если JWT не требуется/не задан.

- `join`
  - Подписка на комнату.
  - Поля: `room` (string, required).
  - Ошибки: `rate_limited`, `bad_request` (пустая room), `already_joined`.

- `leave`
  - Отписка от комнаты.
  - Поля: `room` (string, required).
  - Ошибки: `rate_limited`, `room_not_found`, `not_in_room`.

- `msg`
  - Отправка сообщения в комнату.
  - Поля: `room` (string, required), `text` (string, optional).
  - Ошибки: `rate_limited`, `bad_request`, `not_in_room`.

## Outbound events

- `event: "user_joined"`
  ```json
  { "room": "<room>", "user": "<user>" }
  ```
- `event: "user_left"`
  ```json
  { "room": "<room>", "user": "<user>" }
  ```
- `event: "message"`
  ```json
  { "room": "<room>", "user": "<user>", "text": "<text>", "ts": <unix_seconds> }
  ```
- `type: "error"`
  ```json
  { "error": { "code": "<string>", "msg": "<string>" } }
  ```

## Error codes (текущий набор)

- `unsupported_version` — неверный `hello.protocol`.
- `unauthorized` — отсутствует/невалидный JWT при `jwt_required`.
- `invalid_message` — неизвестный `type`.
- `bad_request` — неверные поля (пустая room и т.п.).
- `room_not_found` — попытка `leave` несуществующей комнаты.
- `already_joined` — повторный `join` комнаты.
- `not_in_room` — отправка/leave без членства.
- `rate_limited` — превышен лимит join/msg.

## Auth (JWT)

- Алгоритм: HS256.
- Настройки: `jwt_secret` (обязателен для валидации), `jwt_audience` (optional), `jwt_issuer` (optional), `jwt_required` (bool).
- Имя пользователя берётся из `name` claim, иначе `sub`. При отсутствии токена и `jwt_required=true` — `unauthorized`.

## Limits / таймауты

Конфигурируются через `config.yaml`:
- `max_message_bytes` — ограничение размера входящего сообщения.
- `rate_limit_join_per_min`, `rate_limit_msg_per_min` — лимиты на соединение; при превышении `rate_limited`.
- `client_idle_timeout` — дедлайн чтения; при истечении соединение закрывается.

## Примеры

Handshake (JWT):
```json
// client -> server
{ "type": "hello", "data": { "protocol": 1, "token": "<jwt>" } }
```

Join:
```json
{ "type": "join", "data": { "room": "general" } }
```

Message:
```json
{ "type": "msg", "data": { "room": "general", "text": "hi" } }
```

Ошибка версии:
```json
// server -> client
{ "type": "error", "error": { "code": "unsupported_version", "msg": "unsupported protocol version" } }
```
