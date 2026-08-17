package wecomarchivedemo

import "fmt"

type FinanceSDK interface {
	GetChatData(seq uint64, limit uint32, timeoutSeconds int) ([]byte, error)
	DecryptData(randomKey, encryptedMessage string) ([]byte, error)
	Close() error
}

type SDKError struct {
	Operation string
	Code      int
}

func (e SDKError) Error() string {
	return fmt.Sprintf("WeCom Finance SDK %s failed with code %d", e.Operation, e.Code)
}
