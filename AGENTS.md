# HateSentry Development Guide

## Project Direction

HateSentry should be developed as a practical content moderation gateway, not as a broad "enterprise-grade multimodal platform" demo.

Target product shape:

- Accept content moderation requests from small applications such as blogs, forums, comment systems, support tools, or AI apps.
- Use AI providers to classify text risk.
- Convert model output into business decisions: `allow`, `review`, or `block`.
- Store moderation records for auditability and later review.
- Support a human review queue for uncertain cases.
- Provide API key access and webhook callbacks so external systems can integrate it.

The near-term product should prioritize a reliable text moderation workflow over advanced image, batch, high-concurrency, or multi-tenant claims.

## Current Baseline

The repository currently contains a Go backend service with:

- Gin HTTP routing.
- JWT registration and login.
- GORM models for users, detection requests, and detection results.
- Redis-backed rate limiting and result cache helpers.
- RabbitMQ wrapper and consumer scaffolding.
- OpenAI and Ollama provider implementations.
- Prometheus HTTP metrics endpoint.
- Docker Compose definitions for MySQL, Redis, and RabbitMQ.

Known limitations:

- No `_test.go` files are present.
- The current local environment may not have the `go` command available.
- Async detection is not correctly wired because the detection handler publishes a model request where the RabbitMQ publisher expects a `queue.Task`.
- Batch detection code exists but is not registered in routes, and batch status/result endpoints are not implemented.
- Image and mixed-content handling mostly pass image URLs/descriptions into prompts; the main HTTP path does not perform real image download or provider image submission.
- Feedback handler code exists but is not registered in routes, and feedback models are not currently migrated.
- Docker Compose environment variables are broader than the config loader's override support, so container networking may not work as documented.
- README and docs may still contain feature claims ahead of actual implementation.

## Development Strategy

Prefer building a small, verifiable MVP over adding new surface area.

Default priorities:

1. Make the project build and run.
2. Make one text moderation API reliable.
3. Add policy-based decisions.
4. Add audit records and human review.
5. Add external integration features.
6. Only then revisit batch, async queueing, image moderation, and advanced observability.

Avoid:

- Expanding README claims before implementation and tests exist.
- Adding more providers before the core workflow is stable.
- Treating RabbitMQ, multimodal image analysis, smart caching, or high concurrency as MVP requirements.
- Introducing broad refactors before the build and current runtime behavior are understood.

## Development Standards

Use these standards for all future code changes.

General rules:

- Read this file before starting non-trivial work.
- Start every task with `git status --short`.
- Preserve existing user changes. Do not reset, restore, or overwrite unrelated modified files.
- Keep each change focused on one behavior, one phase, or one defect.
- Prefer clear, boring code over new abstractions.
- Add an abstraction only when it removes real duplication or clarifies an existing boundary.
- Do not introduce new infrastructure dependencies for MVP work unless the phase explicitly needs them.
- Keep old detection routes working only when compatibility is intentional; otherwise document deprecation clearly.

Go rules:

- Use idiomatic Go naming, small functions, and explicit error handling.
- Keep handlers thin. Put business logic in service/helper packages so it can be tested without HTTP.
- Use `context.Context` for request-scoped operations and external calls.
- Avoid package-level mutable state except for existing infrastructure clients that already use it.
- Do not panic for expected runtime errors.
- Return structured errors through the existing error handling layer where practical.
- Run `gofmt` on changed Go files.

API rules:

- New public moderation APIs should use `moderation` terminology instead of `detection`.
- Public moderation decisions are limited to `allow`, `review`, and `block`.
- Validate request payloads at the HTTP boundary.
- Keep response JSON stable and documented.
- Store enough metadata for auditability: source, external ID, actor ID, provider/model, policy version, labels, risk score, decision, and reason.
- Do not expose raw model output by default unless the endpoint is clearly operator/internal.

Configuration rules:

- Environment variables used by Docker Compose must be supported by the config loader.
- Configuration defaults should be safe for local development, but secrets must not be production-looking.
- Do not commit real API keys, webhook secrets, tokens, or private endpoints.
- Prefer explicit config fields over hidden behavior.

Database rules:

- Model changes must be reflected in migration/AutoMigrate behavior intentionally.
- Keep persisted state compatible with the moderation workflow: AI suggestion, policy decision, and human final decision should not be conflated.
- Add indexes for fields that are used for lookup, such as request ID, client ID, external ID, status, and created time.

## Proposed Product Model

Rename the public product concept from "detection" toward "moderation" in new APIs and docs.

Core request:

```json
{
  "content": "user submitted text",
  "source": "comment",
  "external_id": "comment_123",
  "actor_id": "user_456"
}
```

Core response:

```json
{
  "request_id": "uuid",
  "decision": "review",
  "risk_score": 0.82,
  "labels": ["harassment", "identity_attack"],
  "reason": "Brief explanation suitable for operators",
  "policy_version": "default-v1"
}
```

Decision semantics:

- `allow`: content can be published automatically.
- `review`: content needs human review before final action.
- `block`: content should be rejected or hidden automatically.

Recommended labels for the first version:

- `hate`
- `harassment`
- `identity_attack`
- `threat`
- `sexual`
- `violence`
- `spam`
- `self_harm`
- `illegal`
- `safe`

The AI provider may suggest labels and risk, but the service should own the final decision through server-side policy rules.

## Phase Plan

### Phase 0: Stabilize The Existing Repository

Goal: establish a trustworthy baseline before product changes.

Tasks:

- Ensure Go is installed and available in the development environment.
- Run `go test ./...` or `go build ./...` and record current failures.
- Fix compile errors before changing behavior.
- Verify Docker Compose service wiring.
- Update config loading so container-provided `DB_*`, `REDIS_*`, `RABBITMQ_*`, `JWT_SECRET`, and `OPENAI_API_KEY` values actually override YAML config.
- Keep README wording conservative until behavior is verified.

Done when:

- `go test ./...` passes, or at minimum `go build ./...` passes with known test gaps documented.
- `docker compose up` starts MySQL, Redis, RabbitMQ, and the API service successfully.
- `GET /api/v1/health` returns healthy with all dependencies reachable.

### Phase 1: Build The Text Moderation MVP

Goal: replace the current broad detection surface with one reliable text moderation workflow.

Tasks:

- Add `POST /api/v1/moderation/check`.
- Keep the first version text-only.
- Add models such as `ModerationRequest`, `ModerationResult`, and optionally `ModerationPolicy`.
- Store `content`, `source`, `external_id`, `actor_id`, `provider`, `model`, `raw_output`, `risk_score`, `labels`, `decision`, and `reason`.
- Add a strict parser for provider JSON output.
- Generate final `decision` server-side from policy thresholds instead of trusting the model's final action.
- Keep the old `/api/v1/detection/*` routes only if backward compatibility is desired; otherwise mark them deprecated in docs.

Done when:

- A client can submit text and receive `allow`, `review`, or `block`.
- Results are persisted and queryable.
- Unit tests cover provider response parsing, policy decision logic, and route registration.

### Phase 2: Add Policy Configuration

Goal: make moderation behavior adjustable without changing prompts.

Tasks:

- Define a default policy with score thresholds.
- Support per-label thresholds where useful.
- Store a `policy_version` on each moderation result.
- Add internal helpers for calculating final decisions.
- Keep policy data simple at first: YAML or database table, not a complex rules engine.

Suggested initial thresholds:

- `risk_score < 0.4`: `allow`
- `0.4 <= risk_score < 0.75`: `review`
- `risk_score >= 0.75`: `block`

Done when:

- Policy tests prove threshold boundaries.
- Moderation results include the policy version used.
- The README documents the policy model accurately.

### Phase 3: Add Human Review Workflow

Goal: turn the API into an operational moderation tool.

Tasks:

- Add a `ReviewCase` model.
- Automatically create a review case when decision is `review`.
- Add review endpoints:
  - `GET /api/v1/reviews?status=pending`
  - `POST /api/v1/reviews/:id/approve`
  - `POST /api/v1/reviews/:id/reject`
  - `POST /api/v1/reviews/:id/mark-mistake`
- Store reviewer ID, final decision, review notes, and timestamps.
- Add minimal stats: total moderated, allowed, blocked, pending review, reviewed, and mistake rate.

Done when:

- Uncertain content appears in the review queue.
- A reviewer can approve or reject it.
- The final decision is stored separately from the AI suggestion.

### Phase 4: Add Integration Features

Goal: make the service easy to connect to real applications.

Tasks:

- Add API key authentication for external clients.
- Keep JWT for admin/operator workflows if needed.
- Add client/application records with API key, status, optional webhook URL, and policy assignment.
- Add webhook callbacks for final decisions.
- Sign webhook callbacks with an HMAC secret.
- Add idempotency using `external_id` plus client identity.
- Add request-level rate limiting per client.

Done when:

- An external app can call the API using an API key.
- The service can call back the external app after a final decision.
- Repeated requests with the same external ID do not create duplicate active cases.

### Phase 5: Optional Admin UI

Goal: make the project feel like a real moderation product instead of only an API demo.

Suggested screens:

- Review queue.
- Moderation history.
- Policy settings.
- Client/API key settings.
- Basic metrics dashboard.

Keep the UI small and operator-focused. Avoid marketing-style pages.

Done when:

- A reviewer can process pending cases without using curl.
- Operators can inspect history and understand why content was blocked or reviewed.

### Phase 6: Revisit Advanced Capabilities

Only after Phases 0-4 are working:

- Restore async processing with a correct `queue.Task` publication path.
- Reintroduce batch moderation with persisted batch status and result retrieval.
- Add real image moderation by downloading and validating images, enforcing size/type limits, and using provider image APIs.
- Improve queue reliability with dead-letter queues and retry policies.
- Add richer Prometheus metrics for moderation outcomes and review latency.

## Testing Expectations

Add tests before or alongside behavior changes.

Minimum useful coverage:

- Config environment overrides.
- JWT or API key authentication.
- Route registration.
- Provider JSON parsing.
- Policy decision logic.
- Moderation record persistence.
- Review case creation and finalization.
- Webhook signing and retry behavior when implemented.

Default verification commands:

```bash
go test ./...
go build ./...
```

Also run `gofmt` on changed Go files before committing:

```bash
gofmt -w <changed-go-files>
```

If Docker is used:

```bash
docker compose up -d mysql redis rabbitmq
go test ./...
docker compose up --build
```

If a full verification command cannot run because of missing credentials, external services, network access, or local environment limits, state the exact reason and run the closest meaningful subset.

## Documentation Rules

Keep documentation tied to implemented behavior.

When a feature is not fully wired and tested:

- Mark it as planned.
- Move it to a roadmap section.
- Do not list it as a current feature.

README should emphasize:

- Text moderation.
- Business decisions: `allow`, `review`, `block`.
- Audit records.
- Human review workflow once implemented.
- API integration.

README should avoid claiming:

- Enterprise-grade reliability.
- Production-ready high concurrency.
- Full multimodal image understanding.
- Batch processing.
- Smart cache preheating.
- Complete async queue workflow.

## Git Workflow

Commit work in small, reviewable slices.

Before editing:

- Run `git status --short`.
- Identify existing modified/untracked files and avoid mixing unrelated changes into your commit.
- If existing changes are unrelated, leave them alone.
- If existing changes affect the same files, inspect them and work with them instead of reverting them.

Before committing:

- Run `git diff --check`.
- Run `gofmt` on changed Go files.
- Run `go test ./...`.
- Run `go build ./...`.
- Update docs when public behavior changes.
- Review `git diff --stat` and `git diff` to ensure the commit is scoped.

Commit message format:

```text
<type>: <short Chinese summary>
```

Allowed types:

- `feat`: user-visible feature or new workflow capability.
- `fix`: bug fix or broken behavior repair.
- `test`: tests only or test infrastructure.
- `docs`: documentation only.
- `refactor`: behavior-preserving code restructuring.
- `chore`: tooling, repository hygiene, or non-product maintenance.

Examples:

```text
fix: 修复 Docker 环境变量配置覆盖
feat: 新增文本审核决策接口
test: 补充策略阈值边界测试
docs: 同步内容审核网关开发计划
refactor: 拆分审核策略计算逻辑
chore: 整理本地开发验证说明
```

Commit grouping:

- Keep changes scoped to one phase or one feature.
- Separate behavior changes from broad documentation rewrites when practical.
- Separate tests from large feature commits if that makes review easier, but do not leave behavior untested.
- Do not include generated caches, local toolchains, downloaded modules, build binaries, logs, or private `.env` files.
- If a phase needs multiple commits, commit each stable slice after verification.

Commit timing:

- Commit after each phase or each independently verified slice.
- Do not wait until several unrelated phases accumulate.
- If review finds defects, fix them in a follow-up commit instead of amending history unless the user explicitly asks for history cleanup.

## Review Workflow

Use review for stage-level changes and risky patches.

When to call another Codex review:

- After completing each AGENTS phase.
- Before merging or pushing larger feature work.
- After touching auth, API key handling, webhook signing, policy decisions, database persistence, or Docker/runtime config.
- When a change removes or deprecates old behavior.

Recommended review prompt:

```text
Please review the current HateSentry diff or the latest commit as a strict code reviewer.
Focus on functional bugs, missing tests, API/documentation mismatch, auth/security issues, config/deployment risks, and over-broad changes.
List findings first, ordered by severity, with file and line references.
Avoid generic style advice unless it affects correctness or maintainability.
```

Review handling:

- Fix clear bugs, security issues, broken tests, and docs/implementation mismatches.
- Add tests for important review findings.
- If a review suggestion is only stylistic or would expand scope, defer it and mention why.
- After review fixes, rerun verification and commit a follow-up fix.

## Reporting Standards

For each completed task, report:

- What changed.
- Which files matter most.
- Which verification commands ran and whether they passed.
- Any commands that could not run and the exact reason.
- Any remaining risks or follow-up work.

## Practical Next Step

The next concrete development task should be Phase 0:

1. Install or expose Go in the environment.
2. Run `go test ./...`.
3. Fix build failures.
4. Fix Docker Compose config overrides.
5. Add the first tests around config loading and route setup.

After that, start Phase 1 with a new `POST /api/v1/moderation/check` endpoint and a text-only moderation result model.
