# casais-saas

SaaS application for couples mediation and management (Harmonia).

## Agent Behavior Mandate
Before implementing any significant change, you MUST investigate and ask clarifying questions to ensure full alignment with the user's needs. Do not proceed based on intuition or assumptions alone; validate the direction first.

## Architecture
- **`app/`**: Single Go binary, hexagonal architecture:
  - `internal/core/domain` — entities + freemium rules (Plan, Quota, UsageStatus, events)
  - `internal/core/ports` — interfaces (UserRepository, UsageRepository, EventRepository, AgentInvoker, ...)
  - `internal/core/service` — use cases (Auth, Entitlements, Chat) — unit-tested with fakes
  - `internal/adapters/sqlite` — persistence (modernc.org/sqlite, pure Go, DB at `DB_PATH`, default `/data/harmonia.db`)
  - `internal/adapters/agentcore` — Bedrock AgentCore invoker (+ `Mock` when `MOCK_AGENT=1`)
  - `internal/adapters/web` — HTTP handlers + HTMX templates (embedded)
- **`agents/`**: Python multi-agent system (orchestrator + 5 specialists) deployed as Bedrock AgentCore runtimes.
- **Deploy**: One Docker image (`ghcr.io/<owner>/casais-saas/app`) on a single EC2 behind nginx-proxy + acme-companion. `harmonia.duborges.com`. SQLite persisted in the `appdata` Docker volume.
- **Auth**: real users (signup/login, bcrypt) in SQLite; sessions in SQLite (7d). `HARMONIA_USER`/`HARMONIA_PASS` seed the premium admin account (defaults: harmonia / Conexao2025!), which also unlocks `GET /metrics`.
- **Freemium (mocked ads for usability testing)**: free = `FREE_DAILY_MESSAGES`/day (5); mocked rewarded ad grants `+AD_BONUS_MESSAGES` (3), max `MAX_ADS_PER_DAY` (3); premium unlimited. Funnel events (paywall_shown, ad_started, ad_completed, premium_click) recorded in SQLite, visible at `/metrics`. Daily reset at midnight `QUOTA_TZ` (America/Sao_Paulo).

## Build Commands
- **App**: `cd app && go build .`
- **Docker**: `cd app && docker build -t harmonia .`
- **Run dev**: `cd app && go run .` (port 8080; needs `ORCHESTRATOR_ARN` + AWS creds for chat)

## Test Commands
- **App (unit)**: `cd app && go test ./internal/...` (core services tested with in-memory fakes)
- **BDD (Godog)**: `app/bdd/run.sh` — sobe docker compose (WireMock como AWS via `AWS_ENDPOINT_URL` + nginx reverse proxy + app) e roda os cenários Gherkin de `app/bdd/features/` contra o stack real. Roda no CI antes do build/deploy. `go test ./bdd` sozinho é skip sem `BDD_BASE_URL`.
- **Local run without AWS**: `cd app && MOCK_AGENT=1 DB_PATH=/tmp/h.db go run .`

## Lint/Format Commands
- **App**: `cd app && gofmt -l . && go vet ./...`

## Style & Architecture
- **App**: Go stdlib only for HTTP (net/http with method-prefixed routes), AWS SDK Go v2 (`bedrockagentcore`, `ssm`). Templates embedded via `go:embed` in `app/templates/`.
- **UI**: HTMX 2 + Tailwind (both via CDN). POST /chat returns an HTML fragment (`message.html`).
- **Env vars**: `PORT`, `AWS_REGION`, `ORCHESTRATOR_ARN`, `CREW_CONFIG_SSM_PATH`, `HARMONIA_USER`, `HARMONIA_PASS`, `DB_PATH`, `MOCK_AGENT`, `FREE_DAILY_MESSAGES`, `AD_BONUS_MESSAGES`, `MAX_ADS_PER_DAY`, `QUOTA_TZ`.

## Project Context
- **Mediation**: See `MEDIATION_GUIDE.md` for logic details.
- **Infra**: Terraform lives in the separate `aws-terraform` repo (`ec2-casais/` module).
