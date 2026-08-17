package wecomarchivedemo

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

type fakeFinanceSDK struct {
	chatData []byte
	plain    map[string][]byte
	getErr   error
	keys     []string
}

func (f *fakeFinanceSDK) GetChatData(_ uint64, _ uint32, _ int) ([]byte, error) {
	return f.chatData, f.getErr
}

func (f *fakeFinanceSDK) DecryptData(randomKey, encryptedMessage string) ([]byte, error) {
	f.keys = append(f.keys, randomKey)
	plain, ok := f.plain[encryptedMessage]
	if !ok {
		return nil, errors.New("unknown encrypted message")
	}
	return plain, nil
}

func (f *fakeFinanceSDK) Close() error { return nil }

func TestArchiveServicePullDecryptsAndAdvancesSeq(t *testing.T) {
	privatePEM, encryptedRandomKey := archiveRSAFixture(t, []byte("session-key"))
	response := map[string]any{
		"errcode": 0,
		"errmsg":  "ok",
		"chatdata": []map[string]any{{
			"seq": 7, "msgid": "msg-7", "publickey_ver": 3,
			"encrypt_random_key": encryptedRandomKey, "encrypt_chat_msg": "cipher-7",
		}},
	}
	chatData, _ := json.Marshal(response)
	sdk := &fakeFinanceSDK{chatData: chatData, plain: map[string][]byte{
		"cipher-7": []byte(`{"msgid":"msg-7","action":"send","from":"zhangsan","tolist":["lisi"],"msgtime":1710000000000,"msgtype":"text","text":{"content":"demo message"}}`),
	}}
	store, err := NewEvidenceStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewArchiveService(sdk, privatePEM, store, 100, 5)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Pull(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.StartSeq != 0 || result.NextSeq != 7 || result.MessageCount != 1 || len(sdk.keys) != 1 || sdk.keys[0] != "session-key" {
		t.Fatalf("result=%+v keys=%v", result, sdk.keys)
	}
	state, err := store.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if state.Seq != 7 || state.PulledMessageCount != 1 || state.LastPublicKeyVersion != 3 {
		t.Fatalf("state=%+v", state)
	}
}

func TestArchiveServiceRepeatedPageDoesNotDuplicateEvidence(t *testing.T) {
	privatePEM, encryptedRandomKey := archiveRSAFixture(t, []byte("session-key"))
	chatData, _ := json.Marshal(map[string]any{"errcode": 0, "chatdata": []map[string]any{{
		"seq": 7, "msgid": "msg-7", "publickey_ver": 3,
		"encrypt_random_key": encryptedRandomKey, "encrypt_chat_msg": "cipher-7",
	}}})
	sdk := &fakeFinanceSDK{chatData: chatData, plain: map[string][]byte{
		"cipher-7": []byte(`{"msgid":"msg-7","msgtype":"text","text":{"content":"same message"}}`),
	}}
	dir := t.TempDir()
	store, err := NewEvidenceStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewArchiveService(sdk, privatePEM, store, 100, 5)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Pull(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Pull(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := jsonLineCount(t, filepath.Join(dir, "archive-messages.jsonl")); got != 1 {
		t.Fatalf("archive evidence lines = %d, want 1", got)
	}
	state, err := store.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if state.PulledMessageCount != 1 || state.PullCount != 2 || state.Seq != 7 {
		t.Fatalf("state=%+v", state)
	}
}

func TestArchiveServiceRecoversWhenEvidenceExistsBeforeCursor(t *testing.T) {
	privatePEM, encryptedRandomKey := archiveRSAFixture(t, []byte("session-key"))
	chatData, _ := json.Marshal(map[string]any{"errcode": 0, "chatdata": []map[string]any{{
		"seq": 7, "msgid": "msg-7", "publickey_ver": 3,
		"encrypt_random_key": encryptedRandomKey, "encrypt_chat_msg": "cipher-7",
	}}})
	sdk := &fakeFinanceSDK{chatData: chatData, plain: map[string][]byte{
		"cipher-7": []byte(`{"msgid":"msg-7","msgtype":"text","text":{"content":"same message"}}`),
	}}
	dir := t.TempDir()
	store, err := NewEvidenceStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AppendArchive(ArchiveEvidence{Seq: 7, MsgID: "msg-7", ContentSHA256: "existing"}); err != nil {
		t.Fatal(err)
	}
	service, err := NewArchiveService(sdk, privatePEM, store, 100, 5)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Pull(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := jsonLineCount(t, filepath.Join(dir, "archive-messages.jsonl")); got != 1 {
		t.Fatalf("archive evidence lines = %d, want 1", got)
	}
	state, err := store.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if state.PulledMessageCount != 1 || state.Seq != 7 {
		t.Fatalf("state=%+v", state)
	}
}

func jsonLineCount(t *testing.T, path string) int {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		count++
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return count
}

func TestArchiveServiceDoesNotAdvanceOnSDKError(t *testing.T) {
	store, err := NewEvidenceStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewArchiveService(&fakeFinanceSDK{getErr: errors.New("sdk 10009")}, testPrivateKeyPEM(t), store, 100, 5)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Pull(context.Background()); err == nil {
		t.Fatal("SDK error was ignored")
	}
	state, err := store.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if state.Seq != 0 {
		t.Fatalf("seq advanced to %d", state.Seq)
	}
}

func archiveRSAFixture(t *testing.T, plaintext []byte) (string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := rsa.EncryptPKCS1v15(rand.Reader, &key.PublicKey, plaintext)
	if err != nil {
		t.Fatal(err)
	}
	privatePEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	return string(privatePEM), base64.StdEncoding.EncodeToString(ciphertext)
}

func testPrivateKeyPEM(t *testing.T) string {
	t.Helper()
	privatePEM, _ := archiveRSAFixture(t, []byte(fmt.Sprintf("key-%s", t.Name())))
	return privatePEM
}
