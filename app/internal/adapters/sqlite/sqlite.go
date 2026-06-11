// Package sqlite is the driven persistence adapter. One Store implements
// all repository ports (users, sessions, usage, events).
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"harmonia/internal/core/domain"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)")
	if err != nil {
		return nil, err
	}
	// modernc.org/sqlite is single-writer; avoid SQLITE_BUSY under concurrency
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS users (
  id            TEXT PRIMARY KEY,
  email         TEXT NOT NULL UNIQUE,
  name          TEXT NOT NULL DEFAULT '',
  password_hash TEXT NOT NULL,
  plan          TEXT NOT NULL DEFAULT 'free',
  created_at    TIMESTAMP NOT NULL
);
CREATE TABLE IF NOT EXISTS sessions (
  token            TEXT PRIMARY KEY,
  user_id          TEXT NOT NULL REFERENCES users(id),
  agent_session_id TEXT NOT NULL,
  created_at       TIMESTAMP NOT NULL,
  expires_at       TIMESTAMP NOT NULL
);
CREATE TABLE IF NOT EXISTS daily_usage (
  user_id     TEXT NOT NULL,
  day         TEXT NOT NULL,
  messages    INTEGER NOT NULL DEFAULT 0,
  ads_watched INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (user_id, day)
);
CREATE TABLE IF NOT EXISTS events (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id    TEXT NOT NULL,
  type       TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_events_type ON events(type);
`)
	return err
}

// ---- ports.UserRepository ----

func (s *Store) Create(ctx context.Context, u *domain.User) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO users (id, email, name, password_hash, plan, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		u.ID, u.Email, u.Name, u.PasswordHash, string(u.Plan), u.CreatedAt)
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed: users.email") {
		return domain.ErrEmailTaken
	}
	return err
}

func (s *Store) scanUser(row *sql.Row) (*domain.User, error) {
	var u domain.User
	var plan string
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &plan, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	u.Plan = domain.Plan(plan)
	return &u, nil
}

func (s *Store) ByEmail(ctx context.Context, email string) (*domain.User, error) {
	return s.scanUser(s.db.QueryRowContext(ctx,
		`SELECT id, email, name, password_hash, plan, created_at FROM users WHERE email = ?`, email))
}

func (s *Store) ByID(ctx context.Context, id string) (*domain.User, error) {
	return s.scanUser(s.db.QueryRowContext(ctx,
		`SELECT id, email, name, password_hash, plan, created_at FROM users WHERE id = ?`, id))
}

// ---- ports.SessionRepository ----

func (s *Store) CreateSession(ctx context.Context, sess *domain.Session) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (token, user_id, agent_session_id, created_at, expires_at) VALUES (?, ?, ?, ?, ?)`,
		sess.Token, sess.UserID, sess.AgentSessionID, sess.CreatedAt, sess.ExpiresAt)
	return err
}

func (s *Store) SessionByToken(ctx context.Context, token string) (*domain.Session, error) {
	var sess domain.Session
	err := s.db.QueryRowContext(ctx,
		`SELECT token, user_id, agent_session_id, created_at, expires_at FROM sessions WHERE token = ? AND expires_at > ?`,
		token, time.Now()).
		Scan(&sess.Token, &sess.UserID, &sess.AgentSessionID, &sess.CreatedAt, &sess.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &sess, nil
}

func (s *Store) DeleteSession(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token = ?`, token)
	return err
}

// ---- ports.UsageRepository ----

func (s *Store) Get(ctx context.Context, userID, day string) (domain.Usage, error) {
	var u domain.Usage
	err := s.db.QueryRowContext(ctx,
		`SELECT messages, ads_watched FROM daily_usage WHERE user_id = ? AND day = ?`, userID, day).
		Scan(&u.Messages, &u.AdsWatched)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Usage{}, nil
	}
	return u, err
}

func (s *Store) IncrementMessages(ctx context.Context, userID, day string) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO daily_usage (user_id, day, messages) VALUES (?, ?, 1)
ON CONFLICT (user_id, day) DO UPDATE SET messages = messages + 1`, userID, day)
	return err
}

func (s *Store) IncrementAds(ctx context.Context, userID, day string) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO daily_usage (user_id, day, ads_watched) VALUES (?, ?, 1)
ON CONFLICT (user_id, day) DO UPDATE SET ads_watched = ads_watched + 1`, userID, day)
	return err
}

// ---- ports.EventRepository ----

func (s *Store) Record(ctx context.Context, userID string, event domain.EventType) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO events (user_id, type, created_at) VALUES (?, ?, ?)`,
		userID, string(event), time.Now())
	return err
}

func (s *Store) CountByType(ctx context.Context) (map[domain.EventType]int, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT type, COUNT(*) FROM events GROUP BY type`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[domain.EventType]int{}
	for rows.Next() {
		var t string
		var n int
		if err := rows.Scan(&t, &n); err != nil {
			return nil, err
		}
		out[domain.EventType(t)] = n
	}
	return out, rows.Err()
}

// CountUsers supports the operator metrics endpoint.
func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}
