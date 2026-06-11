package service

import (
	"context"
	"time"

	"harmonia/internal/core/domain"
	"harmonia/internal/core/ports"
)

// Entitlements enforces the freemium rules: daily message quota for free
// users, extra credits via (mocked) rewarded ads, unlimited for premium.
type Entitlements struct {
	usage  ports.UsageRepository
	events ports.EventRepository
	quota  domain.Quota
	loc    *time.Location
	now    func() time.Time
}

func NewEntitlements(usage ports.UsageRepository, events ports.EventRepository, quota domain.Quota, loc *time.Location) *Entitlements {
	if loc == nil {
		loc = time.UTC
	}
	return &Entitlements{usage: usage, events: events, quota: quota, loc: loc, now: time.Now}
}

func (e *Entitlements) day() string {
	return e.now().In(e.loc).Format("2006-01-02")
}

func (e *Entitlements) Status(ctx context.Context, u *domain.User) (domain.UsageStatus, error) {
	usage, err := e.usage.Get(ctx, u.ID, e.day())
	if err != nil {
		return domain.UsageStatus{}, err
	}
	return domain.UsageStatus{
		Plan:       u.Plan,
		Used:       usage.Messages,
		Limit:      e.quota.Limit(usage.AdsWatched),
		Unlimited:  u.Plan == domain.PlanPremium,
		AdsWatched: usage.AdsWatched,
		MaxAds:     e.quota.MaxAdsPerDay,
	}, nil
}

// Authorize checks whether the user may send a message right now.
// On denial it records the paywall funnel event and returns ErrQuotaExceeded.
func (e *Entitlements) Authorize(ctx context.Context, u *domain.User) (domain.UsageStatus, error) {
	st, err := e.Status(ctx, u)
	if err != nil {
		return st, err
	}
	if !st.CanSend() {
		e.events.Record(ctx, u.ID, domain.EventPaywallShown)
		return st, domain.ErrQuotaExceeded
	}
	return st, nil
}

// RecordMessage counts one sent message and returns the fresh status.
func (e *Entitlements) RecordMessage(ctx context.Context, u *domain.User) (domain.UsageStatus, error) {
	if err := e.usage.IncrementMessages(ctx, u.ID, e.day()); err != nil {
		return domain.UsageStatus{}, err
	}
	e.events.Record(ctx, u.ID, domain.EventMessageSent)
	return e.Status(ctx, u)
}

// StartAd validates the user may watch a rewarded ad and records the funnel event.
func (e *Entitlements) StartAd(ctx context.Context, u *domain.User) (domain.UsageStatus, error) {
	st, err := e.Status(ctx, u)
	if err != nil {
		return st, err
	}
	if !st.CanWatchAd() {
		return st, domain.ErrAdLimitReached
	}
	e.events.Record(ctx, u.ID, domain.EventAdStarted)
	return st, nil
}

// CompleteAd grants the ad bonus (AdBonusMessages extra for today).
func (e *Entitlements) CompleteAd(ctx context.Context, u *domain.User) (domain.UsageStatus, error) {
	st, err := e.Status(ctx, u)
	if err != nil {
		return st, err
	}
	if !st.CanWatchAd() {
		return st, domain.ErrAdLimitReached
	}
	if err := e.usage.IncrementAds(ctx, u.ID, e.day()); err != nil {
		return st, err
	}
	e.events.Record(ctx, u.ID, domain.EventAdCompleted)
	return e.Status(ctx, u)
}

// RecordPremiumClick logs intent to subscribe — the key validation metric.
func (e *Entitlements) RecordPremiumClick(ctx context.Context, u *domain.User) {
	e.events.Record(ctx, u.ID, domain.EventPremiumClick)
}

func (e *Entitlements) Quota() domain.Quota { return e.quota }
