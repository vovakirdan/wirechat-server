# WireChat Protocol v1

**Version**: 1
**Transport**: WebSocket over HTTP(S)
**Message Format**: JSON

---

## Table of Contents

- [Overview](#overview)
- [Connection Lifecycle](#connection-lifecycle)
- [Message Envelopes](#message-envelopes)
- [Inbound Messages (Client → Server)](#inbound-messages-client--server)
- [Outbound Events (Server → Client)](#outbound-events-server--client)
- [Error Codes](#error-codes)
- [Authentication](#authentication)
- [Room Types & Access Control](#room-types--access-control)
- [Rate Limiting](#rate-limiting)
- [WebSocket Keepalive](#websocket-keepalive)
- [Call Protocol (Voice/Video Calls)](#call-protocol-voicevideo-calls)
- [REST API](#rest-api)
- [SDK Implementation Contract](#sdk-implementation-contract)
- [Examples](#examples)

---

## Overview

WireChat is a real-time chat protocol built on WebSocket. It supports:
- **Room-based messaging**: Users join rooms to send/receive messages
- **Authentication**: JWT-based auth with guest mode support
- **Room types**: Public rooms (anyone can join), private rooms (invite-only), direct messages (1-on-1)
- **Message persistence**: Messages are saved to database and retrievable via REST API
- **Real-time events**: User join/leave notifications, message broadcasts

---

## Connection Lifecycle

1. **Client connects** to WebSocket endpoint: `ws://host:port/ws`
2. **Client sends `hello` message** with protocol version and optional JWT token
3. **Server validates** protocol version and token (if required)
4. **Client sends commands**: `join`, `leave`, `msg`
5. **Server sends events**: `user_joined`, `user_left`, `message`, `history`, `error`
6. **Client disconnects** gracefully or due to timeout/error

---

## Message Envelopes

### Inbound Envelope (Client → Server)

```json
{
  "type": "hello" | "join" | "leave" | "msg",
  "data": { ... }
}
```

- **type** (string, required): Message type
- **data** (object, required): Message payload (raw JSON)

### Outbound Envelope (Server → Client)

```json
{
  "type": "event" | "error",
  "event": "message" | "user_joined" | "user_left" | "history",
  "room": "<room_name>",
  "user": "<username>",
  "text": "<message_text>",
  "id": <message_id>,
  "ts": <unix_timestamp>,
  "messages": [ ... ],
  "error": {
    "code": "<error_code>",
    "msg": "<error_message>"
  }
}
```

- **type** (string, required): `"event"` or `"error"`
- **event** (string, optional): Event type (when `type == "event"`)
- **error** (object, optional): Error details (when `type == "error"`)
- Other fields depend on event type (see [Outbound Events](#outbound-events-server--client))

---

## Inbound Messages (Client → Server)

### `hello` - Handshake

**Must be sent first** after connecting.

```json
{
  "type": "hello",
  "data": {
    "protocol": 1,
    "token": "<jwt_token>",
    "user": "<username>"
  }
}
```

**Fields**:
- `protocol` (int, optional): Protocol version. If omitted, defaults to 1. If specified and not `1`, server returns `unsupported_version` error.
- `token` (string, optional): JWT authentication token. Required if server has `jwt_required=true`.
- `user` (string, optional): Fallback username for guest mode (when token not provided and `jwt_required=false`). If omitted, server generates username like `guest-<client_id>`.

**Errors**:
- `unsupported_version`: Protocol version mismatch
- `unauthorized`: Missing or invalid JWT when `jwt_required=true`

---

### `join` - Join Room

Subscribe to a room to receive messages and events.

```json
{
  "type": "join",
  "data": {
    "room": "general"
  }
}
```

**Fields**:
- `room` (string, required): Room name to join

**Behavior**:
- **Public rooms**: Anyone can join
- **Private rooms**: Only members (users in `room_members` table) can join
- **Direct rooms**: Only the two participants can join
- Upon successful join:
  1. Server broadcasts `user_joined` event to all room members
  2. Server sends `history` event to joining client with last 20 messages

**Errors**:
- `bad_request`: Empty or invalid room name
- `already_joined`: Client already in this room
- `access_denied`: User not authorized to join (private/direct rooms)
- `rate_limited`: Too many join requests (see [Rate Limiting](#rate-limiting))

---

### `leave` - Leave Room

Unsubscribe from a room.

```json
{
  "type": "leave",
  "data": {
    "room": "general"
  }
}
```

**Fields**:
- `room` (string, required): Room name to leave

**Behavior**:
- Server broadcasts `user_left` event to remaining room members
- Client stops receiving messages/events from this room

**Errors**:
- `bad_request`: Empty or invalid room name
- `room_not_found`: Room does not exist
- `not_in_room`: Client not a member of this room

---

### `msg` - Send Message

Send a message to a room.

```json
{
  "type": "msg",
  "data": {
    "room": "general",
    "text": "Hello, world!"
  }
}
```

**Fields**:
- `room` (string, required): Room name
- `text` (string, required): Message content

**Behavior**:
- **For authenticated users**: Message is saved to database before broadcast, assigned an ID
- **For guest users**: Message is broadcast but not persisted (ID will be 0)
- Server broadcasts `message` event to all room members (including sender)

**Errors**:
- `bad_request`: Empty room or missing text
- `not_in_room`: Client must join room first
- `rate_limited`: Too many messages (see [Rate Limiting](#rate-limiting))

---

## Outbound Events (Server → Client)

### `event: "user_joined"` - User Joined Room

Broadcasted to all room members when a user joins.

```json
{
  "type": "event",
  "event": "user_joined",
  "room": "general",
  "user": "alice"
}
```

**Fields**:
- `room` (string): Room name
- `user` (string): Username of joining user

---

### `event: "user_left"` - User Left Room

Broadcasted to remaining room members when a user leaves.

```json
{
  "type": "event",
  "event": "user_left",
  "room": "general",
  "user": "alice"
}
```

**Fields**:
- `room` (string): Room name
- `user` (string): Username of leaving user

---

### `event: "message"` - New Message

Broadcasted to all room members when a message is sent.

```json
{
  "type": "event",
  "event": "message",
  "room": "general",
  "user": "alice",
  "text": "Hello, world!",
  "id": 12345,
  "ts": 1701234567
}
```

**Fields**:
- `room` (string): Room name
- `user` (string): Username of sender
- `text` (string): Message content
- `id` (int64): Message ID from database (0 for guest user messages)
- `ts` (int64): Unix timestamp (seconds since epoch)

---

### `event: "history"` - Message History

Sent to joining client with recent messages from the room.

```json
{
  "type": "event",
  "event": "history",
  "room": "general",
  "messages": [
    {
      "id": 12343,
      "room": "general",
      "user": "alice",
      "text": "Hi there",
      "ts": 1701234500
    },
    {
      "id": 12344,
      "room": "general",
      "user": "bob",
      "text": "Hello!",
      "ts": 1701234550
    }
  ]
}
```

**Fields**:
- `room` (string): Room name
- `messages` (array): Array of message objects (last 20 messages, chronological order)
  - Each message has: `id`, `room`, `user`, `text`, `ts`

**Behavior**:
- Sent only to joining client (unicast, not broadcast)
- Includes last 20 messages from database
- Best-effort: If room doesn't exist in DB or fetch fails, no history is sent (join still succeeds)

---

### `type: "error"` - Error Response

Sent when a client command fails.

```json
{
  "type": "error",
  "error": {
    "code": "not_in_room",
    "msg": "you must join the room first"
  }
}
```

**Fields**:
- `code` (string): Error code (see [Error Codes](#error-codes))
- `msg` (string): Human-readable error description

---

## Error Codes

| Code | Description | Triggered By |
|------|-------------|--------------|
| `unsupported_version` | Protocol version mismatch | `hello` with invalid `protocol` |
| `unauthorized` | Missing or invalid JWT token | `hello` when `jwt_required=true` |
| `invalid_message` | Unknown message type | Any inbound message with invalid `type` |
| `bad_request` | Invalid fields (empty room, etc.) | Any command with malformed data |
| `room_not_found` | Room does not exist | `leave` on non-existent room |
| `already_joined` | Already a member of room | `join` when already joined |
| `not_in_room` | Not a member of room | `msg`, `leave` without prior `join` |
| `access_denied` | Not authorized for this room | `join` private/direct room without membership |
| `rate_limited` | Too many requests | Exceeding `rate_limit_join_per_min` or `rate_limit_msg_per_min` |
| `internal_error` | Server-side error | Database failures, etc. |

---

## Authentication

### JWT-based Authentication

**Algorithm**: HS256
**Token Delivery**: `hello` message `token` field

#### JWT Configuration (Server)

```yaml
jwt_secret: "your-secret-key"         # Required for validation
jwt_audience: "wirechat"              # Optional, validated if set
jwt_issuer: "wirechat-server"         # Optional, validated if set
jwt_required: false                   # If true, rejects connections without valid token
```

#### JWT Claims

```json
{
  "user_id": 123,
  "username": "alice",
  "is_guest": false,
  "aud": ["wirechat"],
  "iss": "wirechat-server",
  "exp": 1701234567,
  "iat": 1701148167
}
```

**Standard Claims**:
- `user_id` (int64): Database user ID
- `username` (string): Display name
- `is_guest` (bool): `true` for guest users, `false` for registered users
- `aud` (array): Audience (validated against `jwt_audience`)
- `iss` (string): Issuer (validated against `jwt_issuer`)
- `exp` (int64): Expiration time (Unix timestamp)
- `iat` (int64): Issued at time (Unix timestamp)

**Token Generation**: Obtain via REST API (see [REST API - Authentication](#authentication-1))

---

### Guest Mode

If `jwt_required=false` (default), clients can connect without authentication:

```json
{
  "type": "hello",
  "data": {
    "protocol": 1,
    "user": "GuestUser"
  }
}
```

**Behavior**:
- Client is marked as guest (`is_guest=true`)
- Username defaults to `guest-<client_id>` if not provided
- Messages from guests are **not persisted** to database (ID will be 0)
- Guests can join public rooms but cannot create rooms or access private rooms

---

## Room Types & Access Control

### Room Types

| Type | Description | Creation | Access Control |
|------|-------------|----------|----------------|
| **public** | Open to everyone | REST API: `POST /api/rooms` with `type: "public"` | Anyone can join via WebSocket |
| **private** | Invite-only | REST API: `POST /api/rooms` with `type: "private"` | Only members in `room_members` table can join |
| **direct** | 1-on-1 private chat | REST API: `POST /api/rooms/direct` | Only the two participants can join |

### Access Control Rules

**WebSocket `join` command**:
- **Public rooms**: ✅ Anyone can join (no membership check)
- **Private rooms**: ✅ Only if user is in `room_members` table
- **Direct rooms**: ✅ Only if user is one of the two participants in `room_members`

**REST API** (see [REST API - Room Management](#room-management)):
- Room creation requires authentication
- Private room membership managed by room owner
- Direct rooms automatically add both participants to `room_members`

---

## Rate Limiting

Per-connection rate limits prevent abuse:

```yaml
rate_limit_join_per_min: 60    # Max join commands per minute
rate_limit_msg_per_min: 300    # Max message commands per minute
```

**Behavior**:
- Limits reset every minute
- Exceeded limit → `rate_limited` error
- Limits apply per WebSocket connection, not per user

---

## WebSocket Keepalive

**Purpose**: Detect dead connections and close them gracefully.

**Mechanism**:
1. Server sends WebSocket **ping frame** every `ping_interval` (default: 30s)
2. Client **automatically responds** with pong frame (handled by WebSocket library)
3. If no activity (JSON messages or pong) within `client_idle_timeout` (default: 90s), connection is closed

**Configuration**:
```yaml
ping_interval: 30s           # How often server sends ping
client_idle_timeout: 90s     # Inactivity timeout (3x ping_interval)
```

**Client Behavior**:
- **Do not set read timeout** on WebSocket connection (or set to 0/infinite)
- WebSocket library automatically handles pong responses
- Messages arrive sporadically in chat, so blocking read is expected

**Implementation Note**: WireChat uses `github.com/coder/websocket` library, which automatically handles pong frames during Read operations. The `client_idle_timeout` is applied via context timeout on each JSON read.

---

## Call Protocol (Voice/Video Calls)

WireChat supports voice and video calls via LiveKit integration. Call signaling happens over WebSocket; actual media is handled by LiveKit.

**Requirements**:
- LiveKit must be enabled on server (`livekit.enabled: true`)
- User must be authenticated (guests cannot make calls)
- For direct calls: users must be friends (unless target allows calls from everyone)

### Call State Machine

```
IDLE ──(call.invite)──> RINGING ──(call.accept)──> ACTIVE ──(call.leave/end)──> ENDED
                           │                          │
                           └──(call.reject)──> ENDED  │
                                                      │
                           (timeout)──────────────────┘
```

---

### Call Inbound Messages (Client → Server)

#### `call.invite` - Initiate Call

Start a direct call to a user or room call.

**Direct call**:
```json
{
  "type": "call.invite",
  "data": {
    "call_type": "direct",
    "to_user_id": 456
  }
}
```

**Room call**:
```json
{
  "type": "call.invite",
  "data": {
    "call_type": "room",
    "room_id": 123
  }
}
```

**Fields**:
- `call_type` (string, required): `"direct"` or `"room"`
- `to_user_id` (int64): Target user ID (required for direct calls)
- `room_id` (int64): Room ID (required for room calls)

**Behavior**:
- Creates call record in database with status `ringing`
- Direct calls: Sends `call.incoming` to target user, `call.ringing` to initiator
- Room calls: Sends `call.incoming` to all room members (except initiator)

**Errors**:
- `unauthorized`: Guest users cannot make calls
- `bad_request`: Missing `to_user_id` or `room_id`
- `calls_disabled`: LiveKit not enabled on server
- `not_friends`: Cannot call user who is not a friend (if target requires friends_only)
- `calls_not_allowed`: Target user does not accept calls from non-friends
- `rate_limited`: Too many call requests

---

#### `call.accept` - Accept Incoming Call

Accept an incoming call and receive LiveKit join credentials.

```json
{
  "type": "call.accept",
  "data": {
    "call_id": "uuid-string"
  }
}
```

**Fields**:
- `call_id` (string, required): Call UUID from `call.incoming` event

**Behavior**:
- Updates call status to `active`
- Sends `call.join-info` with LiveKit credentials to acceptor
- Sends `call.accepted` to initiator
- Sends `call.join-info` to initiator

**Errors**:
- `call_not_found`: Call does not exist
- `call_ended`: Call has already ended
- `not_participant`: User is not a participant in this call

---

#### `call.reject` - Reject Incoming Call

Reject an incoming call.

```json
{
  "type": "call.reject",
  "data": {
    "call_id": "uuid-string",
    "reason": "busy"
  }
}
```

**Fields**:
- `call_id` (string, required): Call UUID
- `reason` (string, optional): Rejection reason (`"busy"`, `"declined"`, `"unavailable"`)

**Behavior**:
- Updates call status to `ended`
- Sends `call.rejected` to initiator
- Sends `call.ended` to all participants

---

#### `call.join` - Join Active Call

Join or rejoin an active call (for reconnection or late joining).

```json
{
  "type": "call.join",
  "data": {
    "call_id": "uuid-string"
  }
}
```

**Fields**:
- `call_id` (string, required): Call UUID

**Behavior**:
- Sends `call.join-info` with LiveKit credentials
- Sends `call.participant-joined` to other participants

**Errors**:
- `call_not_found`: Call does not exist
- `call_ended`: Call has ended
- `not_participant`: User is not a participant

---

#### `call.leave` - Leave Call

Leave an active call (participant leaves, call may continue for others).

```json
{
  "type": "call.leave",
  "data": {
    "call_id": "uuid-string"
  }
}
```

**Fields**:
- `call_id` (string, required): Call UUID

**Behavior**:
- Updates participant `left_at` timestamp
- Sends `call.participant-left` to remaining participants
- If all participants have left, call status changes to `ended`

---

#### `call.end` - End Call

End the call for all participants (typically by initiator).

```json
{
  "type": "call.end",
  "data": {
    "call_id": "uuid-string"
  }
}
```

**Fields**:
- `call_id` (string, required): Call UUID

**Behavior**:
- Updates call status to `ended`
- Sends `call.ended` to all participants
- Terminates LiveKit room (if supported by engine)

---

### Call Outbound Events (Server → Client)

#### `event: "call.incoming"` - Incoming Call

Sent to target user(s) when someone initiates a call.

```json
{
  "type": "event",
  "event": "call.incoming",
  "data": {
    "call_id": "uuid-string",
    "call_type": "direct",
    "from_user_id": 123,
    "from_username": "alice",
    "room_id": null,
    "room_name": null,
    "created_at": 1702000000
  }
}
```

**Fields**:
- `call_id` (string): Unique call identifier
- `call_type` (string): `"direct"` or `"room"`
- `from_user_id` (int64): Initiator's user ID
- `from_username` (string): Initiator's username
- `room_id` (int64, nullable): Room ID for room calls
- `room_name` (string, nullable): Room name for room calls
- `created_at` (int64): Unix timestamp when call was created

---

#### `event: "call.ringing"` - Call Ringing

Sent to initiator confirming the call is ringing.

```json
{
  "type": "event",
  "event": "call.ringing",
  "data": {
    "call_id": "uuid-string",
    "to_user_id": 456,
    "to_username": "bob"
  }
}
```

**Fields**:
- `call_id` (string): Call identifier
- `to_user_id` (int64): Target user ID
- `to_username` (string): Target username

---

#### `event: "call.accepted"` - Call Accepted

Sent to initiator when target accepts the call.

```json
{
  "type": "event",
  "event": "call.accepted",
  "data": {
    "call_id": "uuid-string",
    "accepted_by_user_id": 456,
    "accepted_by_username": "bob"
  }
}
```

---

#### `event: "call.rejected"` - Call Rejected

Sent to initiator when target rejects the call.

```json
{
  "type": "event",
  "event": "call.rejected",
  "data": {
    "call_id": "uuid-string",
    "rejected_by_user_id": 456,
    "reason": "busy"
  }
}
```

---

#### `event: "call.join-info"` - LiveKit Join Credentials

Sent when user should connect to LiveKit (after accept or join).

```json
{
  "type": "event",
  "event": "call.join-info",
  "data": {
    "call_id": "uuid-string",
    "url": "ws://localhost:7880",
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "room_name": "wirechat-direct-uuid",
    "identity": "user-123"
  }
}
```

**Fields**:
- `call_id` (string): Call identifier
- `url` (string): LiveKit WebSocket URL
- `token` (string): LiveKit JWT token (valid for 1 hour)
- `room_name` (string): LiveKit room name
- `identity` (string): User's identity in LiveKit room

**Client Action**: Use these credentials to connect to LiveKit using their SDK.

---

#### `event: "call.participant-joined"` - Participant Joined

Sent to participants when someone joins an active call.

```json
{
  "type": "event",
  "event": "call.participant-joined",
  "data": {
    "call_id": "uuid-string",
    "user_id": 789,
    "username": "charlie"
  }
}
```

---

#### `event: "call.participant-left"` - Participant Left

Sent to participants when someone leaves the call.

```json
{
  "type": "event",
  "event": "call.participant-left",
  "data": {
    "call_id": "uuid-string",
    "user_id": 789,
    "username": "charlie",
    "reason": "left"
  }
}
```

**Reason values**: `"left"` (voluntary), `"disconnected"` (connection lost)

---

#### `event: "call.ended"` - Call Ended

Sent to all participants when the call ends.

```json
{
  "type": "event",
  "event": "call.ended",
  "data": {
    "call_id": "uuid-string",
    "ended_by_user_id": 123,
    "reason": "ended"
  }
}
```

**Reason values**: `"ended"` (normal), `"rejected"`, `"timeout"`, `"failed"`

---

### Call Error Codes

| Code | Description | Triggered By |
|------|-------------|--------------|
| `calls_disabled` | LiveKit not enabled | Any call command when LiveKit disabled |
| `call_not_found` | Call does not exist | `accept`, `reject`, `join`, `leave`, `end` with invalid call_id |
| `call_ended` | Call has already ended | Any action on ended call |
| `not_participant` | Not a call participant | Actions by non-participants |
| `not_friends` | Users are not friends | Direct call to non-friend (if required) |
| `calls_not_allowed` | Target blocks non-friend calls | Direct call when target has `friends_only` setting |

---

## REST API

Base URL: `http://host:port/api`

All authenticated endpoints require `Authorization: Bearer <token>` header.

---

### Authentication

#### `POST /api/register` - Register User

Create a new user account.

**Request**:
```json
{
  "username": "alice",
  "password": "securepassword123"
}
```

**Response** (201 Created):
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**Validation**:
- Username: 3-32 characters, unique
- Password: minimum 6 characters

**Errors**:
- `400 Bad Request`: Invalid request body or validation failure
- `409 Conflict`: Username already exists

---

#### `POST /api/login` - Login

Authenticate with existing credentials.

**Request**:
```json
{
  "username": "alice",
  "password": "securepassword123"
}
```

**Response** (200 OK):
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**Errors**:
- `400 Bad Request`: Invalid request body
- `401 Unauthorized`: Invalid credentials

---

#### `POST /api/guest` - Create Guest User

Create a temporary guest user (no password required).

**Request**: Empty body

**Response** (200 OK):
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**Additional**:
- Sets `guest_session` cookie (7-day expiry, httpOnly)
- Guest username format: `guest_<session_id_prefix>`
- Guest messages are not persisted

---

### Room Management

#### `POST /api/rooms` - Create Room

Create a public or private room.

**Request**:
```json
{
  "name": "general",
  "type": "public"
}
```

**Fields**:
- `name` (string, required): Unique room name
- `type` (string, optional): `"public"` (default) or `"private"`

**Response** (201 Created):
```json
{
  "id": 1,
  "name": "general",
  "type": "public",
  "owner_id": 123,
  "created_at": "2025-12-02T12:00:00Z"
}
```

**Errors**:
- `400 Bad Request`: Invalid request or duplicate name
- `401 Unauthorized`: Missing or invalid token

---

#### `GET /api/rooms` - List Rooms

List all accessible rooms for the authenticated user.

**Response** (200 OK):
```json
[
  {
    "id": 1,
    "name": "general",
    "type": "public",
    "owner_id": null,
    "created_at": "2025-12-02T12:00:00Z"
  },
  {
    "id": 2,
    "name": "secret-club",
    "type": "private",
    "owner_id": 123,
    "created_at": "2025-12-02T13:00:00Z"
  }
]
```

**Included Rooms**:
- All public rooms
- Private rooms where user is a member
- Direct rooms where user is a participant
- Rooms owned by the user

---

#### `POST /api/rooms/direct` - Create/Get Direct Room

Create or retrieve a direct message room between two users.

**Request**:
```json
{
  "user_id": 456
}
```

**Response** (200 OK):
```json
{
  "id": 3,
  "name": "dm-123-456",
  "type": "direct",
  "created_at": "2025-12-02T14:00:00Z"
}
```

**Behavior**:
- **Idempotent**: Calling multiple times returns the same room
- **Reversible**: `user1→user2` and `user2→user1` return the same room
- Automatically adds both users to `room_members`
- Room name format: `dm-<min_user_id>-<max_user_id>`

**Errors**:
- `400 Bad Request`: Cannot create DM with yourself or invalid user_id
- `401 Unauthorized`: Missing or invalid token

---

#### `POST /api/rooms/:id/join` - Join Public Room

Add user to `room_members` for a public room.

**Response** (200 OK):
```json
{
  "message": "joined room"
}
```

**Behavior**:
- Only works for **public rooms**
- Private rooms return `403 Forbidden`
- User must still send WebSocket `join` command to receive real-time messages

**Errors**:
- `403 Forbidden`: Room is private
- `404 Not Found`: Room does not exist

---

#### `DELETE /api/rooms/:id/leave` - Leave Room

Remove user from `room_members`.

**Response** (200 OK):
```json
{
  "message": "left room"
}
```

---

#### `POST /api/rooms/:id/members` - Add Member (Owner Only)

Add a user to a private room.

**Request**:
```json
{
  "user_id": 456
}
```

**Response** (200 OK):
```json
{
  "message": "member added"
}
```

**Authorization**: Only room owner can add members

**Errors**:
- `403 Forbidden`: Not room owner

---

#### `DELETE /api/rooms/:id/members/:userId` - Remove Member (Owner Only)

Remove a user from a private room.

**Response** (200 OK):
```json
{
  "message": "member removed"
}
```

**Authorization**: Only room owner can remove members

---

### Message History

#### `GET /api/rooms/:id/messages` - Get Message History

Retrieve paginated message history for a room.

**Query Parameters**:
- `limit` (int, optional): Number of messages to return (default: 50, max: 100)
- `before` (int64, optional): Cursor - return messages with `id < before`

**Example**: `GET /api/rooms/1/messages?limit=20&before=12345`

**Response** (200 OK):
```json
{
  "messages": [
    {
      "id": 12344,
      "room_id": 1,
      "user_id": 123,
      "user": "alice",
      "body": "Hello!",
      "created_at": "2025-12-02T12:00:00Z"
    },
    {
      "id": 12343,
      "room_id": 1,
      "user_id": 456,
      "user": "bob",
      "body": "Hi there",
      "created_at": "2025-12-02T11:59:50Z"
    }
  ],
  "has_more": true
}
```

**Fields**:
- `messages` (array): Messages in **reverse chronological order** (newest first)
- `has_more` (bool): `true` if more messages exist (for pagination)

**Pagination**:
1. Client requests first page: `GET /api/rooms/1/messages?limit=50`
2. Client stores `oldest_seen_id = messages[last].id`
3. Client requests next page: `GET /api/rooms/1/messages?limit=50&before=<oldest_seen_id>`
4. Repeat until `has_more == false`

**Access Control**:
- User must be a member of the room to retrieve history

---

## SDK Implementation Contract

This section defines requirements for client SDK implementers.

### Core Requirements

1. **Protocol Compliance**
   - MUST send `hello` as first message
   - MUST set `protocol: 1` in hello
   - MUST handle all outbound event types: `message`, `user_joined`, `user_left`, `history`, `error`
   - MUST parse error codes and expose them to application

2. **WebSocket Configuration**
   - MUST set read timeout to **0 (infinite)** or very large value
   - MUST NOT use short read timeouts (chat messages arrive sporadically)
   - WebSocket library will automatically handle pong responses

3. **Authentication**
   - If using JWT: MUST include token in `hello` message
   - If token invalid: MUST handle `unauthorized` error
   - If token expires: MUST provide mechanism to reconnect with new token

4. **Event Handling**
   - MUST provide event handler interface (callbacks or async iterators)
   - MUST deliver events to application in order received
   - SHOULD provide handlers for each event type: `onMessage`, `onUserJoined`, `onUserLeft`, `onHistory`, `onError`

5. **Thread Safety**
   - Send operations (`join`, `leave`, `send_message`) MUST be thread-safe (goroutine-safe/async-safe)
   - `connect` and `close` operations MAY NOT be thread-safe (single-threaded lifecycle expected)

### Recommended Features

1. **Automatic Reconnection**
   - SHOULD attempt reconnection on unexpected disconnect
   - SHOULD use exponential backoff (e.g., 1s, 2s, 4s, 8s, max 30s)
   - SHOULD re-send `hello` and re-join rooms after reconnect
   - MUST provide option to disable auto-reconnect

2. **Error Recovery**
   - SHOULD expose connection state to application (connecting, connected, disconnected, error)
   - SHOULD provide error callbacks for network errors, protocol errors, etc.

3. **Message Buffering**
   - MAY buffer outgoing messages when disconnected
   - If buffering: SHOULD flush buffer on reconnect
   - SHOULD have configurable buffer size limit

4. **Logging**
   - SHOULD provide structured logging (debug, info, warn, error levels)
   - SHOULD allow application to inject custom logger

### Language-Specific Considerations

**Go**:
- Use channels for event delivery OR callback-based API
- Provide `context.Context` for cancellation
- Connection read loop in separate goroutine

**Python**:
- Use async/await (`asyncio`)
- Provide event handlers as async functions
- Use `websockets` library for WebSocket client

**JavaScript/TypeScript**:
- Use EventEmitter or Promise-based API
- Support both Node.js and browser environments
- Use `ws` (Node.js) or browser WebSocket API

---

## Examples

### Complete Flow: Register → Create Room → Join → Send Message

**1. Register User (REST)**

```bash
curl -X POST http://localhost:8080/api/register \
  -H "Content-Type: application/json" \
  -d '{"username":"alice","password":"secret123"}'
```

Response:
```json
{"token":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."}
```

---

**2. Create Public Room (REST)**

```bash
curl -X POST http://localhost:8080/api/rooms \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"name":"general","type":"public"}'
```

Response:
```json
{"id":1,"name":"general","type":"public","owner_id":123,"created_at":"2025-12-02T12:00:00Z"}
```

---

**3. Connect to WebSocket**

```
ws://localhost:8080/ws
```

---

**4. Send Hello (WebSocket)**

```json
{
  "type": "hello",
  "data": {
    "protocol": 1,
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  }
}
```

---

**5. Join Room (WebSocket)**

```json
{
  "type": "join",
  "data": {
    "room": "general"
  }
}
```

Server sends `user_joined` event:
```json
{
  "type": "event",
  "event": "user_joined",
  "room": "general",
  "user": "alice"
}
```

Server sends `history` event with last 20 messages (if any):
```json
{
  "type": "event",
  "event": "history",
  "room": "general",
  "messages": []
}
```

---

**6. Send Message (WebSocket)**

```json
{
  "type": "msg",
  "data": {
    "room": "general",
    "text": "Hello, everyone!"
  }
}
```

Server broadcasts `message` event to all room members:
```json
{
  "type": "event",
  "event": "message",
  "room": "general",
  "user": "alice",
  "text": "Hello, everyone!",
  "id": 12345,
  "ts": 1701234567
}
```

---

**7. Retrieve Message History (REST)**

```bash
curl http://localhost:8080/api/rooms/1/messages?limit=50 \
  -H "Authorization: Bearer <token>"
```

Response:
```json
{
  "messages": [
    {
      "id": 12345,
      "room_id": 1,
      "user_id": 123,
      "user": "alice",
      "body": "Hello, everyone!",
      "created_at": "2025-12-02T12:05:00Z"
    }
  ],
  "has_more": false
}
```

---

### Direct Message Flow

**1. Create Direct Room (REST)**

User `alice` (ID: 123) creates DM with user `bob` (ID: 456):

```bash
curl -X POST http://localhost:8080/api/rooms/direct \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <alice_token>" \
  -d '{"user_id":456}'
```

Response:
```json
{"id":2,"name":"dm-123-456","type":"direct","created_at":"2025-12-02T13:00:00Z"}
```

---

**2. Join Direct Room (WebSocket)**

Both alice and bob send:

```json
{
  "type": "join",
  "data": {
    "room": "dm-123-456"
  }
}
```

---

**3. Send Private Message (WebSocket)**

```json
{
  "type": "msg",
  "data": {
    "room": "dm-123-456",
    "text": "Hi Bob, this is private!"
  }
}
```

Only alice and bob receive the message event.

---

### Error Handling Example

**Attempt to join private room without membership:**

Client sends:
```json
{
  "type": "join",
  "data": {
    "room": "secret-club"
  }
}
```

Server responds:
```json
{
  "type": "error",
  "error": {
    "code": "access_denied",
    "msg": "access denied"
  }
}
```

**SDK should**:
- Parse error code
- Call error handler: `onError("access_denied", "access denied")`
- NOT add room to local joined rooms list

---

## Appendix: Configuration Reference

Server configuration file (`config.yaml`):

```yaml
# Server
addr: ":8080"
read_header_timeout: 5s
shutdown_timeout: 5s

# Database
database_path: "data/wirechat.db"

# WebSocket
max_message_bytes: 1048576        # 1MB
ping_interval: 30s
client_idle_timeout: 90s

# Rate Limiting
rate_limit_join_per_min: 60
rate_limit_msg_per_min: 300

# Authentication
jwt_secret: "your-secret-key"
jwt_audience: "wirechat"
jwt_issuer: "wirechat-server"
jwt_required: false                # Set true to require JWT for all connections
```

**Environment Variables**: All config fields can be overridden via `WIRECHAT_*` env vars:
- `WIRECHAT_ADDR=:9000`
- `WIRECHAT_JWT_SECRET=my-secret`
- `WIRECHAT_JWT_REQUIRED=true`

---

## Version History

- **v1** (2025-12-02): Initial release
  - Core WebSocket protocol with join/leave/msg
  - JWT authentication + guest mode
  - Room types: public, private, direct
  - Message persistence and REST API history
  - Message history on WebSocket join

---

**End of Protocol Documentation**
