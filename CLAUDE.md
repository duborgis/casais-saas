# casais-saas

SaaS application for couples mediation and management (Harmonia).

## Agent Behavior Mandate
Before implementing any significant change, you MUST investigate and ask clarifying questions to ensure full alignment with the user's needs. Do not proceed based on intuition or assumptions alone; validate the direction first.

## Architecture
- **`app/`**: Single Go binary serving the web UI (html/template + HTMX via CDN) and invoking the Bedrock AgentCore orchestrator. No separate frontend/backend.
- **`agents/`**: Python multi-agent system (orchestrator + 5 specialists) deployed as Bedrock AgentCore runtimes.
- **Deploy**: One Docker image (`ghcr.io/<owner>/casais-saas/app`) on a single EC2 behind nginx-proxy + acme-companion. `harmonia.duborges.com`.
- **Auth**: MVP server-side session with credentials from `HARMONIA_USER`/`HARMONIA_PASS` env vars (defaults: harmonia / Conexao2025!).

## Build Commands
- **App**: `cd app && go build .`
- **Docker**: `cd app && docker build -t harmonia .`
- **Run dev**: `cd app && go run .` (port 8080; needs `ORCHESTRATOR_ARN` + AWS creds for chat)

## Test Commands
- **App**: `cd app && go test ./...` (none yet)

## Lint/Format Commands
- **App**: `cd app && gofmt -l . && go vet ./...`

## Style & Architecture
- **App**: Go stdlib only for HTTP (net/http with method-prefixed routes), AWS SDK Go v2 (`bedrockagentcore`, `ssm`). Templates embedded via `go:embed` in `app/templates/`.
- **UI**: HTMX 2 + Tailwind (both via CDN). POST /chat returns an HTML fragment (`message.html`).
- **Env vars**: `PORT`, `AWS_REGION`, `ORCHESTRATOR_ARN`, `CREW_CONFIG_SSM_PATH`, `HARMONIA_USER`, `HARMONIA_PASS`.

## Project Context
- **Mediation**: See `MEDIATION_GUIDE.md` for logic details.
- **Infra**: Terraform lives in the separate `aws-terraform` repo (`ec2-casais/` module).
