package sqlite

import (
	"context"

	"harmonia/internal/core/domain"
	"harmonia/internal/core/ports"
)

// Sessions adapts the Store to ports.SessionRepository (the method names
// clash with the user repository on the same struct).
func (s *Store) Sessions() ports.SessionRepository { return sessionRepo{s} }

type sessionRepo struct{ s *Store }

func (r sessionRepo) Create(ctx context.Context, sess *domain.Session) error {
	return r.s.CreateSession(ctx, sess)
}

func (r sessionRepo) ByToken(ctx context.Context, token string) (*domain.Session, error) {
	return r.s.SessionByToken(ctx, token)
}

func (r sessionRepo) Delete(ctx context.Context, token string) error {
	return r.s.DeleteSession(ctx, token)
}
