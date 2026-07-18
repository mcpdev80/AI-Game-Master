OpenAI-kompatibler TTS-Wrapper fuer Piper.

Endpoints:
- `GET /health`
- `POST /v1/audio/speech`

Erwartetes Payload:
```json
{
  "input": "Hallo Welt",
  "voice": "de_DE-thorsten-high",
  "response_format": "wav"
}
```
