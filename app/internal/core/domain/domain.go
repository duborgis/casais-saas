// Package domain holds the core business entities and rules of Harmonia.
// It has no dependencies on adapters (HTTP, SQLite, AWS).
package domain

import (
	"errors"
	"time"
)

var (
	ErrNotFound           = errors.New("não encontrado")
	ErrEmailTaken         = errors.New("e-mail já cadastrado")
	ErrInvalidCredentials = errors.New("credenciais inválidas")
	ErrQuotaExceeded      = errors.New("limite diário de mensagens atingido")
	ErrAdLimitReached     = errors.New("limite diário de anúncios atingido")
)

type Plan string

const (
	PlanFree    Plan = "free"
	PlanPremium Plan = "premium"
)

type User struct {
	ID           string
	Email        string
	Name         string
	PasswordHash string
	Plan         Plan
	CreatedAt    time.Time
}

type Session struct {
	Token          string
	UserID         string
	AgentSessionID string
	CreatedAt      time.Time
	ExpiresAt      time.Time
}

// Usage is the consumption of a single user in a single day.
type Usage struct {
	Messages   int
	AdsWatched int
}

// Quota defines the freemium rules. Premium users bypass it entirely.
type Quota struct {
	FreeDailyMessages int // base allowance per day
	AdBonusMessages   int // extra messages granted per watched ad
	MaxAdsPerDay      int
}

// Limit is the total number of messages a free user may send today,
// given how many ads they already watched.
func (q Quota) Limit(adsWatched int) int {
	return q.FreeDailyMessages + adsWatched*q.AdBonusMessages
}

// UsageStatus is what the UI needs to render the quota state.
type UsageStatus struct {
	Plan       Plan
	Used       int
	Limit      int // meaningless when Unlimited
	Unlimited  bool
	AdsWatched int
	MaxAds     int
}

func (s UsageStatus) Remaining() int {
	if s.Unlimited {
		return -1
	}
	if r := s.Limit - s.Used; r > 0 {
		return r
	}
	return 0
}

func (s UsageStatus) CanSend() bool   { return s.Unlimited || s.Used < s.Limit }
func (s UsageStatus) CanWatchAd() bool { return !s.Unlimited && s.AdsWatched < s.MaxAds }

// EventType labels funnel events used to validate the freemium model.
type EventType string

const (
	EventSignup       EventType = "signup"
	EventLogin        EventType = "login"
	EventMessageSent  EventType = "message_sent"
	EventPaywallShown EventType = "paywall_shown"
	EventAdStarted    EventType = "ad_started"
	EventAdCompleted  EventType = "ad_completed"
	EventPremiumClick EventType = "premium_click"
)
