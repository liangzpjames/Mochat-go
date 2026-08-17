//go:build !linux

package wecomarchivedemo

import "errors"

func CheckFinanceSDKLibrary() error {
	return errors.New("WeCom Finance SDK requires Linux")
}

func NewFinanceSDK(_, _ string) (FinanceSDK, error) {
	return nil, errors.New("WeCom Finance SDK requires Linux")
}
