package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentcore"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

//go:embed templates/*.html
var templateFS embed.FS

var templates = template.Must(template.ParseFS(templateFS, "templates/*.html"))

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// ---- Sessions (in-memory, MVP single instance) ----

type session struct {
	agentSessionID string
	createdAt      time.Time
}

type sessionStore struct {
	mu       sync.Mutex
	sessions map[string]*session
}

func (s *sessionStore) create() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	token := newID() + newID()
	s.sessions[token] = &session{agentSessionID: newID(), createdAt: time.Now()}
	return token
}

func (s *sessionStore) get(token string) *session {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessions[token]
}

func (s *sessionStore) delete(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, token)
}

// ---- AgentCore client ----

type app struct {
	agentClient     *bedrockagentcore.Client
	ssmClient       *ssm.Client
	orchestratorARN string
	crewConfigPath  string
	mvpUser         string
	mvpPass         string
	store           *sessionStore

	crewOnce   sync.Once
	crewConfig json.RawMessage
}

// getCrewConfig loads the crew config from SSM once; changes only on deploy.
func (a *app) getCrewConfig(ctx context.Context) json.RawMessage {
	a.crewOnce.Do(func() {
		out, err := a.ssmClient.GetParameter(ctx, &ssm.GetParameterInput{
			Name: aws.String(a.crewConfigPath),
		})
		if err != nil {
			log.Printf("[!] Could not load crew config from SSM: %v", err)
			return
		}
		a.crewConfig = json.RawMessage(*out.Parameter.Value)
		log.Printf("[i] Crew config loaded from SSM (%d bytes)", len(a.crewConfig))
	})
	return a.crewConfig
}

func (a *app) invokeOrchestrator(ctx context.Context, text, agentSessionID string) (string, error) {
	runID := newID()

	input := map[string]any{
		"text":    text,
		"userId":  "user-1",
		"agentId": "orchestrator",
		"runtimeContext": map[string]any{
			"runId":           runID,
			"conversationId":  agentSessionID,
			"delegationDepth": 0,
		},
	}
	if crew := a.getCrewConfig(ctx); crew != nil {
		input["agentConfig"] = map[string]any{"id": "orchestrator", "runtime_arn": a.orchestratorARN}
		input["crewConfig"] = crew
	}

	payload, err := json.Marshal(map[string]any{
		"sessionId": agentSessionID,
		"input":     input,
	})
	if err != nil {
		return "", err
	}

	log.Printf("[>] Invocando orquestrador | session=%s", agentSessionID)

	out, err := a.agentClient.InvokeAgentRuntime(ctx, &bedrockagentcore.InvokeAgentRuntimeInput{
		AgentRuntimeArn: aws.String(a.orchestratorARN),
		Qualifier:       aws.String("DEFAULT"),
		ContentType:     aws.String("application/json"),
		Accept:          aws.String("application/json"),
		Payload:         payload,
	})
	if err != nil {
		return "", err
	}
	defer out.Response.Close()

	body, err := io.ReadAll(out.Response)
	if err != nil {
		return "", err
	}

	var result struct {
		Output struct {
			Text string `json:"text"`
		} `json:"output"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("resposta inválida do agente: %w", err)
	}

	log.Printf("[<] Resposta recebida | session=%s chars=%d", agentSessionID, len(result.Output.Text))
	return result.Output.Text, nil
}

// ---- HTTP handlers ----

const sessionCookie = "harmonia_session"

func (a *app) currentSession(r *http.Request) *session {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return nil
	}
	return a.store.get(c.Value)
}

func (a *app) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if a.currentSession(r) == nil {
		templates.ExecuteTemplate(w, "login.html", map[string]any{"Error": r.URL.Query().Get("error") != ""})
		return
	}
	templates.ExecuteTemplate(w, "chat.html", nil)
}

func (a *app) handleLogin(w http.ResponseWriter, r *http.Request) {
	user := r.FormValue("username")
	pass := r.FormValue("password")
	userOK := subtle.ConstantTimeCompare([]byte(user), []byte(a.mvpUser)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(pass), []byte(a.mvpPass)) == 1
	if !userOK || !passOK {
		http.Redirect(w, r, "/?error=1", http.StatusSeeOther)
		return
	}
	token := a.store.create()
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   60 * 60 * 24 * 7,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *app) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		a.store.delete(c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *app) handleChat(w http.ResponseWriter, r *http.Request) {
	sess := a.currentSession(r)
	if sess == nil {
		w.Header().Set("HX-Redirect", "/")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	text := r.FormValue("message")
	if text == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	reply, err := a.invokeOrchestrator(ctx, text, sess.agentSessionID)
	if err != nil {
		log.Printf("[!] Erro ao invocar orquestrador: %v", err)
		templates.ExecuteTemplate(w, "message.html", map[string]any{
			"Text":  "Falha na comunicação com o agente. Tente novamente.",
			"Error": true,
		})
		return
	}

	templates.ExecuteTemplate(w, "message.html", map[string]any{"Text": reply})
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok","service":"harmonia"}`))
}

func main() {
	ctx := context.Background()

	region := envOr("AWS_REGION", "us-east-1")
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		log.Fatalf("[!] Erro ao carregar config AWS: %v", err)
	}

	a := &app{
		agentClient:     bedrockagentcore.NewFromConfig(cfg),
		ssmClient:       ssm.NewFromConfig(cfg),
		orchestratorARN: os.Getenv("ORCHESTRATOR_ARN"),
		crewConfigPath:  envOr("CREW_CONFIG_SSM_PATH", "/casais-saas/agents/crew-config"),
		mvpUser:         envOr("HARMONIA_USER", "harmonia"),
		mvpPass:         envOr("HARMONIA_PASS", "Conexao2025!"),
		store:           &sessionStore{sessions: map[string]*session{}},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", a.handleIndex)
	mux.HandleFunc("POST /login", a.handleLogin)
	mux.HandleFunc("POST /logout", a.handleLogout)
	mux.HandleFunc("POST /chat", a.handleChat)
	mux.HandleFunc("GET /health", handleHealth)

	port := envOr("PORT", "8080")
	log.Printf("[OK] Harmonia rodando na porta %s", port)
	log.Printf("[i] Orquestrador ARN: %s", envOr("ORCHESTRATOR_ARN", "(não configurado)"))
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
