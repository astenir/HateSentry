# AI Package Status

## Current State

`internal/ai` currently provides the provider-facing implementation used by both
the legacy detection routes and the text moderation MVP.

Active Go files seen by the compiler:

```bash
go list -f '{{.GoFiles}}' ./internal/ai
```

Current output:

```text
[detection_service.go moderation_prompt.go moderation_response.go ollama_provider.go openai_provider.go prompt.go types.go]
```

## Active Provider Paths

- `openai_provider.go`: OpenAI-backed detection and text moderation.
- `ollama_provider.go`: Ollama-backed detection and text moderation.
- `detection_service.go`: selects `AI_PROVIDER=openai` or `AI_PROVIDER=ollama`
  and exposes `AnalyzeText` for the moderation service.

Both OpenAI and Ollama providers are expected to return normalized text
moderation suggestions containing:

- `risk_score`
- `labels`
- `reason`
- raw provider output for internal audit storage

The service-owned final decision remains in `internal/moderation` policy code,
not in the provider implementation.

## MVP Boundary

The current MVP path is synchronous text moderation through:

```text
POST /api/v1/moderation/check
```

The legacy detection APIs still exist for compatibility, but image, batch, and
full async queue behavior are not the current verified MVP path.

## Verification

Useful checks:

```bash
go test ./internal/ai ./internal/moderation ./internal/router
go build ./...
```

For runtime smoke testing, ensure the configured provider is reachable:

- OpenAI requires a real `OPENAI_API_KEY`.
- Ollama requires a reachable `OLLAMA_BASE_URL` and the configured model pulled
  locally or by the Compose `ollama` profile.
