//go:build linux

package wecomarchivedemo

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

type financeSymbols struct {
	newSDK              func() uintptr
	init                func(uintptr, string, string) int32
	destroySDK          func(uintptr)
	getChatData         func(uintptr, uint64, uint32, string, string, int32, uintptr) int32
	decryptData         func(string, string, uintptr) int32
	newSlice            func() uintptr
	freeSlice           func(uintptr)
	getContentFromSlice func(uintptr) uintptr
	getSliceLen         func(uintptr) int32
}

type nativeFinanceSDK struct {
	mu      sync.Mutex
	library uintptr
	handle  uintptr
	symbols financeSymbols
}

func CheckFinanceSDKLibrary() error {
	path := os.Getenv("WECOM_FINANCE_SDK_PATH")
	if path == "" {
		path = "/opt/wecom-sdk/libWeWorkFinanceSdk_C.so"
	}
	library, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return fmt.Errorf("load WeCom Finance SDK: %w", err)
	}
	defer purego.Dlclose(library)
	symbols, err := loadFinanceSymbols(library)
	if err != nil {
		return err
	}
	handle := symbols.newSDK()
	if handle == 0 {
		return errors.New("WeCom Finance SDK NewSdk returned nil")
	}
	symbols.destroySDK(handle)
	return nil
}

func NewFinanceSDK(corpID, secret string) (FinanceSDK, error) {
	path := os.Getenv("WECOM_FINANCE_SDK_PATH")
	if path == "" {
		path = "/opt/wecom-sdk/libWeWorkFinanceSdk_C.so"
	}
	library, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil, fmt.Errorf("load WeCom Finance SDK: %w", err)
	}
	symbols, err := loadFinanceSymbols(library)
	if err != nil {
		_ = purego.Dlclose(library)
		return nil, err
	}
	handle := symbols.newSDK()
	if handle == 0 {
		_ = purego.Dlclose(library)
		return nil, errors.New("WeCom Finance SDK NewSdk returned nil")
	}
	if code := int(symbols.init(handle, corpID, secret)); code != 0 {
		symbols.destroySDK(handle)
		_ = purego.Dlclose(library)
		return nil, SDKError{Operation: "Init", Code: code}
	}
	return &nativeFinanceSDK{library: library, handle: handle, symbols: symbols}, nil
}

func loadFinanceSymbols(library uintptr) (financeSymbols, error) {
	var symbols financeSymbols
	for name, target := range map[string]any{
		"NewSdk":              &symbols.newSDK,
		"Init":                &symbols.init,
		"DestroySdk":          &symbols.destroySDK,
		"GetChatData":         &symbols.getChatData,
		"DecryptData":         &symbols.decryptData,
		"NewSlice":            &symbols.newSlice,
		"FreeSlice":           &symbols.freeSlice,
		"GetContentFromSlice": &symbols.getContentFromSlice,
		"GetSliceLen":         &symbols.getSliceLen,
	} {
		address, err := purego.Dlsym(library, name)
		if err != nil {
			return financeSymbols{}, fmt.Errorf("resolve Finance SDK symbol %s: %w", name, err)
		}
		purego.RegisterFunc(target, address)
	}
	return symbols, nil
}

func (s *nativeFinanceSDK) GetChatData(seq uint64, limit uint32, timeoutSeconds int) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.handle == 0 {
		return nil, errors.New("WeCom Finance SDK is closed")
	}
	result := s.symbols.newSlice()
	if result == 0 {
		return nil, errors.New("WeCom Finance SDK NewSlice returned nil")
	}
	defer s.symbols.freeSlice(result)
	if code := int(s.symbols.getChatData(s.handle, seq, limit, "", "", int32(timeoutSeconds), result)); code != 0 {
		return nil, SDKError{Operation: "GetChatData", Code: code}
	}
	return s.copySlice(result)
}

func (s *nativeFinanceSDK) DecryptData(randomKey, encryptedMessage string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.handle == 0 {
		return nil, errors.New("WeCom Finance SDK is closed")
	}
	result := s.symbols.newSlice()
	if result == 0 {
		return nil, errors.New("WeCom Finance SDK NewSlice returned nil")
	}
	defer s.symbols.freeSlice(result)
	if code := int(s.symbols.decryptData(randomKey, encryptedMessage, result)); code != 0 {
		return nil, SDKError{Operation: "DecryptData", Code: code}
	}
	return s.copySlice(result)
}

func (s *nativeFinanceSDK) copySlice(result uintptr) ([]byte, error) {
	length := int(s.symbols.getSliceLen(result))
	if length < 0 {
		return nil, errors.New("WeCom Finance SDK returned a negative slice length")
	}
	if length == 0 {
		return []byte{}, nil
	}
	pointer := s.symbols.getContentFromSlice(result)
	if pointer == 0 {
		return nil, errors.New("WeCom Finance SDK returned a nil content pointer")
	}
	return append([]byte(nil), unsafe.Slice((*byte)(unsafe.Pointer(pointer)), length)...), nil
}

func (s *nativeFinanceSDK) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.handle != 0 {
		s.symbols.destroySDK(s.handle)
		s.handle = 0
	}
	if s.library != 0 {
		err := purego.Dlclose(s.library)
		s.library = 0
		return err
	}
	return nil
}
