// Package ports defines the interfaces (hexagon edges) implemented by
// driven adapters (sqlite, agentcore) and consumed by core services.
package ports

import (
	"context"

	"harmonia/internal/core/domain"
)

type UserRepository interface {
	Create(ctx context.Context, u *domain.User) error // domain.ErrEmailTaken on duplicate
	ByEmail(ctx context.Context, email string) (*domain.User, error)
	ByID(ctx context.Context, id string) (*domain.User, error)
}

type SessionRepository interface {
	Create(ctx context.Context, s *domain.Session) error
	ByToken(ctx context.Context, token string) (*domain.Session, error) // expired = ErrNotFound
	Delete(ctx context.Context, token string) error
}

// UsageRepository tracks per-user daily consumption. day is "YYYY-MM-DD".
type UsageRepository interface {
	Get(ctx context.Context, userID, day string) (domain.Usage, error)
	IncrementMessages(ctx context.Context, userID, day string) error
	IncrementAds(ctx context.Context, userID, day string) error
}

type EventRepository interface {
	Record(ctx context.Context, userID string, event domain.EventType) error
	CountByType(ctx context.Context) (map[domain.EventType]int, error)
}

// MessageRepository persists the chat history per user.
type MessageRepository interface {
	SaveMessage(ctx context.Context, m *domain.Message) error
	// RecentMessages returns the last n messages in chronological order.
	RecentMessages(ctx context.Context, userID string, n int) ([]domain.Message, error)
}

// AgentInvoker is the outbound port to the AI mediator.
type AgentInvoker interface {
	Invoke(ctx context.Context, text, userID, agentSessionID string) (string, error)
}
