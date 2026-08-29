package main

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/store"
)

type fakeCallbackRedisCapabilities struct {
	pingErr error
}

func (f *fakeCallbackRedisCapabilities) Ping(context.Context) error { return f.pingErr }
func (f *fakeCallbackRedisCapabilities) EnqueueContactWelcome(context.Context, dashboard.ContactWelcomeEvent) error {
	return nil
}
func (f *fakeCallbackRedisCapabilities) WorkContactWelcomeStatus(context.Context, int) (int, error) {
	return 0, nil
}
func (f *fakeCallbackRedisCapabilities) SetWorkContactWelcomeStatus(context.Context, int, int, time.Duration) error {
	return nil
}
func (f *fakeCallbackRedisCapabilities) EnqueueMarkTags(context.Context, dashboard.MarkTagsEvent) error {
	return nil
}

func TestOptionalWeWorkCallbackCapabilitiesDegradeWithoutRedis(t *testing.T) {
	caps, available := optionalWeWorkCallbackCapabilities(context.Background(), &fakeCallbackRedisCapabilities{pingErr: errors.New("redis down")})
	if available || caps.ContactWelcomeQueue != nil || caps.ContactWelcomeCache != nil || caps.MarkTagsQueue != nil {
		t.Fatalf("capabilities=%+v available=%t", caps, available)
	}
}

func TestOptionalWeWorkCallbackCapabilitiesUseHealthyRedis(t *testing.T) {
	redis := &fakeCallbackRedisCapabilities{}
	caps, available := optionalWeWorkCallbackCapabilities(context.Background(), redis)
	if !available || caps.ContactWelcomeQueue == nil || caps.ContactWelcomeCache == nil || caps.MarkTagsQueue == nil {
		t.Fatalf("capabilities=%+v available=%t", caps, available)
	}
}

func TestWeWorkCallbackRedisCapabilityResolverRecoversAfterRedisReturns(t *testing.T) {
	const secret = "callback-secret-value"
	redis := &fakeCallbackRedisCapabilities{pingErr: errors.New("redis password=" + secret)}
	resolver := weWorkCallbackRedisCapabilityResolver{candidate: redis}

	if _, err := resolver.ResolveWeWorkCallbackCapabilities(context.Background()); !errors.Is(err, dashboard.ErrWeWorkCallbackDependencyUnavailable) || strings.Contains(err.Error(), secret) {
		t.Fatalf("redis-down error=%v", err)
	}

	redis.pingErr = nil
	caps, err := resolver.ResolveWeWorkCallbackCapabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if caps.ContactWelcomeQueue != redis || caps.ContactWelcomeCache != redis || caps.MarkTagsQueue != redis {
		t.Fatalf("recovered capabilities=%+v", caps)
	}
}

func TestWeWorkCallbackRedisCapabilityResolverRecoversWithRealRedis(t *testing.T) {
	if os.Getenv("MOCHAT_GO_CALLBACK_REDIS_RECOVERY_INTEGRATION") != "1" {
		t.Skip("SKIP: set MOCHAT_GO_CALLBACK_REDIS_RECOVERY_INTEGRATION=1 with an isolated Redis that starts unavailable and then recovers")
	}
	addr := strings.TrimSpace(os.Getenv("MOCHAT_REDIS_ADDR"))
	if addr == "" {
		t.Fatal("MOCHAT_REDIS_ADDR is required")
	}
	redis := store.NewRedisStore(store.RedisConfig{Addr: addr})
	defer redis.Close()
	resolver := weWorkCallbackRedisCapabilityResolver{candidate: redis}

	downCtx, cancelDown := context.WithTimeout(context.Background(), 200*time.Millisecond)
	_, downErr := resolver.ResolveWeWorkCallbackCapabilities(downCtx)
	cancelDown()
	if !errors.Is(downErr, dashboard.ErrWeWorkCallbackDependencyUnavailable) {
		t.Fatalf("initial paused Redis error=%v", downErr)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		probeCtx, cancelProbe := context.WithTimeout(context.Background(), 500*time.Millisecond)
		caps, err := resolver.ResolveWeWorkCallbackCapabilities(probeCtx)
		cancelProbe()
		if err == nil {
			if caps.ContactWelcomeQueue == nil || caps.ContactWelcomeCache == nil || caps.MarkTagsQueue == nil {
				t.Fatalf("recovered capabilities=%+v", caps)
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("Redis capability did not recover before deadline")
}

func TestContactWelcomeConsumerRecoversWithQueuedDeliveryAfterRedisReturns(t *testing.T) {
	if os.Getenv("MOCHAT_GO_CONTACT_WELCOME_RECOVERY_INTEGRATION") != "1" {
		t.Skip("SKIP: set MOCHAT_GO_CONTACT_WELCOME_RECOVERY_INTEGRATION=1 with an isolated Redis containing a queued welcome, initially stopped, then started")
	}
	addr := strings.TrimSpace(os.Getenv("MOCHAT_REDIS_ADDR"))
	if addr == "" {
		t.Fatal("MOCHAT_REDIS_ADDR is required")
	}
	redis := store.NewRedisStore(store.RedisConfig{Addr: addr})
	defer redis.Close()
	downCtx, cancelDown := context.WithTimeout(context.Background(), 200*time.Millisecond)
	downErr := redis.Ping(downCtx)
	cancelDown()
	if downErr == nil {
		t.Fatal("Redis must be unavailable when the contact welcome consumer starts")
	}

	queue := &observedContactWelcomeQueue{queue: redis, acked: make(chan struct{})}
	worker := dashboard.NewContactWelcomeWorker(queue, inertContactWelcomeStore{}, inertContactWelcomeClient{}, "", "", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	result := make(chan error, 1)
	go func() { result <- worker.Run(ctx) }()
	select {
	case <-queue.acked:
		cancel()
	case <-ctx.Done():
		t.Fatal("queued contact welcome was not consumed after Redis recovered")
	}
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("contact welcome worker error=%v", err)
	}
}

type observedContactWelcomeQueue struct {
	queue *store.RedisStore
	once  sync.Once
	acked chan struct{}
}

func (q *observedContactWelcomeQueue) DequeueContactWelcome(ctx context.Context, timeout time.Duration) (dashboard.ContactWelcomeDelivery, bool, error) {
	return q.queue.DequeueContactWelcome(ctx, timeout)
}

func (q *observedContactWelcomeQueue) AckContactWelcome(ctx context.Context, delivery dashboard.ContactWelcomeDelivery) error {
	if err := q.queue.AckContactWelcome(ctx, delivery); err != nil {
		return err
	}
	q.once.Do(func() { close(q.acked) })
	return nil
}

func (q *observedContactWelcomeQueue) RetryContactWelcome(ctx context.Context, delivery dashboard.ContactWelcomeDelivery, reason string, maxAttempts int) (bool, error) {
	return q.queue.RetryContactWelcome(ctx, delivery, reason, maxAttempts)
}

func (q *observedContactWelcomeQueue) RecoverContactWelcomeProcessing(ctx context.Context, staleAfter time.Duration, maxAttempts int) (int, error) {
	return q.queue.RecoverContactWelcomeProcessing(ctx, staleAfter, maxAttempts)
}

type inertContactWelcomeStore struct{}

func (inertContactWelcomeStore) RoomWelcomeCorpCredentialByID(context.Context, int) (dashboard.RoomWelcomeCorpCredential, bool, error) {
	return dashboard.RoomWelcomeCorpCredential{}, false, nil
}

type inertContactWelcomeClient struct{}

func (inertContactWelcomeClient) UploadTemporaryImage(context.Context, dashboard.RoomWelcomeCorpCredential, string) (string, error) {
	return "", nil
}

func (inertContactWelcomeClient) SendExternalContactWelcome(context.Context, dashboard.RoomWelcomeCorpCredential, string, dashboard.ContactWelcomePayload) error {
	return nil
}
