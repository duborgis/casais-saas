// Package service implements the core use cases of Harmonia, depending
// only on domain types and ports.
package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"harmonia/internal/core/domain"
	"harmonia/internal/core/ports"
)

const sessionTTL = 7 * 24 * time.Hour

func NewID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

type Auth struct {
	users    ports.UserRepository
	sessions ports.SessionRepository
	events   ports.EventRepository
}

func NewAuth(users ports.UserRepository, sessions ports.SessionRepository, events ports.EventRepository) *Auth {
	return &Auth{users: users, sessions: sessions, events: events}
}

func (a *Auth) Signup(ctx context.Context, name, email, password string) (*domain.User, error) {
	name = strings.TrimSpace(name)
	email = strings.ToLower(strings.TrimSpace(email))
	if name == "" || email == "" || !strings.Contains(email, "@") {
		return nil, errors.New("preencha nome e um e-mail válido")
	}
	if len(password) < 6 {
		return nil, errors.New("a senha precisa de pelo menos 6 caracteres")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	u := &domain.User{
		ID:           NewID(),
		Email:        email,
		Name:         name,
		PasswordHash: string(hash),
		Plan:         domain.PlanFree,
		CreatedAt:    time.Now(),
	}
	if err := a.users.Create(ctx, u); err != nil {
		return nil, err
	}
	a.events.Record(ctx, u.ID, domain.EventSignup)
	return u, nil
}

func (a *Auth) Login(ctx context.Context, email, password string) (*domain.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	u, err := a.users.ByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrInvalidCredentials
		}
		return nil, err
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return nil, domain.ErrInvalidCredentials
	}
	a.events.Record(ctx, u.ID, domain.EventLogin)
	return u, nil
}

func (a *Auth) CreateSession(ctx context.Context, userID string) (*domain.Session, error) {
	s := &domain.Session{
		Token:          NewID() + NewID(),
		UserID:         userID,
		AgentSessionID: NewID(),
		CreatedAt:      time.Now(),
		ExpiresAt:      time.Now().Add(sessionTTL),
	}
	if err := a.sessions.Create(ctx, s); err != nil {
		return nil, err
	}
	return s, nil
}

// UserBySessionToken resolves a cookie token to a live user, or ErrNotFound.
func (a *Auth) UserBySessionToken(ctx context.Context, token string) (*domain.User, *domain.Session, error) {
	s, err := a.sessions.ByToken(ctx, token)
	if err != nil {
		return nil, nil, err
	}
	u, err := a.users.ByID(ctx, s.UserID)
	if err != nil {
		return nil, nil, err
	}
	return u, s, nil
}

func (a *Auth) Logout(ctx context.Context, token string) {
	a.sessions.Delete(ctx, token)
}

// SeedAdmin ensures a premium admin account exists (MVP operator login).
func (a *Auth) SeedAdmin(ctx context.Context, email, password string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if _, err := a.users.ByEmail(ctx, email); err == nil {
		return nil
	} else if !errors.Is(err, domain.ErrNotFound) {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return a.users.Create(ctx, &domain.User{
		ID:           NewID(),
		Email:        email,
		Name:         "Admin",
		PasswordHash: string(hash),
		Plan:         domain.PlanPremium,
		CreatedAt:    time.Now(),
	})
}
