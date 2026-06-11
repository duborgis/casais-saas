// Package agentcore is the driven adapter for the Bedrock AgentCore
// orchestrator (ports.AgentInvoker).
package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentcore"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	"harmonia/internal/core/service"
)

type Invoker struct {
	agentClient     *bedrockagentcore.Client
	ssmClient       *ssm.Client
	orchestratorARN string
	crewConfigPath  string

	crewOnce   sync.Once
	crewConfig json.RawMessage
}

func New(cfg aws.Config, orchestratorARN, crewConfigPath string) *Invoker {
	return &Invoker{
		agentClient:     bedrockagentcore.NewFromConfig(cfg),
		ssmClient:       ssm.NewFromConfig(cfg),
		orchestratorARN: orchestratorARN,
		crewConfigPath:  crewConfigPath,
	}
}

// getCrewConfig loads the crew config from SSM once; changes only on deploy.
func (a *Invoker) getCrewConfig(ctx context.Context) json.RawMessage {
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

func (a *Invoker) Invoke(ctx context.Context, text, agentSessionID string) (string, error) {
	runID := service.NewID()

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

// Mock replaces the real orchestrator in local dev / usability tests
// (MOCK_AGENT=1): canned reply, no AWS calls, no token cost.
type Mock struct{}

func (Mock) Invoke(_ context.Context, text, _ string) (string, error) {
	return "(mock) Recebi sua mensagem: “" + text + "”. Em produção, quem responde é a Valentina, nossa orquestradora.", nil
}
