package service

import (
	"context"

	"harmonia/internal/core/domain"
	"harmonia/internal/core/ports"
)

// Chat combines entitlements and the agent: a message only reaches the
// orchestrator (and only consumes quota) if the user is authorized.
type Chat struct {
	agent ports.AgentInvoker
	ent   *Entitlements
}

func NewChat(agent ports.AgentInvoker, ent *Entitlements) *Chat {
	return &Chat{agent: agent, ent: ent}
}

// Send returns the agent reply and the post-send usage status.
// Denial returns domain.ErrQuotaExceeded with the current status.
// Quota is only consumed after a successful agent reply, so transient
// agent failures don't burn free messages.
func (c *Chat) Send(ctx context.Context, u *domain.User, agentSessionID, text string) (string, domain.UsageStatus, error) {
	st, err := c.ent.Authorize(ctx, u)
	if err != nil {
		return "", st, err
	}
	reply, err := c.agent.Invoke(ctx, text, agentSessionID)
	if err != nil {
		return "", st, err
	}
	st, err = c.ent.RecordMessage(ctx, u)
	return reply, st, err
}
