// Package bdd runs the Godog acceptance suite against a running Harmonia
// instance (BDD_BASE_URL). The full stack — nginx → app → WireMock playing
// AWS (SSM + AgentCore) — is provided by docker-compose; use bdd/run.sh.
package bdd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cucumber/godog"
)

// agentReplyMarker must match the canned reply in wiremock/mappings/invoke-agent.json.
const agentReplyMarker = "resposta simulada pelo WireMock"

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// world holds per-scenario state: one fresh user, one cookie jar.
type world struct {
	base     string
	wiremock string // admin API do WireMock (BDD_WIREMOCK_URL), p/ inspecionar o journal
	client   *http.Client

	email    string
	password string

	lastStatus   int
	lastBody     string
	lastLocation string
}

func newWorld(base, wiremock string) *world {
	jar, _ := cookiejar.New(nil)
	return &world{
		base:     base,
		wiremock: wiremock,
		password: "senha-bdd-123",
		client: &http.Client{
			Jar:     jar,
			Timeout: 130 * time.Second,
			// Redirects are asserted explicitly (signup/login return 303).
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (w *world) record(resp *http.Response) error {
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	w.lastStatus = resp.StatusCode
	w.lastBody = string(b)
	w.lastLocation = resp.Header.Get("Location")
	return nil
}

func (w *world) get(path string) error {
	resp, err := w.client.Get(w.base + path)
	if err != nil {
		return err
	}
	return w.record(resp)
}

func (w *world) postForm(path string, data url.Values) error {
	resp, err := w.client.PostForm(w.base+path, data)
	if err != nil {
		return err
	}
	return w.record(resp)
}

// ---- steps ----

func (w *world) crieiContaNova() error {
	w.email = fmt.Sprintf("bdd-%d@harmonia.test", time.Now().UnixNano())
	err := w.postForm("/signup", url.Values{
		"name":     {"Casal BDD"},
		"email":    {w.email},
		"password": {w.password},
	})
	if err != nil {
		return err
	}
	if w.lastStatus != http.StatusSeeOther {
		return fmt.Errorf("signup esperava 303, recebeu %d: %s", w.lastStatus, w.lastBody)
	}
	return nil
}

func (w *world) abroOChat() error { return w.get("/") }

func (w *world) vejoPilula(texto string) error {
	if err := w.get("/"); err != nil {
		return err
	}
	if !strings.Contains(w.lastBody, texto) {
		return fmt.Errorf("pílula %q não encontrada na página do chat", texto)
	}
	return nil
}

func (w *world) envioMensagem(texto string) error {
	return w.postForm("/chat", url.Values{"message": {texto}})
}

func (w *world) jaEnvieiMensagens(n int) error {
	for i := 1; i <= n; i++ {
		if err := w.envioMensagem(fmt.Sprintf("mensagem %d", i)); err != nil {
			return err
		}
		if !strings.Contains(w.lastBody, agentReplyMarker) {
			return fmt.Errorf("mensagem %d/%d não foi respondida pelo agente: %s", i, n, w.lastBody)
		}
	}
	return nil
}

func (w *world) assistoAnuncio() error {
	if err := w.postForm("/ads/start", nil); err != nil {
		return err
	}
	if !strings.Contains(w.lastBody, "Resgatar") {
		return fmt.Errorf("anúncio não iniciou (sem botão Resgatar): %s", w.lastBody)
	}
	return w.postForm("/ads/complete", nil)
}

func (w *world) jaAssistiAnuncios(n int) error {
	for i := 1; i <= n; i++ {
		if err := w.assistoAnuncio(); err != nil {
			return err
		}
		if !strings.Contains(w.lastBody, "Obrigado por assistir") {
			return fmt.Errorf("anúncio %d/%d não completou: %s", i, n, w.lastBody)
		}
	}
	return nil
}

func (w *world) tentoIniciarOutroAnuncio() error {
	return w.postForm("/ads/start", nil)
}

func (w *world) consigoEnviarMensagem(texto string) error {
	if err := w.envioMensagem(texto); err != nil {
		return err
	}
	if !strings.Contains(w.lastBody, agentReplyMarker) {
		return fmt.Errorf("mensagem deveria passar, mas não foi respondida: %s", w.lastBody)
	}
	return nil
}

func (w *world) clicoAssinarPremium() error {
	return w.postForm("/paywall/premium", nil)
}

func (w *world) tentoEntrarComSenhaErrada() error {
	return w.postForm("/login", url.Values{"email": {w.email}, "password": {"senha-errada"}})
}

func (w *world) entroComoAdmin() error {
	err := w.postForm("/login", url.Values{
		"email":    {envOr("BDD_ADMIN_USER", "harmonia")},
		"password": {envOr("BDD_ADMIN_PASS", "Conexao2025!")},
	})
	if err != nil {
		return err
	}
	if w.lastStatus != http.StatusSeeOther || w.lastLocation != "/" {
		return fmt.Errorf("login admin falhou: status=%d location=%q", w.lastStatus, w.lastLocation)
	}
	return nil
}

func (w *world) acessoPagina(path string) error { return w.get(path) }

func (w *world) souRedirecionadoPara(loc string) error {
	if w.lastStatus < 300 || w.lastStatus > 399 || w.lastLocation != loc {
		return fmt.Errorf("esperava redirect para %q, recebeu status=%d location=%q", loc, w.lastStatus, w.lastLocation)
	}
	return nil
}

func (w *world) statusDaResposta(code int) error {
	if w.lastStatus != code {
		return fmt.Errorf("esperava status %d, recebeu %d: %s", code, w.lastStatus, w.lastBody)
	}
	return nil
}

func (w *world) respostaContem(texto string) error {
	if !strings.Contains(w.lastBody, texto) {
		return fmt.Errorf("resposta não contém %q:\n%s", texto, w.lastBody)
	}
	return nil
}

func (w *world) respostaNaoContem(texto string) error {
	if strings.Contains(w.lastBody, texto) {
		return fmt.Errorf("resposta não deveria conter %q:\n%s", texto, w.lastBody)
	}
	return nil
}

// ---- memória / sessão de runtime ----

func (w *world) recarregoAPagina() error { return w.get("/") }

// historicoWireMockLimpo zera o journal de requests do WireMock para que o
// cenário possa contar apenas as próprias chamadas ao agente.
func (w *world) historicoWireMockLimpo() error {
	if w.wiremock == "" {
		return fmt.Errorf("BDD_WIREMOCK_URL não definida — rode via app/bdd/run.sh")
	}
	req, err := http.NewRequest(http.MethodDelete, w.wiremock+"/__admin/requests", nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("falha ao limpar journal do WireMock: status %d", resp.StatusCode)
	}
	return nil
}

const runtimeSessionHeader = "X-Amzn-Bedrock-Agentcore-Runtime-Session-Id"

// agenteRecebeuChamadasMesmaSessao lê o journal do WireMock e verifica que
// todas as chamadas de InvokeAgentRuntime carregam o MESMO RuntimeSessionId
// (header com ≥33 chars) — é isso que dá afinidade de sessão no AgentCore.
func (w *world) agenteRecebeuChamadasMesmaSessao(n int) error {
	if w.wiremock == "" {
		return fmt.Errorf("BDD_WIREMOCK_URL não definida — rode via app/bdd/run.sh")
	}
	resp, err := http.Get(w.wiremock + "/__admin/requests")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var journal struct {
		Requests []struct {
			Request struct {
				URL     string                     `json:"url"`
				Headers map[string]json.RawMessage `json:"headers"`
			} `json:"request"`
		} `json:"requests"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&journal); err != nil {
		return fmt.Errorf("journal do WireMock ilegível: %w", err)
	}

	var sessions []string
	for _, r := range journal.Requests {
		if !strings.Contains(r.Request.URL, "/invocations") {
			continue
		}
		val := ""
		for name, raw := range r.Request.Headers {
			if strings.EqualFold(name, runtimeSessionHeader) {
				// o valor pode vir como string ou lista de strings
				var s string
				if json.Unmarshal(raw, &s) == nil {
					val = s
				} else {
					var list []string
					if json.Unmarshal(raw, &list) == nil && len(list) > 0 {
						val = list[0]
					}
				}
			}
		}
		if val == "" {
			return fmt.Errorf("chamada ao agente sem o header %s (sem afinidade de sessão)", runtimeSessionHeader)
		}
		if len(val) < 33 {
			return fmt.Errorf("RuntimeSessionId %q tem %d chars; AgentCore exige ≥33", val, len(val))
		}
		sessions = append(sessions, val)
	}

	if len(sessions) != n {
		return fmt.Errorf("esperava %d chamadas ao agente, journal registrou %d", n, len(sessions))
	}
	for _, s := range sessions[1:] {
		if s != sessions[0] {
			return fmt.Errorf("RuntimeSessionId variou entre mensagens: %q vs %q — agente perde a memória", sessions[0], s)
		}
	}
	return nil
}

// ---- suite ----

func initializeScenario(base, wiremock string) func(*godog.ScenarioContext) {
	return func(sc *godog.ScenarioContext) {
		w := newWorld(base, wiremock)
		sc.Step(`^que eu criei uma conta nova$`, w.crieiContaNova)
		sc.Step(`^eu abro o chat$`, w.abroOChat)
		sc.Step(`^eu recarrego a página$`, w.recarregoAPagina)
		sc.Step(`^que o histórico de chamadas do WireMock está limpo$`, w.historicoWireMockLimpo)
		sc.Step(`^o agente recebeu (\d+) chamadas na mesma sessão de runtime$`, w.agenteRecebeuChamadasMesmaSessao)
		sc.Step(`^eu vejo a pílula de quota "([^"]*)"$`, w.vejoPilula)
		sc.Step(`^eu envio a mensagem "([^"]*)"$`, w.envioMensagem)
		sc.Step(`^eu já enviei (\d+) mensagens$`, w.jaEnvieiMensagens)
		sc.Step(`^eu assisto um anúncio até o final$`, w.assistoAnuncio)
		sc.Step(`^eu já assisti (\d+) anúncios$`, w.jaAssistiAnuncios)
		sc.Step(`^eu tento iniciar outro anúncio$`, w.tentoIniciarOutroAnuncio)
		sc.Step(`^eu consigo enviar a mensagem "([^"]*)"$`, w.consigoEnviarMensagem)
		sc.Step(`^eu clico em assinar Premium$`, w.clicoAssinarPremium)
		sc.Step(`^eu tento entrar com a senha errada$`, w.tentoEntrarComSenhaErrada)
		sc.Step(`^eu entro como admin$`, w.entroComoAdmin)
		sc.Step(`^eu acesso a página "([^"]*)"$`, w.acessoPagina)
		sc.Step(`^eu sou redirecionado para "([^"]*)"$`, w.souRedirecionadoPara)
		sc.Step(`^o status da resposta é (\d+)$`, w.statusDaResposta)
		sc.Step(`^a resposta contém "([^"]*)"$`, w.respostaContem)
		sc.Step(`^a resposta não contém "([^"]*)"$`, w.respostaNaoContem)
	}
}

func TestFeatures(t *testing.T) {
	base := os.Getenv("BDD_BASE_URL")
	if base == "" {
		t.Skip("BDD_BASE_URL não definida — rode app/bdd/run.sh para subir o stack e executar a suíte")
	}

	suite := godog.TestSuite{
		Name:                "harmonia-freemium",
		ScenarioInitializer: initializeScenario(strings.TrimRight(base, "/"), strings.TrimRight(os.Getenv("BDD_WIREMOCK_URL"), "/")),
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features"},
			Strict:   true,
			TestingT: t,
		},
	}
	if suite.Run() != 0 {
		t.Fatal("suíte BDD falhou")
	}
}
