package service

import (
	"context"
	"log"
	"time"

	"harmonia/internal/core/domain"
	"harmonia/internal/core/ports"
)

// historyLimit caps how many past messages are rendered on page load.
const historyLimit = 50

// Chat combines entitlements and the agent: a message only reaches the
// orchestrator (and only consumes quota) if the user is authorized.
type Chat struct {
	agent ports.AgentInvoker
	ent   *Entitlements
	msgs  ports.MessageRepository
}

func NewChat(agent ports.AgentInvoker, ent *Entitlements, msgs ports.MessageRepository) *Chat {
	return &Chat{agent: agent, ent: ent, msgs: msgs}
}

// Send returns the agent reply and the post-send usage status.
// Denial returns domain.ErrQuotaExceeded with the current status.
// Quota is only consumed after a successful agent reply, so transient
// agent failures don't burn free messages. The exchange is persisted with
// the same rule: failed sends leave no trace in the history.
func (c *Chat) Send(ctx context.Context, u *domain.User, agentSessionID, text string) (string, domain.UsageStatus, error) {
	st, err := c.ent.Authorize(ctx, u)
	if err != nil {
		return "", st, err
	}
	reply, err := c.agent.Invoke(ctx, text, u.ID, agentSessionID)
	if err != nil {
		return "", st, err
	}
	now := time.Now()
	for _, m := range []*domain.Message{
		{UserID: u.ID, Role: domain.RoleUser, Content: text, CreatedAt: now},
		{UserID: u.ID, Role: domain.RoleAgent, Content: reply, CreatedAt: now},
	} {
		if err := c.msgs.SaveMessage(ctx, m); err != nil {
			log.Printf("[!] Falha ao persistir mensagem (user=%s role=%s): %v", u.ID, m.Role, err)
		}
	}
	st, err = c.ent.RecordMessage(ctx, u)
	return reply, st, err
}

// History returns the recent conversation so the UI can restore it on reload.
func (c *Chat) History(ctx context.Context, u *domain.User) ([]domain.Message, error) {
	return c.msgs.RecentMessages(ctx, u.ID, historyLimit)
}
