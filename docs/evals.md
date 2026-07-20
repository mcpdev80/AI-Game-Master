# Evaluations

This document records the current evaluation coverage for the Build Week version as of July 20, 2026.

## Evaluation goals

We want evidence for four things:

1. the main gameplay loop works end to end
2. model outputs stay inside the allowed state-update boundary
3. player-visible failure modes are understandable
4. the demo still works when camera/audio features are unavailable

## Current automated coverage

### Backend validation and model-output handling

Covered in Go tests under `apps/api/internal/httpapi`:

- allowlisted state updates only
- reject unknown entity IDs
- reject campaign entity mutation
- reject empty entity IDs
- reject mismatched delta/value payloads
- validate roll request structure and follow-up chains
- treat untrusted retrieved content as untrusted prompt context
- classify OpenAI response failures:
  - rate limit
  - invalid schema
  - invalid JSON
  - completed response without usable output
- preserve and compact LLM session history safely

Current status:

- backend HTTP/API validation tests are green

Reference command:

```bash
docker run --rm \
  -v "$PWD/apps/api:/workspace" \
  -w /workspace \
  golang:1.25.8-bookworm \
  /usr/local/go/bin/go test ./internal/httpapi/...
```

### Deterministic golden path

Covered by:

- `scripts/golden_path_api_test.sh`
- `scripts/test_golden_path.sh`
- Playwright browser flow in `apps/web/e2e`

What it verifies:

- demo seeding
- builder conversation turn
- character completion
- session start
- initial narration
- roll request
- roll confirmation
- resulting state updates
- player portal visibility rules

Reference command:

```bash
npm run test:golden-path
```

### Negative browser-path coverage

Covered by Playwright:

- camera denied -> manual roll fallback works
- `/api/gm/respond` rate limited -> visible player-facing error is shown
- visible loading state while the player turn is in flight
- generic `5xx` player-facing error with retry action
- visible fallback narration notice when the backend uses the fallback path

Reference commands:

```bash
PLAYWRIGHT_BASE_URL=http://localhost:13005 npm run test:e2e -- e2e/dice-fallback.spec.ts
PLAYWRIGHT_BASE_URL=http://localhost:13005 npm run test:e2e -- e2e/rate-limit.spec.ts
```

## Manual evaluation checklist

These are still important because they involve real browser/device behavior:

- operator desktop over internal HTTPS
- player phone over internal HTTPS
- microphone permission denied
- camera permission denied
- autoplay/audio blocked
- text-only play still usable
- language switch between English and German

## Results summary

### Passing now

- backend validation and OpenAI error classification tests
- deterministic API golden path
- deterministic browser golden path
- player-facing rate-limit error path
- manual dice fallback when camera permission is denied
- player-facing loading, generic error, retry, and fallback-notice states

### Still open / not fully captured yet

- responsive operator-desktop vs. player-smartphone screenshots/assertions
- repeated real-device HTTPS run on desktop plus phone three times without restart

## Known limits of the current eval set

- the deterministic golden path uses a local OpenAI-compatible mock, not the live API
- current browser negative-path tests focus on the player screen, not every operator UI state
- performance and latency are observed operationally, not yet tracked as formal metrics

## Suggested next eval additions

1. a Playwright test for visible loading and retry guidance
2. a Playwright test for timeout-specific copy
3. a responsive screenshot/smoke pass for desktop operator and smartphone player layouts
4. a short real-device HTTPS checklist with recorded pass/fail results

## Evidence map

Code and test references:

- `apps/api/internal/httpapi/validation_test.go`
- `apps/api/internal/httpapi/llm_openai_test.go`
- `apps/api/internal/httpapi/llm_sessions_test.go`
- `apps/web/e2e/dice-fallback.spec.ts`
- `apps/web/e2e/player-feedback.spec.ts`
- `apps/web/e2e/rate-limit.spec.ts`
- `scripts/golden_path_api_test.sh`
- `scripts/test_golden_path.sh`
- `scripts/mvp_smoke_test.sh`
