# Security

This repository is a local/LAN-hosted demo for AI-assisted tabletop sessions. It is not intended to run as an unauthenticated public internet service in its current form.

## Scope

The current Build Week target is:

- local development on one machine
- internal LAN demo hosting
- browser-based judging on operator desktop plus player phone

Out of scope:

- multi-tenant public SaaS hosting
- long-term storage of user-generated campaign content
- untrusted anonymous uploads from the public internet

## Supported deployment model

Supported:

- local Docker Compose stack
- internal HTTPS via self-signed certificate as documented in [docs/LOCAL_HTTPS.md](docs/LOCAL_HTTPS.md)

Not supported:

- direct public exposure of the operator interface
- public exposure of internal databases or Redis
- production use with shared credentials

## Security controls currently implemented

### Access boundaries

- Player access uses revocable random join tokens.
- Operator-only configuration routes are restricted or disabled for demo use.
- CORS uses an allowlist instead of `*`.
- Trusted proxy handling is explicit.

### API and model safety

- Request and upload sizes are bounded.
- ZIP uploads are validated against path traversal and unsafe archive contents.
- Rate limits exist for GPT, upload, STT, and vision endpoints.
- Structured model outputs are validated before state updates are applied.
- Untrusted retrieved content is wrapped and separated from system instructions.

### Secrets and privacy

- The OpenAI API key is read only by the Go API.
- System endpoints do not return the API key.
- Logs are intended to avoid raw secrets, full player tokens, and full sensitive prompt content.
- Demo content is seeded and can be reset; long-term retention is not a product goal.

## Residual risk and demo limits

This project still has meaningful residual risk if exposed publicly without extra work:

- no hardened public identity layer for operators
- no managed certificate/public edge setup
- no WAF or internet-facing abuse controls beyond local demo rate limits
- no formal security review or penetration test
- self-signed HTTPS is acceptable only for local device testing

For Build Week judging, the expected safe usage is an internal controlled demo environment.

## Responsible disclosure

Please do not post secrets or exploit details publicly.

Report issues privately to the repository owner and include:

- affected component
- reproduction steps
- expected impact
- whether credentials or user data were involved

If a secret may have been exposed:

1. rotate the key immediately
2. replace `.env` values
3. restart the affected services
4. review recent logs and local demo data

## Hardening checklist before any public deployment

- add real operator authentication
- move HTTPS to a trusted public certificate and reverse proxy
- restrict access by origin and network boundary
- add central secret management and key rotation
- add audit logging and log redaction verification
- add automated dependency and secret scanning in CI
- add external abuse protection and public uptime monitoring

## Build Week note

This file documents the current July 20, 2026 demo posture, not a claim of production readiness.
