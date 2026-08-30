package migration

import (
	"context"
	"errors"
)

// AIInsight0165BodyForServerForTest exposes only the immutable 0165
// compatibility transformation to the external-package integration contract.
func AIInsight0165BodyForServerForTest(body, serverVersion string) string {
	return aiInsight0165BodyForServer(body, serverVersion)
}

// AcquireAIInsight0165LockForTest exposes only the exact controlled-0165 lock
// boundary to the external-package cross-schema concurrency contract.
func AcquireAIInsight0165LockForTest(ctx context.Context, controller *AIInsight0165Controller) (func(), error) {
	if controller == nil {
		return nil, errors.New("0165 controlled migration controller is required")
	}
	_, release, err := controller.lockedConnection(ctx)
	return release, err
}
