// Harmonia — composition root. Wires driven adapters (sqlite, agentcore)
// into core services and exposes them through the web adapter.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
	_ "time/tzdata"

	"github.com/aws/aws-sdk-go-v2/config"

	"harmonia/internal/adapters/agentcore"
	"harmonia/internal/adapters/sqlite"
	"harmonia/internal/adapters/web"
	"harmonia/internal/core/domain"
	"harmonia/internal/core/ports"
	"harmonia/internal/core/service"
)

// envOr returns the value of an environment variable or a fallback string.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envInt returns the value of an environment variable as an integer or a fallback.
func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func main() {
	ctx := context.Background()

	// ---- persistence ----
	dbPath := envOr("DB_PATH", "/data/harmonia.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		log.Fatalf("[!] Não foi possível criar o diretório do banco (%s): %v — defina DB_PATH", dbPath, err)
	}
	store, err := sqlite.Open(dbPath)
	if err != nil {
		log.Fatalf("[!] Erro ao abrir SQLite em %s: %v", dbPath, err)
	}
	defer store.Close()

	// ---- core services ----
	auth := service.NewAuth(store, store.Sessions(), store)

	quota := domain.Quota{
		FreeDailyMessages: envInt("FREE_DAILY_MESSAGES", 5),
		AdBonusMessages:   envInt("AD_BONUS_MESSAGES", 3),
		MaxAdsPerDay:      envInt("MAX_ADS_PER_DAY", 3),
	}
	loc, err := time.LoadLocation(envOr("QUOTA_TZ", "America/Sao_Paulo"))
	if err != nil {
		log.Printf("[!] QUOTA_TZ inválida, usando UTC: %v", err)
		loc = time.UTC
	}
	ent := service.NewEntitlements(store, store, quota, loc)

	var invoker ports.AgentInvoker
	if os.Getenv("MOCK_AGENT") == "1" {
		log.Printf("[i] MOCK_AGENT=1 — respostas do agente são simuladas (sem AWS)")
		invoker = agentcore.Mock{}
	} else {
		cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(envOr("AWS_REGION", "us-east-1")))
		if err != nil {
			log.Fatalf("[!] Erro ao carregar config AWS: %v", err)
		}
		invoker = agentcore.New(cfg,
			os.Getenv("ORCHESTRATOR_ARN"),
			envOr("CREW_CONFIG_SSM_PATH", "/casais-saas/agents/crew-config"))
	}
	chat := service.NewChat(invoker, ent, store)

	// ---- operator account (premium, also unlocks /metrics) ----
	adminEmail := envOr("HARMONIA_USER", "harmonia")
	if err := auth.SeedAdmin(ctx, adminEmail, envOr("HARMONIA_PASS", "Conexao2025!")); err != nil {
		log.Fatalf("[!] Erro ao criar conta admin: %v", err)
	}

	srv := web.New(auth, ent, chat, store, adminEmail)

	port := envOr("PORT", "8080")
	log.Printf("[OK] Harmonia rodando na porta %s | quota free=%d/dia, ad=+%d (máx %d/dia)",
		port, quota.FreeDailyMessages, quota.AdBonusMessages, quota.MaxAdsPerDay)
	log.Printf("[i] Orquestrador ARN: %s", envOr("ORCHESTRATOR_ARN", "(não configurado)"))
	log.Fatal(http.ListenAndServe(":"+port, srv.Routes()))
}
