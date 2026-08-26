package wecomarchivedemo

import (
	"context"
	"errors"
	"fmt"
)

var ErrFinanceSDKCapabilityUnavailable = errors.New("WeCom Finance SDK capability unavailable on this platform")

const maxMediaIndexLength = 1024

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

func validateMediaIndexLength(length int) (int, error) {
	if length < 0 {
		return 0, errors.New("WeCom Finance SDK returned a negative media index length")
	}
	if length > maxMediaIndexLength {
		return 0, errors.New("WeCom Finance SDK returned an oversized media index")
	}
	return length, nil
}

type SDKError struct {
	Operation string
	Code      int
}

func (e SDKError) Error() string {
	return fmt.Sprintf("WeCom Finance SDK %s failed with code %d", e.Operation, e.Code)
}
