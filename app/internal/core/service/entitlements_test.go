package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"harmonia/internal/core/domain"
)

// ---- in-memory fakes (no SQLite, no AWS) ----

type fakeUsage struct {
	data map[string]domain.Usage // key: userID|day
}

func (f *fakeUsage) key(userID, day string) string { return userID + "|" + day }

func (f *fakeUsage) Get(_ context.Context, userID, day string) (domain.Usage, error) {
	return f.data[f.key(userID, day)], nil
}

func (f *fakeUsage) IncrementMessages(_ context.Context, userID, day string) error {
	u := f.data[f.key(userID, day)]
	u.Messages++
	f.data[f.key(userID, day)] = u
	return nil
}

func (f *fakeUsage) IncrementAds(_ context.Context, userID, day string) error {
	u := f.data[f.key(userID, day)]
	u.AdsWatched++
	f.data[f.key(userID, day)] = u
	return nil
}

type fakeEvents struct {
	counts map[domain.EventType]int
}

func (f *fakeEvents) Record(_ context.Context, _ string, e domain.EventType) error {
	f.counts[e]++
	return nil
}

func (f *fakeEvents) CountByType(_ context.Context) (map[domain.EventType]int, error) {
	return f.counts, nil
}

func newTestEntitlements() (*Entitlements, *fakeEvents) {
	events := &fakeEvents{counts: map[domain.EventType]int{}}
	ent := NewEntitlements(
		&fakeUsage{data: map[string]domain.Usage{}},
		events,
		domain.Quota{FreeDailyMessages: 5, AdBonusMessages: 3, MaxAdsPerDay: 3},
		time.UTC,
	)
	return ent, events
}

func freeUser() *domain.User    { return &domain.User{ID: "u1", Plan: domain.PlanFree} }
func premiumUser() *domain.User { return &domain.User{ID: "p1", Plan: domain.PlanPremium} }

// ---- tests ----

func TestFreeUserHitsDailyQuota(t *testing.T) {
	ent, events := newTestEntitlements()
	ctx := context.Background()
	u := freeUser()

	for i := 0; i < 5; i++ {
		if _, err := ent.Authorize(ctx, u); err != nil {
			t.Fatalf("message %d should be allowed: %v", i+1, err)
		}
		if _, err := ent.RecordMessage(ctx, u); err != nil {
			t.Fatal(err)
		}
	}

	st, err := ent.Authorize(ctx, u)
	if !errors.Is(err, domain.ErrQuotaExceeded) {
		t.Fatalf("6th message should be denied, got err=%v", err)
	}
	if st.Remaining() != 0 || st.Used != 5 || st.Limit != 5 {
		t.Fatalf("unexpected status: %+v", st)
	}
	if events.counts[domain.EventPaywallShown] != 1 {
		t.Fatalf("paywall_shown should be recorded once, got %d", events.counts[domain.EventPaywallShown])
	}
}

func TestAdGrantsBonusMessages(t *testing.T) {
	ent, events := newTestEntitlements()
	ctx := context.Background()
	u := freeUser()

	for i := 0; i < 5; i++ {
		ent.RecordMessage(ctx, u)
	}
	if _, err := ent.Authorize(ctx, u); !errors.Is(err, domain.ErrQuotaExceeded) {
		t.Fatal("quota should be exhausted before the ad")
	}

	if _, err := ent.StartAd(ctx, u); err != nil {
		t.Fatalf("ad should be available: %v", err)
	}
	st, err := ent.CompleteAd(ctx, u)
	if err != nil {
		t.Fatal(err)
	}
	if st.Limit != 8 || st.Remaining() != 3 {
		t.Fatalf("ad should grant +3 (limit 8, remaining 3), got %+v", st)
	}
	if _, err := ent.Authorize(ctx, u); err != nil {
		t.Fatalf("should be allowed after the ad: %v", err)
	}
	if events.counts[domain.EventAdCompleted] != 1 {
		t.Fatal("ad_completed not recorded")
	}
}

func TestAdDailyCapIsEnforced(t *testing.T) {
	ent, _ := newTestEntitlements()
	ctx := context.Background()
	u := freeUser()

	for i := 0; i < 3; i++ {
		if _, err := ent.CompleteAd(ctx, u); err != nil {
			t.Fatalf("ad %d should be allowed: %v", i+1, err)
		}
	}
	if _, err := ent.CompleteAd(ctx, u); !errors.Is(err, domain.ErrAdLimitReached) {
		t.Fatalf("4th ad should be denied, got %v", err)
	}
	if _, err := ent.StartAd(ctx, u); !errors.Is(err, domain.ErrAdLimitReached) {
		t.Fatalf("StartAd should also deny, got %v", err)
	}
}

func TestPremiumIsUnlimitedAndCannotWatchAds(t *testing.T) {
	ent, _ := newTestEntitlements()
	ctx := context.Background()
	u := premiumUser()

	for i := 0; i < 50; i++ {
		if _, err := ent.Authorize(ctx, u); err != nil {
			t.Fatalf("premium should never be denied: %v", err)
		}
		ent.RecordMessage(ctx, u)
	}
	st, _ := ent.Status(ctx, u)
	if !st.Unlimited || st.Remaining() != -1 {
		t.Fatalf("premium should be unlimited: %+v", st)
	}
	if st.CanWatchAd() {
		t.Fatal("premium should not see rewarded ads")
	}
}

func TestQuotaResetsNextDay(t *testing.T) {
	ent, _ := newTestEntitlements()
	ctx := context.Background()
	u := freeUser()

	day := time.Date(2026, 6, 11, 23, 0, 0, 0, time.UTC)
	ent.now = func() time.Time { return day }
	for i := 0; i < 5; i++ {
		ent.RecordMessage(ctx, u)
	}
	if _, err := ent.Authorize(ctx, u); !errors.Is(err, domain.ErrQuotaExceeded) {
		t.Fatal("should be exhausted today")
	}

	ent.now = func() time.Time { return day.Add(2 * time.Hour) } // next day
	st, err := ent.Authorize(ctx, u)
	if err != nil {
		t.Fatalf("quota should reset on a new day: %v", err)
	}
	if st.Used != 0 {
		t.Fatalf("fresh day should have 0 used, got %d", st.Used)
	}
}
