// Package web is the driving HTTP adapter: routes, handlers and the
// HTMX templates. All business decisions live in core/service.
package web

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"html/template"
	"log"
	"net/http"
	"time"

	"harmonia/internal/core/domain"
	"harmonia/internal/core/service"
)

//go:embed templates/*.html
var templateFS embed.FS

var templates = template.Must(template.ParseFS(templateFS, "templates/*.html"))

const sessionCookie = "harmonia_session"

// Metrics is implemented by the sqlite store; optional operator endpoint.
type Metrics interface {
	CountByType(ctx context.Context) (map[domain.EventType]int, error)
	CountUsers(ctx context.Context) (int, error)
}

type Server struct {
	auth       *service.Auth
	ent        *service.Entitlements
	chat       *service.Chat
	metrics    Metrics
	adminEmail string
}

func New(auth *service.Auth, ent *service.Entitlements, chat *service.Chat, metrics Metrics, adminEmail string) *Server {
	return &Server{auth: auth, ent: ent, chat: chat, metrics: metrics, adminEmail: adminEmail}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.handleIndex)
	mux.HandleFunc("GET /signup", s.handleSignupPage)
	mux.HandleFunc("POST /signup", s.handleSignup)
	mux.HandleFunc("POST /login", s.handleLogin)
	mux.HandleFunc("POST /logout", s.handleLogout)
	mux.HandleFunc("POST /chat", s.handleChat)
	mux.HandleFunc("POST /ads/start", s.handleAdStart)
	mux.HandleFunc("POST /ads/complete", s.handleAdComplete)
	mux.HandleFunc("POST /paywall/premium", s.handlePremiumClick)
	mux.HandleFunc("GET /metrics", s.handleMetrics)
	mux.HandleFunc("GET /health", s.handleHealth)
	return mux
}

// ---- session helpers ----

func (s *Server) currentUser(r *http.Request) (*domain.User, *domain.Session) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return nil, nil
	}
	u, sess, err := s.auth.UserBySessionToken(r.Context(), c.Value)
	if err != nil {
		return nil, nil
	}
	return u, sess
}

func (s *Server) startSession(w http.ResponseWriter, r *http.Request, userID string) error {
	sess, err := s.auth.CreateSession(r.Context(), userID)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    sess.Token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   60 * 60 * 24 * 7,
	})
	return nil
}

// pill feeds the quota_pill.html partial; OOB makes it an htmx
// out-of-band update of the header counter.
type pill struct {
	Status domain.UsageStatus
	OOB    bool
}

func render(w http.ResponseWriter, name string, data any) {
	if err := templates.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("[!] template %s: %v", name, err)
	}
}

// ---- pages ----

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	u, _ := s.currentUser(r)
	if u == nil {
		render(w, "login.html", map[string]any{"Error": r.URL.Query().Get("error") != ""})
		return
	}
	st, err := s.ent.Status(r.Context(), u)
	if err != nil {
		http.Error(w, "erro interno", http.StatusInternalServerError)
		return
	}
	history, err := s.chat.History(r.Context(), u)
	if err != nil {
		log.Printf("[!] Erro ao carregar histórico (user=%s): %v", u.ID, err)
	}
	render(w, "chat.html", map[string]any{"User": u, "Pill": pill{Status: st}, "Messages": history})
}

func (s *Server) handleSignupPage(w http.ResponseWriter, r *http.Request) {
	render(w, "signup.html", nil)
}

func (s *Server) handleSignup(w http.ResponseWriter, r *http.Request) {
	u, err := s.auth.Signup(r.Context(), r.FormValue("name"), r.FormValue("email"), r.FormValue("password"))
	if err != nil {
		msg := "Não foi possível criar a conta. Tente novamente."
		if errors.Is(err, domain.ErrEmailTaken) {
			msg = "Este e-mail já está cadastrado. Faça login."
		} else if err.Error() != "" {
			msg = err.Error()
		}
		render(w, "signup.html", map[string]any{"Error": msg, "Name": r.FormValue("name"), "Email": r.FormValue("email")})
		return
	}
	if err := s.startSession(w, r, u.ID); err != nil {
		http.Error(w, "erro interno", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	u, err := s.auth.Login(r.Context(), r.FormValue("email"), r.FormValue("password"))
	if err != nil {
		http.Redirect(w, r, "/?error=1", http.StatusSeeOther)
		return
	}
	if err := s.startSession(w, r, u.ID); err != nil {
		http.Error(w, "erro interno", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		s.auth.Logout(r.Context(), c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// ---- chat + freemium fragments ----

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	u, sess := s.currentUser(r)
	if u == nil {
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

	reply, st, err := s.chat.Send(ctx, u, sess.AgentSessionID, text)
	switch {
	case errors.Is(err, domain.ErrQuotaExceeded):
		// notice bubble in the chat + out-of-band quota modal
		render(w, "message.html", map[string]any{
			"Text":  "Suas mensagens gratuitas de hoje acabaram. 💛",
			"Error": true,
		})
		render(w, "quota_modal.html", map[string]any{"Status": st})
		return
	case err != nil:
		log.Printf("[!] Erro ao invocar orquestrador: %v", err)
		render(w, "message.html", map[string]any{
			"Text":  "Falha na comunicação com o agente. Tente novamente.",
			"Error": true,
		})
		return
	}

	render(w, "message.html", map[string]any{"Text": reply})
	render(w, "quota_pill.html", pill{Status: st, OOB: true})
}

func (s *Server) handleAdStart(w http.ResponseWriter, r *http.Request) {
	u, _ := s.currentUser(r)
	if u == nil {
		w.Header().Set("HX-Redirect", "/")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	st, err := s.ent.StartAd(r.Context(), u)
	if err != nil {
		render(w, "quota_modal.html", map[string]any{"Status": st})
		return
	}
	render(w, "ad_modal.html", map[string]any{"Bonus": s.ent.Quota().AdBonusMessages})
}

func (s *Server) handleAdComplete(w http.ResponseWriter, r *http.Request) {
	u, _ := s.currentUser(r)
	if u == nil {
		w.Header().Set("HX-Redirect", "/")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	st, err := s.ent.CompleteAd(r.Context(), u)
	if err != nil {
		render(w, "quota_modal.html", map[string]any{"Status": st})
		return
	}
	render(w, "modal_close.html", map[string]any{"Bonus": s.ent.Quota().AdBonusMessages})
	render(w, "quota_pill.html", pill{Status: st, OOB: true})
}

func (s *Server) handlePremiumClick(w http.ResponseWriter, r *http.Request) {
	u, _ := s.currentUser(r)
	if u == nil {
		w.Header().Set("HX-Redirect", "/")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	s.ent.RecordPremiumClick(r.Context(), u)
	render(w, "premium_thanks.html", nil)
}

// ---- operator endpoints ----

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	u, _ := s.currentUser(r)
	if u == nil || u.Email != s.adminEmail {
		http.NotFound(w, r)
		return
	}
	events, err := s.metrics.CountByType(r.Context())
	if err != nil {
		http.Error(w, "erro interno", http.StatusInternalServerError)
		return
	}
	users, err := s.metrics.CountUsers(r.Context())
	if err != nil {
		http.Error(w, "erro interno", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"users": users, "events": events})
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok","service":"harmonia"}`))
}
