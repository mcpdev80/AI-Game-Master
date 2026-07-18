# API

Go backend for the AI Game Master MVP.

## Run in a container

```bash
docker compose up api --build
```

## Initial endpoints

- `GET /api/health`
- `GET /api/system/summary`
- `GET /api/documents`
- `POST /api/documents`
- `GET /api/campaigns`
- `POST /api/campaigns`
- `GET /api/sessions`
- `GET /api/sessions/:id`
- `POST /api/sessions`
- `GET /api/voice-profiles`
