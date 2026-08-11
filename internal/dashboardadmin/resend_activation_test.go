package dashboardadmin

import (
	"context"
	"errors"
	"testing"
)

func TestResendActivationRequiresAnExplicitRequestKeyBeforeStore(t *testing.T) {
	service := NewService(nil)
	_, err := service.ResendActivation(context.Background(), validActor(), ResendActivationInput{
		TenantID:        41,
		TargetUserID:    52,
		ExpectedVersion: 3,
	})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("error=%v, want ErrInvalidRequest for missing resend request key", err)
	}
}
