# LiveKit Dev Infrastructure

Local LiveKit server for WireChat development and testing.

## What is LiveKit?

LiveKit is an open-source WebRTC SFU (Selective Forwarding Unit) that handles real-time video/audio communication. WireChat uses LiveKit as its media server for voice and video calls.

> **Note**: This is a dev-only setup. Do not use in production without proper configuration.

## Quick Start

1. **Copy environment file**:
   ```bash
   cp .env.example .env
   ```

2. **Start LiveKit**:
   ```bash
   make up
   # Or from wirechat-server root:
   make livekit-up
   ```

3. **Check logs**:
   ```bash
   make logs
   ```

## Smoke Test

Verify LiveKit is working by running a test with video:

1. **Start the token server** (in a separate terminal):
   ```bash
   # Set credentials (same as in .env)
   export LIVEKIT_API_KEY=devkey
   export LIVEKIT_API_SECRET=devsecret_at_least_32_characters_long

   make token-server
   ```

2. **Open the test client**:
   - Open `testclient/web/index.html` in your browser
   - Or serve it: `python3 -m http.server 8000 -d testclient/web`

3. **Test the connection**:
   - Enter a room name (e.g., "test-room")
   - Enter an identity (e.g., "user1")
   - Click "Join Room"
   - Allow camera/microphone access
   - You should see your video feed

4. **Test multi-user** (optional):
   - Open another browser tab/window
   - Join the same room with a different identity
   - You should see both video feeds

## Stopping LiveKit

```bash
make down
# Or from wirechat-server root:
make livekit-down
```

To remove all data and volumes:
```bash
make clean
```

## Configuration

### Environment Variables (.env)

| Variable | Default | Description |
|----------|---------|-------------|
| `LIVEKIT_API_KEY` | `devkey` | API key for authentication |
| `LIVEKIT_API_SECRET` | (32+ chars) | API secret for JWT signing |
| `LIVEKIT_PORT` | `7880` | HTTP/WebSocket port |
| `LIVEKIT_WS_URL` | `ws://localhost:7880` | WebSocket URL for clients |

### Ports

| Port | Protocol | Description |
|------|----------|-------------|
| 7880 | TCP | HTTP API and WebSocket signaling |
| 7881 | TCP | RTC over TCP (fallback) |
| 7882-7892 | UDP | RTC media (WebRTC) |

## Troubleshooting

### "Connection refused" when joining room

- Check if LiveKit is running: `docker ps | grep livekit`
- Check logs: `make logs`

### Token server errors

- Ensure `LIVEKIT_API_KEY` and `LIVEKIT_API_SECRET` env vars are set
- Secret must be at least 32 characters

### No video/audio

- Check browser permissions for camera/microphone
- Try a different browser (Chrome recommended)
- Check browser console for errors

### Ports already in use

- Change `LIVEKIT_PORT` in `.env`
- Or stop conflicting services

## Limitations

This is a development setup:

- Single-node only (no clustering)
- No persistence (rooms are lost on restart)
- No TLS (use HTTPS proxy for production)
- No TURN server configured (may not work behind strict NAT)
