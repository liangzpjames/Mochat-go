package wecomarchivedemo

import (
	"context"
	"errors"
	"fmt"
)

var ErrFinanceSDKCapabilityUnavailable = errors.New("WeCom Finance SDK capability unavailable on this platform")

type MediaChunk struct {
	Data         []byte
	NextIndexBuf string
	Finished     bool
}

type FinanceSDK interface {
	GetChatData(seq uint64, limit uint32, timeoutSeconds int) ([]byte, error)
	DecryptData(randomKey, encryptedMessage string) ([]byte, error)
	GetMediaData(ctx context.Context, sdkFileID, indexBuf string, timeoutSeconds int) (MediaChunk, error)
	Close() error
}

type SDKError struct {
	Operation string
	Code      int
}

func (e SDKError) Error() string {
	return fmt.Sprintf("WeCom Finance SDK %s failed with code %d", e.Operation, e.Code)
}
