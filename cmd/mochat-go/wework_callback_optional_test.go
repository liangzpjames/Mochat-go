package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"jiyi/mochat-go/internal/dashboard"
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
