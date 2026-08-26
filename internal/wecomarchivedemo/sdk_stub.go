//go:build !linux

package wecomarchivedemo

import "fmt"

func CheckFinanceSDKLibrary() error {
	return fmt.Errorf("%w: Linux is required", ErrFinanceSDKCapabilityUnavailable)
}

func NewFinanceSDK(_, _ string) (FinanceSDK, error) {
	return nil, fmt.Errorf("%w: Linux is required", ErrFinanceSDKCapabilityUnavailable)
}
